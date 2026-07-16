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

// CollectTraffic scrapes each running AWG interface via `awg show transfer`
// and returns the per-inbound byte deltas since the previous scrape.
func (m *Manager) CollectTraffic() []Traffic {
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
	for _, s := range snaps {
		rx, tx, ok := scrapeTransfer(s.ifname)
		if !ok {
			continue
		}
		// AWG `show transfer` returns cumulative counters per peer; we sum
		// them above into rx/tx. Deltas are computed against the previous
		// cumulative total (not per-peer), which is correct as long as peers
		// are only added/removed via Reconcile (which resets the baseline).
		var du, dd int64
		if s.haveLast {
			du = rx
			dd = tx
			// Subtract the previous cumulative total. We track the sum of all
			// peer counters, not per-peer, so a peer leaving the interface
			// would cause a negative delta — clamped to zero below.
			prevRx := sumInt64(s.lastRx)
			prevTx := sumInt64(s.lastTx)
			du -= prevRx
			dd -= prevTx
			if du < 0 {
				du = 0
			}
			if dd < 0 {
				dd = 0
			}
		}
		// Re-acquire lock to persist the new baseline.
		m.mu.Lock()
		if cur, ok := m.procs[s.id]; ok {
			cur.lastRx = map[string]int64{"_total": rx}
			cur.lastTx = map[string]int64{"_total": tx}
			cur.haveLast = true
		}
		m.mu.Unlock()

		if s.haveLast && (du > 0 || dd > 0) {
			out = append(out, Traffic{Tag: s.tag, Up: du, Down: dd})
		}
	}
	return out
}

func sumInt64(m map[string]int64) int64 {
	var total int64
	for _, v := range m {
		total += v
	}
	return total
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
				"ip rule del iif %s lookup %d 2>/dev/null || true; "+
				"ip rule add iif %s lookup %d",
			iface,
			iface, iface, iface, iface,
			tunName, tunName, tunName, tunName,
			iface, table, iface, table,
		)
		postDown = fmt.Sprintf(
			"ip rule del iif %s lookup %d 2>/dev/null || true; "+
				"ip route flush table %d 2>/dev/null || true; "+
				"iptables -D FORWARD -i %s -j ACCEPT 2>/dev/null || true; "+
				"iptables -D FORWARD -o %s -j ACCEPT 2>/dev/null || true; "+
				"iptables -D FORWARD -i %s -j ACCEPT 2>/dev/null || true; "+
				"iptables -D FORWARD -o %s -j ACCEPT 2>/dev/null || true",
			iface, table, table,
			iface, iface, tunName, tunName,
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
			"iptables -A FORWARD -o %s -j ACCEPT",
		subnet, extIface, subnet, extIface,
		iface, iface, iface, iface,
	)
	postDown = fmt.Sprintf(
		"iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE 2>/dev/null || true; "+
			"iptables -D FORWARD -i %s -j ACCEPT 2>/dev/null || true; "+
			"iptables -D FORWARD -o %s -j ACCEPT 2>/dev/null || true",
		subnet, extIface,
		iface, iface,
	)
	return postUp, postDown
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
		fmt.Fprintf(&b, "PresharedKey = %s\n", p.PSK)
		allowed := p.AllowedIPs
		if allowed == "" {
			allowed = "0.0.0.0/0, ::/0"
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", allowed)
		if p.Keepalive > 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.Keepalive)
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
