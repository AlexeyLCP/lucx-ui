// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type managed struct {
	proc        *Process
	tag         string
	fingerprint string
	peerFP      string
	ifname      string
	onlineTTL   int64
	// Traffic baselines per peer public key, so CollectTraffic returns deltas.
	lastRx   map[string]int64
	lastTx   map[string]int64
	haveLast bool
	peers    []PeerSpec
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
	managerOnce   sync.Once
	manager       *Manager
	rebuildPaused atomic.Bool
)

func SetRebuildPause(v bool) {
	rebuildPaused.Store(v)
}

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
	if rebuildPaused.Load() {
		return fmt.Errorf("awg: module rebuild in progress")
	}
	m.sweepOrphansLocked()
	if err := m.ensureLocked(inst); err != nil {
		return err
	}
	m.ensureXrayRouting(inst)
	m.ensureNatRules(inst)
	m.ensurePortForwards(inst)
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
	conf := renderServerConf(inst)
	fp := deviceFingerprint(conf)
	peerFP := inst.peerFingerprint()
	ttl := onlineTTLSeconds(inst)
	if cur, ok := m.procs[inst.Id]; ok {
		if cur.fingerprint == fp && cur.proc.IsRunning() {
			cur.tag = inst.Tag
			cur.onlineTTL = ttl
			cur.peers = inst.Peers
			if cur.peerFP != peerFP {
				if err := m.syncPeersLocked(inst, conf); err != nil {
					return err
				}
				cur.peerFP = peerFP
				cur.lastRx = map[string]int64{}
				cur.lastTx = map[string]int64{}
				cur.haveLast = false
			}
			return nil
		}
		_ = cur.proc.Stop()
		delete(m.procs, inst.Id)
	}
	if err := writeServerConfig(inst.Id, conf); err != nil {
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
		peerFP:      peerFP,
		ifname:      inst.Ifname,
		onlineTTL:   ttl,
		lastRx:      map[string]int64{},
		lastTx:      map[string]int64{},
		peers:       inst.Peers,
	}
	logger.Infof("awg: started interface %s for inbound %d on port %d", inst.Ifname, inst.Id, inst.Port)
	return nil
}

// Remove stops and forgets the AWG interface for an inbound id. The .conf is
// handled unconditionally, not only when the interface is in procs: an inbound
// whose interface never came up (failed setconf/route on the last reconcile)
// has no procs entry, yet its .conf still sits in awgConfigDir and must not
// survive deleting the inbound (tester report: deleted inbound left awg6.conf).
// lucx.67: the conf is moved to awgBackupDir rather than deleted, so a removed
// inbound's config is never lost; only if the backup itself fails is the file
// deleted (it is LucX-UI's own, confirmed by the ownership marker).
func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.procs[id]; ok {
		_ = cur.proc.Stop()
		delete(m.procs, id)
		logger.Infof("awg: stopped interface %s for inbound %d", cur.ifname, id)
	}
	path := configPathForID(id)
	if _, err := os.Stat(path); err == nil {
		if berr := backupConfigFile(path); berr != nil {
			logger.Warningf("awg: remove: could not back up %s, deleting: %v", path, berr)
			_ = os.Remove(path)
		} else {
			logger.Infof("awg: remove: backed up config %s", path)
		}
	}
}

// Reconcile drives the running set toward the desired instances: it stops
// interfaces that are no longer wanted and (re)starts the rest. Used at boot
// and periodically to recover from crashes.
func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rebuildPaused.Load() {
		return
	}
	m.sweepOrphansLocked()
	want := make(map[int]struct{}, len(desired))
	for _, inst := range desired {
		want[inst.Id] = struct{}{}
	}
	for id, cur := range m.procs {
		if _, ok := want[id]; !ok {
			_ = cur.proc.Stop()
			delete(m.procs, id)
			// lucx.67: back up rather than delete (see Remove).
			path := configPathForID(id)
			if _, err := os.Stat(path); err == nil {
				if berr := backupConfigFile(path); berr != nil {
					_ = os.Remove(path)
				}
			}
		}
	}
	// Sweep leftover awg{N}.conf whose inbound is no longer wanted even though
	// no procs entry existed for it — an inbound deleted after its interface
	// failed to come up (or never started) leaves a stale file the procs loop
	// above cannot see. Only inbound confs (awg{digits}.conf) are touched; the
	// outbound subsystem's awgo-*.conf files are never matched.
	sweepOrphanInboundConfigs(want)
	// lucx.67: mark pre-lucx.67 LucX-UI configs (created before the ownership
	// marker existed) so a later deletion/sweep backs them up instead of leaving
	// them. Re-writing is content-identical (renderServerConf is deterministic)
	// plus the marker line and does not change the fingerprint, so no interface
	// restart is triggered. Idempotent: stops once the marker is present.
	for _, inst := range desired {
		path := configPathForID(inst.Id)
		if _, err := os.Stat(path); err == nil && !configIsManaged(path) {
			_ = writeServerConfigFile(inst)
		}
	}
	for _, inst := range desired {
		if err := m.ensureLocked(inst); err != nil {
			logger.Warningf("awg: reconcile failed for inbound %d: %v", inst.Id, err)
			continue
		}
		m.ensureXrayRouting(inst)
		m.ensureNatRules(inst)
		m.ensurePortForwards(inst)
	}
}

// sweepOrphanInboundConfigs removes awg{N}.conf files in awgConfigDir whose
// inbound id N is not in want. Best-effort: a missing/unreadable dir is a
// no-op. It only matches the inbound naming (awg{digits}.conf); awgo-*.conf
// (outbound client tunnels) never parse as an inbound id and are left alone.
//
// lucx.67: a config is swept only when LucX-UI created it (it carries
// xuiManagedMarker). Foreign configs that share the awg{N}.conf naming — most
// notably WGDashboard's own awg0.conf — are left untouched, and swept configs
// are moved to awgBackupDir instead of being deleted, so nothing is destroyed
// irreversibly (previously the sweep wiped WGDashboard's configs every 10s and
// restored-from-backup files vanished again immediately).
func sweepOrphanInboundConfigs(want map[int]struct{}) {
	entries, err := os.ReadDir(awgConfigDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		id, ok := parseInboundConfName(e.Name())
		if !ok {
			continue
		}
		if _, wanted := want[id]; wanted {
			continue
		}
		path := filepath.Join(awgConfigDir, e.Name())
		if !configIsManaged(path) {
			continue
		}
		if err := backupConfigFile(path); err != nil {
			logger.Warningf("awg: sweep: could not back up orphan %s (leaving in place): %v", path, err)
		} else {
			logger.Infof("awg: sweep: backed up orphan config %s", path)
		}
	}
}

// configIsManaged reports whether the .conf at path carries the x-ui ownership
// marker. A missing/unreadable file or an absent marker means the config is not
// LucX-UI's and must be left alone.
func configIsManaged(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(string(data), xuiManagedMarker)
}

// strayInterfaceIsOurs reports whether a live awgN interface belongs to LucX.
// No .conf or an unmarked file (toolza / docker / WGDashboard) is foreign.
func strayInterfaceIsOurs(ifname string) bool {
	return configIsManaged(filepath.Join(awgConfigDir, ifname+".conf"))
}

// ConfigPathIsManaged is the exported form of configIsManaged for the importer.
func ConfigPathIsManaged(path string) bool {
	return configIsManaged(path)
}

// BackupForeignConf moves a foreign .conf into x-ui-backup after a successful import.
func BackupForeignConf(path string) error {
	return backupConfigFile(path)
}

// Adopt takes over a live foreign interface for a newly created inbound:
// rename it to awg{id} when needed, write the managed .conf, and register it
// without awg-quick down/up so existing handshakes survive.
// startIfDown is false for userspace/docker: the inbound is already in the DB
// and reconcile will bring the kernel iface up after the operator stops the
// old manager. Calling Start while the old process still holds the UDP port
// would fail the whole import after a successful AddInbound.
func (m *Manager) Adopt(inst Instance, currentIfname string, startIfDown bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if currentIfname != "" && currentIfname != inst.Ifname {
		if err := renameAwgInterface(currentIfname, inst.Ifname); err != nil {
			return err
		}
	}
	conf := renderServerConf(inst)
	if err := writeServerConfig(inst.Id, conf); err != nil {
		return err
	}
	proc := newProcess(inst.Ifname, configPathForID(inst.Id), fmt.Sprintf("inbound %d", inst.Id))
	if !proc.IsRunning() && startIfDown {
		if err := proc.Start(); err != nil {
			return err
		}
	}
	m.procs[inst.Id] = &managed{
		proc:        proc,
		tag:         inst.Tag,
		fingerprint: deviceFingerprint(conf),
		peerFP:      inst.peerFingerprint(),
		ifname:      inst.Ifname,
		onlineTTL:   onlineTTLSeconds(inst),
		lastRx:      map[string]int64{},
		lastTx:      map[string]int64{},
	}
	logger.Infof("awg: adopted interface %s as %s for inbound %d", currentIfname, inst.Ifname, inst.Id)
	return nil
}

// backupConfigFile moves the .conf at path into awgBackupDir with a unix-time
// suffix so repeated backups of the same name never overwrite each other.
func backupConfigFile(path string) error {
	backupDir := awgBackupDir()
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return err
	}
	dst := filepath.Join(backupDir, fmt.Sprintf("%s.%d", filepath.Base(path), time.Now().Unix()))
	return os.Rename(path, dst)
}

// parseInboundConfName extracts the inbound id from an "awg{N}.conf" file
// name. Returns false for anything else — notably "awgo-{N}.conf" (outbound
// tunnels), whose segment after "awg" starts with a letter, not a digit.
func parseInboundConfName(name string) (int, bool) {
	const suffix = ".conf"
	if !strings.HasPrefix(name, "awg") || !strings.HasSuffix(name, suffix) {
		return 0, false
	}
	mid := name[len("awg") : len(name)-len(suffix)]
	if mid == "" {
		return 0, false
	}
	for _, r := range mid {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.Atoi(mid)
	if err != nil {
		return 0, false
	}
	return id, true
}

// StopAll stops every managed AWG interface. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cur := range m.procs {
		_ = cur.proc.Stop()
		path := configPathForID(id)
		if _, err := os.Stat(path); err == nil {
			if berr := backupConfigFile(path); berr != nil {
				_ = os.Remove(path)
			}
		}
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
		id        int
		ifname    string
		tag       string
		haveLast  bool
		onlineTTL int64
		lastRx    map[string]int64
		lastTx    map[string]int64
	}
	m.mu.Lock()
	snaps := make([]snap, 0, len(m.procs))
	for id, cur := range m.procs {
		if cur.proc == nil || !cur.proc.IsRunning() {
			continue
		}
		ttl := cur.onlineTTL
		if ttl <= 0 {
			ttl = handshakeOnlineTTL
		}
		snaps = append(snaps, snap{
			id:        id,
			ifname:    cur.ifname,
			tag:       cur.tag,
			haveLast:  cur.haveLast,
			onlineTTL: ttl,
			lastRx:    cur.lastRx,
			lastTx:    cur.lastTx,
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
			if peer.LastHandshake > 0 && now-peer.LastHandshake <= s.onlineTTL {
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
	return writeServerConfig(inst.Id, renderServerConf(inst))
}

// writeServerConfig takes already-rendered text so a caller that also needs it
// for the fingerprint does not pay a second render (and second module probe).
func writeServerConfig(id int, conf string) error {
	if err := os.MkdirAll(awgConfigDir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(configPathForID(id), []byte(conf), 0o600)
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

var lastDefaultRouteIface atomic.Value

// stickyDefaultRoute keeps the last interface seen. `ip route show default`
// exits 0 with no output between DHCP leases, and "" erases PostUp — the fingerprint.
func stickyDefaultRoute(probed string) string {
	if probed != "" {
		lastDefaultRouteIface.Store(probed)
		return probed
	}
	last, _ := lastDefaultRouteIface.Load().(string)
	return last
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
	// Subnet MASQUERADE is the reliable path (mark-only NAT silently fails
	// when mangle PREROUTING is missing after a routeThroughXray toggle).
	// MARK+MASQUERADE-by-mark still covers peers whose AllowedIPs sit outside
	// the server Address/24.
	mark := awgNatMark(inst.Id)
	postUp = fmt.Sprintf(
		"echo 1 > /proc/sys/net/ipv4/ip_forward; "+
			"iptables -t mangle -C PREROUTING -i %s -j MARK --set-mark %d 2>/dev/null || "+
			"iptables -t mangle -A PREROUTING -i %s -j MARK --set-mark %d; "+
			"iptables -t nat -C POSTROUTING -s %s -o %s -j MASQUERADE 2>/dev/null || "+
			"iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE; "+
			"iptables -t nat -C POSTROUTING -m mark --mark %d -o %s -j MASQUERADE 2>/dev/null || "+
			"iptables -t nat -A POSTROUTING -m mark --mark %d -o %s -j MASQUERADE; "+
			"iptables -C FORWARD -i %s -j ACCEPT 2>/dev/null || "+
			"iptables -A FORWARD -i %s -j ACCEPT; "+
			"iptables -C FORWARD -o %s -j ACCEPT 2>/dev/null || "+
			"iptables -A FORWARD -o %s -j ACCEPT; "+
			"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
			"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu; "+
			"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || "+
			"iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu",
		iface, mark, iface, mark,
		subnet, extIface, subnet, extIface,
		mark, extIface, mark, extIface,
		iface, iface, iface, iface,
		iface, iface, iface, iface,
	)
	postDown = fmt.Sprintf(
		"iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE 2>/dev/null || true; "+
			"iptables -t nat -D POSTROUTING -m mark --mark %d -o %s -j MASQUERADE 2>/dev/null || true; "+
			"iptables -t mangle -D PREROUTING -i %s -j MARK --set-mark %d 2>/dev/null || true; "+
			"iptables -D FORWARD -i %s -j ACCEPT 2>/dev/null || true; "+
			"iptables -D FORWARD -o %s -j ACCEPT 2>/dev/null || true; "+
			"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -o %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true; "+
			"iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -i %s -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true",
		subnet, extIface,
		mark, extIface,
		iface, mark,
		iface, iface,
		iface, iface,
	)
	return postUp, postDown
}

func awgNatMark(id int) int {
	if id < 0 {
		id = 0
	}
	return 0xA0000 | (id & 0xFFFF)
}

// natRule is one idempotent iptables rule: probed with `-t <table> -C <chain>
// <spec>` and appended with `-A` only when the probe fails.
type natRule struct {
	table string
	chain string
	spec  []string
}

// natRulesFor returns the rule set a kernel-routed (non-routeThroughXray)
// instance needs: mark packets from awgN + MASQUERADE by mark out the
// external interface, plus FORWARD accepts for both awgN legs. Nil when the
// instance is unroutable (Xray-routed, no ifname, or no external interface).
func natRulesFor(inst Instance, extIface string) []natRule {
	if inst.RouteThroughXray || inst.Ifname == "" || extIface == "" {
		return nil
	}
	if clientSubnet(inst.Address) == "" {
		return nil
	}
	mark := strconv.Itoa(awgNatMark(inst.Id))
	subnet := clientSubnet(inst.Address)
	return []natRule{
		{"mangle", "PREROUTING", []string{"-i", inst.Ifname, "-j", "MARK", "--set-mark", mark}},
		{"nat", "POSTROUTING", []string{"-s", subnet, "-o", extIface, "-j", "MASQUERADE"}},
		{"nat", "POSTROUTING", []string{"-m", "mark", "--mark", mark, "-o", extIface, "-j", "MASQUERADE"}},
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
	_ = exec.CommandContext(context.Background(), "ip", "rule", "del", "iif", inst.Ifname, "lookup", strconv.Itoa(awgRouteTable(inst.Id))).Run()
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
	// Ownership marker (lucx.67): the orphan sweep only backs up configs carrying
	// this line and leaves foreign configs (e.g. WGDashboard's) untouched. It is
	// a '#' comment, invisible to awg-quick.
	b.WriteString(xuiManagedMarker + "\n")
	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", confValue(inst.PrivateKey))
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.Port)
	if inst.Address != "" {
		fmt.Fprintf(&b, "Address = %s\n", confValue(inst.Address))
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
	// S3/S4 are AWG v2+ only. Emitting them on a v1.5 server while the
	// client export strips them (filterAwgObfuscation) breaks must-match
	// junk lengths → handshake never completes.
	if NormalizeAWGVersion(inst.AwgVersion) != "1.5" {
		fmt.Fprintf(&b, "S3 = %d\n", inst.S3)
		fmt.Fprintf(&b, "S4 = %d\n", inst.S4)
	}
	// Per field, like the client export: only a blank one is skipped, because
	// "H1 = " alone makes setconf reject the whole file.
	for i, h := range []string{inst.H1, inst.H2, inst.H3, inst.H4} {
		if strings.TrimSpace(h) != "" {
			fmt.Fprintf(&b, "H%d = %s\n", i+1, confValue(h))
		}
	}
	awg3ok := IsAwg3Plus(inst.AwgVersion) && ModuleSupportsAwg3()
	// HeaderProtectionKey (AWG3) is written ONLY when AwgVersion == "3" and the
	// key is non-empty. The upstream kernel module (v3.0.20260731) + tools
	// (v3.0.20260730) now parse the field; older builds reject it with "Line
	// unrecognized: `HeaderProtectionKey=...`" + "Configuration parsing error",
	// awg-quick deletes the half-built interface, and reconcile fails every 10s
	// so the inbound never serves traffic. Version-gating keeps v1/v2 inbounds
	// working on any kernel, and lets a v3 inbound opt in once the operator has
	// installed the AWG3 module. The S1-S4 >= 12 invariant (enforced by the
	// generator) is required for the kernel to accept the key.
	if awg3ok && strings.TrimSpace(inst.HeaderProtectionKey) != "" {
		fmt.Fprintf(&b, "HeaderProtectionKey = %s\n", confValue(inst.HeaderProtectionKey))
	}
	// AWG3 device-level timers/padding — all optional (0 = kernel uses the
	// built-in WireGuard constant). Written only when > 0 and IsAwg3Plus;
	// older kernels reject these lines in setconf. Module-gated: if the
	// installed amneziawg module is < v3.0 (no ContentPaddingAddition/etc
	// support), even an AwgVersion="3" inbound must not emit the lines — the
	// kernel rejects "Line unrecognized" and awg-quick deletes the interface,
	// producing "Device <awgN> does not exist". Blocks the regression seen
	// when an operator picks v3 on a host still running the v1.x module.
	if awg3ok {
		// Values are written verbatim: each is a single integer ("150") or an
		// inclusive range ("100-500"). The kernel u16_range_t parses both and
		// randomizes within a range at rekey (same semantics as H1-H4), so a
		// range must reach the .conf intact — never collapsed to one value.
		if !inst.ContentPaddingAddition.IsZero() {
			fmt.Fprintf(&b, "ContentPaddingAddition = %s\n", confValue(string(inst.ContentPaddingAddition)))
		}
		if !inst.RekeyAfterTime.IsZero() {
			fmt.Fprintf(&b, "RekeyAfterTime = %s\n", confValue(string(inst.RekeyAfterTime)))
		}
		if !inst.RekeyTimeout.IsZero() {
			fmt.Fprintf(&b, "RekeyTimeout = %s\n", confValue(string(inst.RekeyTimeout)))
		}
		if !inst.RejectAfterTime.IsZero() {
			fmt.Fprintf(&b, "RejectAfterTime = %s\n", confValue(string(inst.RejectAfterTime)))
		}
		if !inst.KeepaliveTimeout.IsZero() {
			fmt.Fprintf(&b, "KeepaliveTimeout = %s\n", confValue(string(inst.KeepaliveTimeout)))
		}
		if !inst.MaxHandshakeAttempts.IsZero() {
			fmt.Fprintf(&b, "MaxHandshakeAttempts = %s\n", confValue(string(inst.MaxHandshakeAttempts)))
		}
	}
	// AWG 3.1 device flags — omitted when false so v3.0 tools keep accepting
	// the config. Tools-gated: v3.0 awg-quick rejects "Line unrecognized".
	if IsAwg31(inst.AwgVersion) && ModuleSupportsAwg31() {
		// awg-tools parse_bool accepts on/off/0/1 — not "true"/"false"
		// (E2E lucx.117: "Boolean value is neither on/off nor 0/1").
		if inst.RandomTrailers {
			b.WriteString("RandomTrailers = on\n")
		}
		if inst.DisableCookies {
			b.WriteString("DisableCookies = on\n")
		}
	}
	// Worst-case, not this ifname: the exported client .conf and the share link
	// budget that way, and a set only one side writes is one-way mimicry.
	if NormalizeAWGVersion(inst.AwgVersion) != "1.5" &&
		IBytes(inst.I1, inst.I2, inst.I3, inst.I4, inst.I5) <= WorstCaseIBytesBudget(strings.TrimSpace(inst.HeaderProtectionKey) != "") {
		for _, kv := range []struct{ k, v string }{
			{"I1", inst.I1}, {"I2", inst.I2}, {"I3", inst.I3}, {"I4", inst.I4}, {"I5", inst.I5},
		} {
			if strings.TrimSpace(kv.v) != "" {
				fmt.Fprintf(&b, "%s = %s\n", kv.k, kv.v)
			}
		}
	}
	if postUp, postDown := natPostUpPostDown(inst); postUp != "" {
		fmt.Fprintf(&b, "PostUp = %s\n", postUp)
		fmt.Fprintf(&b, "PostDown = %s\n", postDown)
	}
	for _, p := range inst.Peers {
		b.WriteString("\n[Peer]\n")
		fmt.Fprintf(&b, "PublicKey = %s\n", confValue(p.PublicKey))
		// PresharedKey is written ONLY when non-empty. An empty value renders as
		// "PresharedKey = " which awg setconf rejects ("invalid key"), awg-quick
		// rolls back the half-built interface, and reconcile reports "Device
		// <awgN> does not exist" — the crash seen when a client reaches this
		// renderer without a generated PSK (e.g. the inbound-form update path,
		// which unlike the clients-page path does not run defaultAwgClients).
		// Absent PresharedKey is the WireGuard convention for "no PSK" and
		// matches renderClientConf + SyncPeers, which already omit it.
		if psk := strings.TrimSpace(p.PSK); psk != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", confValue(psk))
		}
		allowed := p.AllowedIPs
		if allowed == "" {
			allowed = "0.0.0.0/0, ::/0"
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", confValue(allowed))
	}
	return b.String()
}

var awgQuickIfaceKeys = map[string]struct{}{
	"address": {}, "mtu": {}, "dns": {}, "table": {},
	"preup": {}, "predown": {}, "postup": {}, "postdown": {},
	"saveconfig": {},
}

func stripAwgQuick(conf string) string {
	var b strings.Builder
	inInterface := false
	for i, line := range strings.Split(conf, "\n") {
		keyLine := line
		if idx := strings.Index(keyLine, "#"); idx >= 0 {
			keyLine = keyLine[:idx]
		}
		key := strings.TrimSpace(keyLine)
		if eq := strings.Index(key, "="); eq >= 0 {
			key = strings.TrimSpace(key[:eq])
		}
		if strings.HasPrefix(key, "[") {
			inInterface = strings.EqualFold(key, "[Interface]")
		} else if inInterface {
			if _, drop := awgQuickIfaceKeys[strings.ToLower(key)]; drop {
				continue
			}
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

func (m *Manager) syncPeersLocked(inst Instance, conf string) error {
	// Full .conf stays on disk for awg-quick up; syncconf gets a stripped
	// temp file — its parser rejects Address/MTU/PostUp (lucx.154).
	if err := writeServerConfig(inst.Id, conf); err != nil {
		return err
	}
	cur, ok := m.procs[inst.Id]
	if !ok || cur.proc == nil || !cur.proc.IsRunning() {
		return nil
	}
	tmp, err := os.CreateTemp("", "awg-sync-*.conf")
	if err != nil {
		return fmt.Errorf("awg syncconf temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(stripAwgQuick(conf)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("awg syncconf temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("awg syncconf temp: %w", err)
	}
	out, err := exec.CommandContext(context.Background(), awgBin("awg"), "syncconf", inst.Ifname, tmpPath).CombinedOutput()
	if err != nil {
		logger.Warningf("awg: syncconf %s: %v\n%s", inst.Ifname, err, string(out))
		return fmt.Errorf("awg syncconf %s: %w\n%s", inst.Ifname, err, string(out))
	}
	return nil
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
	Endpoint      string
	AllowedIPs    string
	Rx            int64
	Tx            int64
	LastHandshake int64
}

// awgShowTimeout bounds one dump: a healthy read is a single netlink round trip
// (~1 ms measured), while an oversized I-field set spins it for ~30 minutes.
const awgShowTimeout = 5 * time.Second

// awgStuckCooldown leaves a timed-out interface unread for this long: the read
// itself leaks ~164 MB/s. A var (not a const) so tests can shorten it.
var awgStuckCooldown = 10 * time.Minute

// stuckShows maps an interface to when its read last ran out of time; three
// readers share it, so one sick device is one warning and one read per cooldown.
var stuckShows sync.Map

// noteAwgRead turns the outcome of one bounded read of ifname into at most one
// log line while the device is unreadable; answering again re-arms the warning.
func noteAwgRead(ctx context.Context, ifname string, err error) {
	if err == nil {
		stuckShows.Delete(ifname)
		return
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return
	}
	if _, warned := stuckShows.Swap(ifname, time.Now()); !warned {
		logger.Warningf("awg: reading interface %s timed out after %s, leaving it unread for %s", ifname, awgShowTimeout, awgStuckCooldown)
	}
}

// stuckReadErr answers for an interface still inside its cooldown, so a reader
// reports it unreadable without spawning the read that wedges and leaks.
func stuckReadErr(ifname string) error {
	v, ok := stuckShows.Load(ifname)
	if !ok {
		return nil
	}
	stuck, _ := v.(time.Time)
	if time.Since(stuck) >= awgStuckCooldown {
		return nil
	}
	return fmt.Errorf("awg show %s: %w", ifname, context.DeadlineExceeded)
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
	if stuckReadErr(ifname) != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), awgShowTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, awgBin("awg"), "show", ifname, "dump").Output()
	noteAwgRead(ctx, ifname, err)
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
		ep, allowed := fields[2], fields[3]
		if ep == "(off)" {
			ep = ""
		}
		peers = append(peers, peerStat{
			PublicKey: fields[0], Endpoint: ep, AllowedIPs: allowed,
			Rx: rx, Tx: tx, LastHandshake: hs,
		})
	}
	return peers, true
}
