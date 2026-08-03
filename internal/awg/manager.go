// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type managed struct {
	proc        *Process
	tag         string
	fingerprint string
	ifname      string
	// Traffic baselines per peer public key, so CollectTraffic returns deltas.
	lastRx   map[string]int64
	lastTx   map[string]int64
	haveLast bool
}

// Manager owns the set of running AWG interfaces keyed by inbound id, exactly
// mirroring the mtproto sidecar Manager. The runtime delegates AWG inbounds to
// this manager instead of the Xray gRPC API.
type Manager struct {
	mu    sync.Mutex
	procs map[int]*managed
	// swept records that the one-time startup cleanup of orphaned awg
	// interfaces and tun2socks processes (survivors of a previous x-ui run)
	// has already run.
	swept bool
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide AWG manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{procs: map[int]*managed{}}
	})
	return manager
}

// Ensure starts the AWG interface for an instance, or restarts it when its
// configuration changed. A no-op when the desired interface is already up
// with matching obfuscation and peers.
func (m *Manager) Ensure(inst Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepOrphansLocked()
	if err := m.ensureLocked(inst); err != nil {
		return err
	}
	m.ensureXrayRouting(inst)
	m.ensureNatRules(inst)
	return nil
}

// sweepOrphansLocked kills awg interfaces and tun2socks processes left running
// by a previous x-ui run, exactly once per process lifetime and before any of
// our own interfaces are started. Because x-ui owns every awgN interface and
// tun2socks process, anything alive at this point is an orphan that would
// otherwise keep holding an inbound port with stale obfuscation.
func (m *Manager) sweepOrphansLocked() {
	if m.swept {
		return
	}
	m.swept = true
	if n := killStrayAwgInterfaces(); n > 0 {
		logger.Warningf("awg: removed %d orphaned interface(s) from a previous run", n)
	}
}

func (m *Manager) ensureLocked(inst Instance) error {
	fp := inst.fingerprint()
	if cur, ok := m.procs[inst.Id]; ok {
		if cur.fingerprint == fp && cur.proc.IsRunning() {
			cur.tag = inst.Tag
			return nil
		}
		_ = cur.proc.Stop()
		delete(m.procs, inst.Id)
	}
	// Write the .conf the sidecar will bring up.
	if err := writeServerConfigFile(inst); err != nil {
		return err
	}
	proc := newProcess(inst.Ifname, configPathForID(inst.Id), fmt.Sprintf("inbound %d", inst.Id))
	if err := proc.Start(); err != nil {
		return err
	}
	m.procs[inst.Id] = &managed{
		proc:        proc,
		tag:         inst.Tag,
		fingerprint: fp,
		ifname:      inst.Ifname,
		lastRx:      map[string]int64{},
		lastTx:      map[string]int64{},
	}
	logger.Infof("awg: started interface %s for inbound %d on port %d", inst.Ifname, inst.Id, inst.Port)
	return nil
}

// Remove stops and forgets the AWG interface for an inbound id.
func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.procs[id]; ok {
		_ = cur.proc.Stop()
		delete(m.procs, id)
		_ = os.Remove(configPathForID(id))
		logger.Infof("awg: stopped interface %s for inbound %d", cur.ifname, id)
	}
}

// Reconcile drives the running set toward the desired instances: it stops
// interfaces that are no longer wanted and (re)starts the rest. Used at boot
// and periodically to recover from crashes.
func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepOrphansLocked()
	want := make(map[int]struct{}, len(desired))
	for _, inst := range desired {
		want[inst.Id] = struct{}{}
	}
	for id, cur := range m.procs {
		if _, ok := want[id]; !ok {
			_ = cur.proc.Stop()
			delete(m.procs, id)
			_ = os.Remove(configPathForID(id))
		}
	}
	for _, inst := range desired {
		if err := m.ensureLocked(inst); err != nil {
			logger.Warningf("awg: reconcile failed for inbound %d: %v", inst.Id, err)
			continue
		}
		m.ensureXrayRouting(inst)
		m.ensureNatRules(inst)
	}
}

// StopAll stops every managed AWG interface. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cur := range m.procs {
		_ = cur.proc.Stop()
		_ = os.Remove(configPathForID(id))
		delete(m.procs, id)
	}
}

// PeerTraffic is a per-peer (per-client) traffic delta: same rx/tx meaning as
// Traffic but attributed to one peer public key, so the panel can account
// per-client traffic for AWG like mtproto does per email.
type PeerTraffic struct {
	Tag       string
	PublicKey string
	Up        int64
	Down      int64
}

// handshakeOnlineTTL is the window in seconds in which a completed handshake
// means the peer is online: WireGuard rekeys every ~120 s (REKEY_TIMEOUT), so
// a handshake older than 180 s implies a dead session — the convention used
// by WireGuard dashboards.
const handshakeOnlineTTL = 180

// CollectTraffic scrapes each running AWG interface once and returns three
// views of the same data: per-inbound byte deltas (for the inbound counters),
// per-peer byte deltas keyed by public key (for per-client accounting), and
// the online peers per tag (handshake within handshakeOnlineTTL) so the
// panel's online status works for AWG clients, which never pass through
// Xray's stats API.
func (m *Manager) CollectTraffic() ([]Traffic, []PeerTraffic, map[string][]string) {
	type snap struct {
		id       int
		ifname   string
		tag      string
		haveLast bool
		lastRx   map[string]int64
		lastTx   map[string]int64
	}
	m.mu.Lock()
	snaps := make([]snap, 0, len(m.procs))
	for id, cur := range m.procs {
		if cur.proc == nil || !cur.proc.IsRunning() {
			continue
		}
		snaps = append(snaps, snap{
			id:       id,
			ifname:   cur.ifname,
			tag:      cur.tag,
			haveLast: cur.haveLast,
			lastRx:   cur.lastRx,
			lastTx:   cur.lastTx,
		})
	}
	m.mu.Unlock()

	out := make([]Traffic, 0, len(snaps))
	peerOut := make([]PeerTraffic, 0, len(snaps))
	online := make(map[string][]string, len(snaps))
	now := time.Now().Unix()
	for _, s := range snaps {
		peers, ok := scrapePeers(s.ifname)
		if !ok {
			continue
		}
		newRx := make(map[string]int64, len(peers))
		newTx := make(map[string]int64, len(peers))
		var inboundUp, inboundDown int64
		var onlinePeers []string
		for _, peer := range peers {
			newRx[peer.PublicKey] = peer.Rx
			newTx[peer.PublicKey] = peer.Tx
			if peer.LastHandshake > 0 && now-peer.LastHandshake <= handshakeOnlineTTL {
				onlinePeers = append(onlinePeers, peer.PublicKey)
			}
			if !s.haveLast {
				continue
			}
			prevRx, hadRx := s.lastRx[peer.PublicKey]
			prevTx, hadTx := s.lastTx[peer.PublicKey]
			if !hadRx || !hadTx {
				continue
			}
			du := peer.Rx - prevRx
			dd := peer.Tx - prevTx
			if du < 0 {
				du = 0
			}
			if dd < 0 {
				dd = 0
			}
			inboundUp += du
			inboundDown += dd
			if du > 0 || dd > 0 {
				peerOut = append(peerOut, PeerTraffic{Tag: s.tag, PublicKey: peer.PublicKey, Up: du, Down: dd})
			}
		}
		if len(onlinePeers) > 0 {
			online[s.tag] = onlinePeers
		}
		// Re-acquire lock to persist the new per-peer baseline. A peer that
		// left the interface simply drops out of the map, so a returning peer
		// starts a fresh baseline instead of producing a negative delta.
		m.mu.Lock()
		if cur, ok := m.procs[s.id]; ok {
			cur.lastRx = newRx
			cur.lastTx = newTx
			cur.haveLast = true
		}
		m.mu.Unlock()

		if s.haveLast && (inboundUp > 0 || inboundDown > 0) {
			out = append(out, Traffic{Tag: s.tag, Up: inboundUp, Down: inboundDown})
		}
	}
	return out, peerOut, online
}

// writeServerConfigFile renders the .conf for an instance and writes it to
// the conventional AWG config path. Mirrors mtproto's writeConfig.
func writeServerConfigFile(inst Instance) error {
	if err := os.MkdirAll(awgConfigDir, 0o750); err != nil {
		return err
	}
	conf := renderServerConf(inst)
	return os.WriteFile(configPathForID(inst.Id), []byte(conf), 0o600)
}

// tunNameFor returns the name of the Xray TUN inbound device paired with an
// AWG inbound id (awgN → tunN), matching injectAwgEgress in the web service.
func tunNameFor(id int) string {
	return fmt.Sprintf("tun%d", id)
}

// awgRouteTable returns the per-inbound policy-routing table that carries the
// default route into tunN. Offset by 1000 to stay clear of the tables admins
// commonly hand-allocate (100, 200, …) and of the reserved 253-255 range.
func awgRouteTable(id int) int {
	return 1000 + id
}

// ensureXrayRoutingCmds returns the idempotent commands that keep one routed
// instance converged: the per-inbound table's default route pinned into tunN
// and loose reverse-path filtering on tunN (Xray writes replies with public
// source addresses into it, which strict rp_filter would drop).
func ensureXrayRoutingCmds(inst Instance) [][]string {
	tunName := tunNameFor(inst.Id)
	return [][]string{
		{"ip", "route", "replace", "default", "dev", tunName, "table", strconv.Itoa(awgRouteTable(inst.Id))},
		{"sysctl", "-qw", "net.ipv4.conf." + tunName + ".rp_filter=2"},
	}
}

// ruleMissing reports whether `ip rule show` output lacks a lookup into the
// given routing table. Suffix-matched per line so table 100 does not shadow
// 1003.
func ruleMissing(ruleOutput string, table int) bool {
	needle := "lookup " + strconv.Itoa(table)
	for _, line := range strings.Split(ruleOutput, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), needle) {
			return false
		}
	}
	return true
}

// ensureXrayRouting converges the kernel routing state a routed instance
// needs around the Xray-owned tunN device. It must run periodically, not once:
// the table's default route dies with tunN on every Xray restart, and PostUp
// cannot install it because tunN does not exist yet when awg-quick runs. A
// no-op (and silent) while tunN is absent — Xray may be down or restarting.
func (m *Manager) ensureXrayRouting(inst Instance) {
	if !inst.RouteThroughXray {
		return
	}
	tunName := tunNameFor(inst.Id)
	if err := exec.CommandContext(context.Background(), "ip", "link", "show", tunName).Run(); err != nil {
		return
	}
	for _, args := range ensureXrayRoutingCmds(inst) {
		if out, err := exec.CommandContext(context.Background(), args[0], args[1:]...).CombinedOutput(); err != nil {
			logger.Warningf("awg: ensure xray routing (%s): %v\n%s", strings.Join(args, " "), err, string(out))
		}
	}
	table := awgRouteTable(inst.Id)
	out, err := exec.CommandContext(context.Background(), "ip", "rule", "show", "iif", inst.Ifname).Output()
	if err != nil || !ruleMissing(string(out), table) {
		return
	}
	if out2, err2 := exec.CommandContext(context.Background(), "ip", "rule", "add", "iif", inst.Ifname, "lookup", strconv.Itoa(table)).CombinedOutput(); err2 != nil {
		logger.Warningf("awg: re-add policy rule for %s: %v\n%s", inst.Ifname, err2, string(out2))
	}
}

// clientSubnet extracts the network prefix (e.g. "10.8.0.0/24") from the
// server's tunnel Address (e.g. "10.8.0.1/24"). Returns empty when Address is
// unset or unparseable, in which case NAT rules are skipped.
func clientSubnet(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	prefix, err := netip.ParsePrefix(address)
	if err != nil {
		return ""
	}
	return prefix.Masked().String()
}

// natPostUpPostDown builds the PostUp/PostDown pair that wires kernel routing
// for an AWG interface.
//
// Without routeThroughXray: the kernel forwards decrypted client packets
// with their private source (e.g. 10.8.0.x) unchanged, so replies never come
// back: ip_forward is off by default and no MASQUERADE exists. We enable
// forwarding, add MASQUERADE on the external interface, and accept FORWARD in
// both directions. This mirrors pumbaX/awg-multi-script.
//
// With routeThroughXray: Xray owns the routing via an injected TUN inbound
// (tunN). PostUp wires the static half — forwarding, loose reverse-path
// filtering on awgN, FORWARD accepts for both awgN and tunN legs, and an iif
// policy rule sending everything received on awgN into the per-inbound table
// awgRouteTable(id). The iif selector (not `from <subnet>`) matches only
// forwarded client traffic, so server-originated packets sourced from the
// awgN address still reach clients directly. The table's default route into
// tunN is NOT set here: tunN does not exist yet at PostUp time and is
// recreated on every Xray restart, so ensureXrayRouting (called from the
// reconcile loop) owns it. No MASQUERADE here — Xray terminates the flows in
// its TUN netstack and dials out with the server's own address.
func natPostUpPostDown(inst Instance) (postUp, postDown string) {
	subnet := clientSubnet(inst.Address)
	if subnet == "" {
		return "", ""
	}
	iface := inst.Ifname
	tunName := tunNameFor(inst.Id)

	if inst.RouteThroughXray {
		table := awgRouteTable(inst.Id)
		// MSS clamping: AWG and TUN interfaces carry MTU 1320 (set in the
		// .conf and the Xray TUN inbound), so TCP sessions crossing them
		// cannot carry a 1460-byte MSS without fragmentation. Without
		// clamping, large HTTPS downloads stall or crawl — classic VPN
		// symptom. `--clamp-mss-to-pmtu` derives MSS from the route MTU
		// per-packet, so it works regardless of the path MTU between client
		// and server. Mirrors pumbaX/awg-multi-script's MSS rule.
		postUp = fmt.Sprintf(
			"echo 1 > /proc/sys/net/ipv4/ip_forward; "+
				"sysctl -qw net.ipv4.conf.%s.rp_filter=2; "+
				"iptables -C FORWARD -i %s -j ACCEPT 2>/dev/null || "+
				"iptables -A FORWARD -i %s -j ACCEPT; "+
				"iptables -C FORWARD -o %s -j ACCEPT 2>/dev/null || "+
				"iptables -A FORWARD -o %s -j ACCEPT; "+
				"iptables -C FORWARD -i %s -j ACCEPT 2>/dev/null || "+
				"iptables -A FORWARD -i %s -j ACCEPT; "+
				"iptables -C FORWARD -o %s -j ACCEPT 2>/dev/null || "+
				"iptables -A FORWARD -o %s -j ACCEPT; "+
				"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
				"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu; "+
				"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
				"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu; "+
				"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
				"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu; "+
				"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
				"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu; "+
				"ip rule del iif %s lookup %d 2>/dev/null || true; "+
				"ip rule add iif %s lookup %d",
			iface,
			iface, iface, iface, iface,
			tunName, tunName, tunName, tunName,
			tunName, tunName, tunName, tunName,
			iface, iface, iface, iface,
			iface, table, iface, table,
		)
		postDown = fmt.Sprintf(
			"ip rule del iif %s lookup %d 2>/dev/null || true; "+
				"ip route flush table %d 2>/dev/null || true; "+
				"iptables -D FORWARD -i %s -j ACCEPT 2>/dev/null || true; "+
				"iptables -D FORWARD -o %s -j ACCEPT 2>/dev/null || true; "+
				"iptables -D FORWARD -i %s -j ACCEPT 2>/dev/null || true; "+
				"iptables -D FORWARD -o %s -j ACCEPT 2>/dev/null || true; "+
				"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true; "+
				"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true; "+
				"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true; "+
				"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true",
			iface, table, table,
			iface, iface, tunName, tunName,
			tunName, tunName, iface, iface,
		)
		return postUp, postDown
	}

	extIface := defaultRouteInterface()
	if extIface == "" {
		return "", ""
	}
	postUp = fmt.Sprintf(
		"echo 1 > /proc/sys/net/ipv4/ip_forward; "+
			"iptables -t nat -C POSTROUTING -s %s -o %s -j MASQUERADE 2>/dev/null || "+
			"iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE; "+
			"iptables -C FORWARD -i %s -j ACCEPT 2>/dev/null || "+
			"iptables -A FORWARD -i %s -j ACCEPT; "+
			"iptables -C FORWARD -o %s -j ACCEPT 2>/dev/null || "+
			"iptables -A FORWARD -o %s -j ACCEPT; "+
			// MSS clamping for kernel-NAT mode too: awgN carries MTU 1320
			// (smaller than the 1500 the client assumes), so large TCP
			// segments crossing the tunnel need their MSS clamped to PMTU
			// or downloads stall. Same --clamp-mss-to-pmtu as above.
			"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
			"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu; "+
			"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
			"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu",
		subnet, extIface, subnet, extIface,
		iface, iface, iface, iface,
		iface, iface, iface, iface,
	)
	postDown = fmt.Sprintf(
		"iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE 2>/dev/null || true; "+
			"iptables -D FORWARD -i %s -j ACCEPT 2>/dev/null || true; "+
			"iptables -D FORWARD -o %s -j ACCEPT 2>/dev/null || true; "+
			"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true; "+
			"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true",
		subnet, extIface,
		iface, iface,
		iface, iface,
	)
	return postUp, postDown
}

// natRule is one idempotent iptables rule: probed with `-t <table> -C <chain>
// <spec>` and appended with `-A` only when the probe fails.
type natRule struct {
	table string
	chain string
	spec  []string
}

// natRulesFor returns the rule set a kernel-routed (non-routeThroughXray)
// instance needs: MASQUERADE of the client subnet out the external interface,
// plus FORWARD accepts for both awgN legs. Nil when the instance is unroutable
// (Xray-routed, no subnet, or no external interface).
func natRulesFor(inst Instance, extIface string) []natRule {
	subnet := clientSubnet(inst.Address)
	if inst.RouteThroughXray || subnet == "" || extIface == "" {
		return nil
	}
	return []natRule{
		{"nat", "POSTROUTING", []string{"-s", subnet, "-o", extIface, "-j", "MASQUERADE"}},
		{"filter", "FORWARD", []string{"-i", inst.Ifname, "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-o", inst.Ifname, "-j", "ACCEPT"}},
	}
}

// ensureNatRules converges the iptables NAT state of a kernel-routed instance.
// Like ensureXrayRouting it must run periodically: PostUp installs the rules
// once, but anything that flushes iptables (fail2ban reload, docker starting,
// admin intervention) silently kills client internet until the interface is
// restarted. A no-op while awgN is absent and for routeThroughXray instances
// (no NAT there — Xray terminates flows in its TUN netstack).
func (m *Manager) ensureNatRules(inst Instance) {
	if inst.RouteThroughXray {
		return
	}
	if err := exec.CommandContext(context.Background(), "ip", "link", "show", inst.Ifname).Run(); err != nil {
		return
	}
	rules := natRulesFor(inst, defaultRouteInterface())
	if len(rules) == 0 {
		return
	}
	if err := exec.CommandContext(context.Background(), "sysctl", "-qw", "net.ipv4.ip_forward=1").Run(); err != nil {
		logger.Warningf("awg: ensure ip_forward: %v", err)
	}
	for _, r := range rules {
		check := append([]string{"-t", r.table, "-C", r.chain}, r.spec...)
		if err := exec.CommandContext(context.Background(), "iptables", check...).Run(); err == nil {
			continue
		}
		add := append([]string{"-t", r.table, "-A", r.chain}, r.spec...)
		if out, err := exec.CommandContext(context.Background(), "iptables", add...).CombinedOutput(); err != nil {
			logger.Warningf("awg: ensure nat (iptables %s): %v\n%s", strings.Join(add, " "), err, string(out))
		}
	}
}

// renderServerConf builds the awg-quick .conf for an instance, reading from
// the Instance struct (desired runtime state) rather than the inbound's
// stored JSON.
func renderServerConf(inst Instance) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", inst.PrivateKey)
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.Port)
	if inst.Address != "" {
		fmt.Fprintf(&b, "Address = %s\n", inst.Address)
	}
	fmt.Fprintf(&b, "MTU = %d\n", inst.MTU)
	// DNS is CLIENT-ONLY — the server does not resolve through the tunnel.
	// Writing DNS to the server .conf makes awg-quick call resolvconf/openresolv
	// and overwrite the server's system DNS (e.g. with "1.1.1.1, 1.0.0.1"),
	// which can break name resolution on the host. pumbaX/awg-multi-script
	// never writes DNS to the server .conf, only to client configs.
	fmt.Fprintf(&b, "Jc = %d\n", inst.Jc)
	fmt.Fprintf(&b, "Jmin = %d\n", inst.Jmin)
	fmt.Fprintf(&b, "Jmax = %d\n", inst.Jmax)
	fmt.Fprintf(&b, "S1 = %d\n", inst.S1)
	fmt.Fprintf(&b, "S2 = %d\n", inst.S2)
	fmt.Fprintf(&b, "S3 = %d\n", inst.S3)
	fmt.Fprintf(&b, "S4 = %d\n", inst.S4)
	fmt.Fprintf(&b, "H1 = %s\n", inst.H1)
	fmt.Fprintf(&b, "H2 = %s\n", inst.H2)
	fmt.Fprintf(&b, "H3 = %s\n", inst.H3)
	fmt.Fprintf(&b, "H4 = %s\n", inst.H4)
	// HeaderProtectionKey (AWG3) is written ONLY when AwgVersion == "3" and the
	// key is non-empty. The upstream kernel module (v3.0.20260731) + tools
	// (v3.0.20260730) now parse the field; older builds reject it with "Line
	// unrecognized: `HeaderProtectionKey=...`" + "Configuration parsing error",
	// awg-quick deletes the half-built interface, and reconcile fails every 10s
	// so the inbound never serves traffic. Version-gating keeps v1/v2 inbounds
	// working on any kernel, and lets a v3 inbound opt in once the operator has
	// installed the AWG3 module. The S1-S4 >= 12 invariant (enforced by the
	// generator) is required for the kernel to accept the key.
	if inst.AwgVersion == "3" && inst.HeaderProtectionKey != "" && ModuleSupportsAwg3() {
		fmt.Fprintf(&b, "HeaderProtectionKey = %s\n", inst.HeaderProtectionKey)
	}
	// AWG3 device-level timers/padding — all optional (0 = kernel uses the
	// built-in WireGuard constant). Written only when > 0 and AwgVersion ==
	// "3"; older kernels reject these lines in setconf. Module-gated: if the
	// installed amneziawg module is < v3.0 (no ContentPaddingAddition/etc
	// support), even an AwgVersion="3" inbound must not emit the lines — the
	// kernel rejects "Line unrecognized" and awg-quick deletes the interface,
	// producing "Device <awgN> does not exist". Blocks the regression seen
	// when an operator picks v3 on a host still running the v1.x module.
	awg3ok := inst.AwgVersion == "3" && ModuleSupportsAwg3()
	if awg3ok {
		if inst.ContentPaddingAddition > 0 {
			fmt.Fprintf(&b, "ContentPaddingAddition = %d\n", inst.ContentPaddingAddition)
		}
		if inst.RekeyAfterTime > 0 {
			fmt.Fprintf(&b, "RekeyAfterTime = %d\n", inst.RekeyAfterTime)
		}
		if inst.RekeyTimeout > 0 {
			fmt.Fprintf(&b, "RekeyTimeout = %d\n", inst.RekeyTimeout)
		}
		if inst.RejectAfterTime > 0 {
			fmt.Fprintf(&b, "RejectAfterTime = %d\n", inst.RejectAfterTime)
		}
		if inst.KeepaliveTimeout > 0 {
			fmt.Fprintf(&b, "KeepaliveTimeout = %d\n", inst.KeepaliveTimeout)
		}
		if inst.MaxHandshakeAttempts > 0 {
			fmt.Fprintf(&b, "MaxHandshakeAttempts = %d\n", inst.MaxHandshakeAttempts)
		}
	}
	// I1-I5 (CPS packets) are CLIENT-ONLY — the server does not use them.
	// Writing I1-I5 to the server .conf crashes awg setconf ("Invalid
	// argument") because the kernel amneziawg module does not accept CPS
	// tags in setconf input. The client sends CPS packets before the
	// handshake for DPI evasion; the server ignores them. (Matches
	// pumbaX/awg-multi-script: server .conf has Jc/S/H only, never I1-I5.)
	if postUp, postDown := natPostUpPostDown(inst); postUp != "" {
		fmt.Fprintf(&b, "PostUp = %s\n", postUp)
		fmt.Fprintf(&b, "PostDown = %s\n", postDown)
	}
	for _, p := range inst.Peers {
		b.WriteString("\n[Peer]\n")
		fmt.Fprintf(&b, "PublicKey = %s\n", p.PublicKey)
		// PresharedKey is written ONLY when non-empty. An empty value renders as
		// "PresharedKey = " which awg setconf rejects ("invalid key"), awg-quick
		// rolls back the half-built interface, and reconcile reports "Device
		// <awgN> does not exist" — the crash seen when a client reaches this
		// renderer without a generated PSK (e.g. the inbound-form update path,
		// which unlike the clients-page path does not run defaultAwgClients).
		// Absent PresharedKey is the WireGuard convention for "no PSK" and
		// matches renderClientConf + SyncPeers, which already omit it.
		if psk := strings.TrimSpace(p.PSK); psk != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", psk)
		}
		allowed := p.AllowedIPs
		if allowed == "" {
			allowed = "0.0.0.0/0, ::/0"
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", allowed)
		if p.Keepalive > 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.Keepalive)
		}
		if awg3ok && p.AdvancedSecurity {
			b.WriteString("AdvancedSecurity = on\n")
		}
	}
	return b.String()
}

// SyncPeers re-syncs the kernel peer set for a running interface without a
// full restart. Called by AddUser/RemoveUser so adding/removing a client does
// not drop existing connections. Uses `awg set <iface> peer <pubkey> ...` /
// `awg set <iface> peer <pubkey> remove`.
func (m *Manager) SyncPeers(id int, peers []PeerSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.procs[id]
	if !ok || !cur.proc.IsRunning() {
		return fmt.Errorf("awg: interface for inbound %d not running", id)
	}
	ifname := cur.ifname
	for _, p := range peers {
		args := []string{"set", ifname, "peer", p.PublicKey}
		if p.PSK != "" {
			args = append(args, "preshared-key", p.PSK)
		}
		allowed := p.AllowedIPs
		if allowed == "" {
			allowed = "0.0.0.0/0, ::/0"
		}
		args = append(args, "allowed-ips", allowed)
		if p.Keepalive > 0 {
			args = append(args, "persistent-keepalive", fmt.Sprintf("%d", p.Keepalive))
		}
		if out, err := exec.CommandContext(context.Background(), "awg", args...).CombinedOutput(); err != nil {
			logger.Warningf("awg: set peer %s on %s: %v\n%s", p.PublicKey[:8], ifname, err, string(out))
		}
	}
	// Remove peers that are no longer desired: diff against kernel state.
	current := kernelPeers(ifname)
	desiredSet := make(map[string]bool, len(peers))
	for _, p := range peers {
		desiredSet[p.PublicKey] = true
	}
	for pub := range current {
		if !desiredSet[pub] {
			if out, err := exec.CommandContext(context.Background(), "awg", "set", ifname, "peer", pub, "remove").CombinedOutput(); err != nil {
				logger.Warningf("awg: remove peer %s on %s: %v\n%s", pub[:8], ifname, err, string(out))
			}
		}
	}
	// Reset the traffic baseline since the peer set changed.
	cur.lastRx = map[string]int64{}
	cur.lastTx = map[string]int64{}
	cur.haveLast = false
	return nil
}

// kernelPeers returns the set of peer public keys currently on an interface.
func kernelPeers(ifname string) map[string]bool {
	out, err := exec.CommandContext(context.Background(), "awg", "show", ifname, "peers").Output()
	if err != nil {
		return nil
	}
	peers := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			peers[line] = true
		}
	}
	return peers
}

// Traffic is a per-inbound traffic delta scraped from `awg show <iface> transfer`.
// Up is bytes from client to server (rx on the server); Down is server to
// client (tx on the server). The tag matches the inbound's Xray tag so the
// delta can be folded into the standard inbound traffic accounting.
type Traffic struct {
	Tag  string
	Up   int64
	Down int64
}

// peerStat is one peer row of `awg show <iface> dump`: cumulative counters
// plus the last-handshake timestamp, from which both traffic deltas and
// online status derive in a single scrape.
type peerStat struct {
	PublicKey     string
	Rx            int64
	Tx            int64
	LastHandshake int64
}

// scrapePeers runs `awg show <iface> dump` and parses the peer rows. The dump
// format is one interface line followed by one line per peer:
//
//	<pubkey>\t<preshared-key>\t<endpoint>\t<allowed-ips>\t<latest-handshake>\t<rx>\t<tx>\t<keepalive>
//
// A single dump carries everything CollectTraffic needs (counters + handshake),
// avoiding two shell-outs per interface per tick. rx is bytes received from
// the peer (upload from the client's perspective); tx is bytes sent to the
// peer (download). Returns ok=false when the interface is down or awg is
// unavailable.
func scrapePeers(ifname string) ([]peerStat, bool) {
	out, err := exec.CommandContext(context.Background(), "awg", "show", ifname, "dump").Output()
	if err != nil {
		return nil, false
	}
	return parseAwgDump(string(out))
}

// parseAwgDump parses `awg show <iface> dump` output: the interface line is
// skipped, each subsequent line is one peer. ok=false when there is no
// interface line at all (interface down or absent); an interface with zero
// peers yields ok=true with an empty slice.
func parseAwgDump(out string) ([]peerStat, bool) {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, false
	}
	lines := strings.Split(out, "\n")
	peers := make([]peerStat, 0, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		hs, errHs := strconv.ParseInt(fields[4], 10, 64)
		rx, errRx := strconv.ParseInt(fields[5], 10, 64)
		tx, errTx := strconv.ParseInt(fields[6], 10, 64)
		if errHs != nil || errRx != nil || errTx != nil {
			continue
		}
		peers = append(peers, peerStat{PublicKey: fields[0], Rx: rx, Tx: tx, LastHandshake: hs})
	}
	return peers, true
}
