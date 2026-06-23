// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type managed struct {
	proc       *Process
	tag        string
	fingerprint string
	ifname     string
	// Traffic baselines per peer public key, so CollectTraffic returns deltas.
	lastRx map[string]int64
	lastTx map[string]int64
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
	return m.ensureLocked(inst)
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
		}
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
		id      int
		ifname  string
		tag     string
		haveLast bool
		lastRx  map[string]int64
		lastTx  map[string]int64
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

// renderServerConf builds the awg-quick .conf for an instance. The layout
// matches BuildServerConfig but reads from the Instance struct (desired
// runtime state) rather than the inbound's stored JSON.
func renderServerConf(inst Instance) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", inst.PrivateKey)
	fmt.Fprintf(&b, "ListenPort = %d\n", inst.Port)
	fmt.Fprintf(&b, "MTU = %d\n", inst.MTU)
	if inst.DNS != "" {
		fmt.Fprintf(&b, "DNS = %s\n", inst.DNS)
	}
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
	if inst.I1 != "" {
		fmt.Fprintf(&b, "I1 = <b 0x%s>\n", inst.I1)
		fmt.Fprintf(&b, "I2 = <b 0x%s>\n", inst.I2)
	}
	if inst.I3 != "" {
		fmt.Fprintf(&b, "I3 = <b 0x%s>\n", inst.I3)
		fmt.Fprintf(&b, "I4 = <b 0x%s>\n", inst.I4)
		fmt.Fprintf(&b, "I5 = <b 0x%s>\n", inst.I5)
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
		if out, err := exec.Command("awg", args...).CombinedOutput(); err != nil {
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
			if out, err := exec.Command("awg", "set", ifname, "peer", pub, "remove").CombinedOutput(); err != nil {
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
	out, err := exec.Command("awg", "show", ifname, "peers").Output()
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