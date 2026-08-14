// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Instance is the desired runtime state of one tunnel core, built by the web
// layer from the stored operator config. The Manager converges the actual
// process state toward it on every Ensure call.
//
// Key selects the manager slot and on-disk paths. Empty Key falls back to
// string(Core) (legacy single-core layout for olcRTC/qWDTT and the global
// Naive tunnel). Naive inbounds use "naive-{id}" so N panel inbounds map to
// N supervised caddy processes with isolated ACME storage.
type Instance struct {
	Core    Name
	Key     string
	Enabled bool

	// ConfigText is the fully rendered config file content (Caddyfile for
	// NaiveProxy, YAML for olcRTC). Empty means the core is CLI-only
	// (qWDTT) and writeConfigFile is a no-op.
	ConfigText string

	// Args, when non-empty, are the full argv passed to the binary
	// (qWDTT CLI flags). Takes precedence over the core-specific switch
	// in start().
	Args []string

	// ExtraFiles carries companion config files (filename → content) written
	// next to the main config (TrustTunnel hosts/credentials/rules TOML).
	// Covered by Fingerprint like ConfigText.
	ExtraFiles map[string]string

	// FingerprintExtra carries opaque material (e.g. the TLS cert content
	// hash) into the Fingerprint without being written to disk — a renewed
	// certificate restarts the core even though file paths are unchanged.
	FingerprintExtra string

	// ExtraArgs are additional CLI arguments appended after the standard
	// ones (operator escape hatch, space-separated). Ignored when Args is set.
	ExtraArgs string

	// ProbePort is the local TCP port used by the listening/responding
	// health probes. Zero disables the probes (running status only).
	ProbePort int

	// RouteThroughXray (qWDTT): policy-route kernel TUN ifaces into an Xray
	// TUN inbound instead of the binary's MASQUERADE. TunName / RouteTable /
	// RouteIfaces are set only when true; Fingerprint includes them so a
	// toggle restarts the process (NAT rebuild).
	RouteThroughXray bool
	TunName          string
	RouteTable       int
	RouteIfaces      []string
}

// ManageKey returns the manager map key for this instance.
func (inst Instance) ManageKey() string {
	if k := strings.TrimSpace(inst.Key); k != "" {
		return k
	}
	return string(inst.Core)
}

// Fingerprint identifies the instance's rendered config plus CLI shape. Any
// change to either forces a restart on the next Ensure.
func (inst Instance) Fingerprint() string {
	extra := ""
	if len(inst.ExtraFiles) > 0 {
		names := make([]string, 0, len(inst.ExtraFiles))
		for name := range inst.ExtraFiles {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		for _, name := range names {
			b.WriteString(name)
			b.WriteByte('=')
			b.WriteString(inst.ExtraFiles[name])
			b.WriteByte(';')
		}
		extra = b.String()
	}
	sum := sha256.Sum256([]byte(
		inst.ConfigText + "\x00" +
			inst.ExtraArgs + "\x00" +
			strings.Join(inst.Args, "\x00") + "\x00" +
			extra + "\x00" +
			inst.FingerprintExtra + "\x00" +
			fmt.Sprintf("rtx=%v|tun=%s|tbl=%d", inst.RouteThroughXray, inst.TunName, inst.RouteTable),
	))
	return hex.EncodeToString(sum[:16])
}

// Status is the three-level health snapshot of one core, mirroring the
// elector1337/3x-ui-naive semantics: Running (process alive) -> Listening
// (TCP probe answered) -> Responding (TLS handshake completed). A process
// that is Running but not Responding is hung or misconfigured, which the UI
// renders as a distinct warning state.
type Status struct {
	Running    bool `json:"running"`
	Listening  bool `json:"listening"`
	Responding bool `json:"responding"`
}

// managed is one supervised key. opMu serialises the slow lifecycle operations
// (stop, config write, exec) of THIS key only, so a core that takes the full
// SIGTERM grace period to die cannot block status reads or the lifecycle of
// every other core the way a single manager-wide lock did.
type managed struct {
	opMu sync.Mutex
	proc *Proc
	fp   string
}

// Manager owns the supervised tunnel-core processes keyed by ManageKey
// (legacy core name or "naive-{id}" for inbound instances).
//
// Locking: mu guards the maps and nothing else — every critical section under
// it is a map lookup or a field read, never process I/O. Per-key lifecycle
// work runs under managed.opMu, taken after mu has been released.
type Manager struct {
	mu    sync.Mutex
	swept bool
	// sweepEnabled is set only on the shared instance built by GetManager.
	// The orphan sweep SIGKILLs every process named like one of our cores,
	// which is right for the running panel and very wrong for `go test` on a
	// developer machine that happens to have a panel running next to it.
	sweepEnabled       bool
	cores              map[string]*managed
	naiveTraffic       map[string]*naiveLogCursor
	mieruTraffic       map[string]map[string]*mieruUserCursor
	trustTunnelTraffic map[string]*trustTunnelCursor
}

func newManager() *Manager {
	return &Manager{
		cores:              make(map[string]*managed),
		mieruTraffic:       make(map[string]map[string]*mieruUserCursor),
		trustTunnelTraffic: make(map[string]*trustTunnelCursor),
	}
}

var (
	managerMu  sync.Mutex
	managerRef *Manager
)

// GetManager returns the shared manager instance.
func GetManager() *Manager {
	managerMu.Lock()
	defer managerMu.Unlock()
	if managerRef == nil {
		managerRef = newManager()
		managerRef.sweepEnabled = true
	}
	return managerRef
}

// slot returns the managed record for key, creating it on first use. Callers
// must hold m.mu.
func (m *Manager) slot(key string) *managed {
	mc, ok := m.cores[key]
	if !ok {
		// proc is created eagerly and never reassigned, so the read-only
		// accessors below can reach it under mu while a lifecycle operation
		// mutates the process under opMu, with no race on the pointer itself.
		mc = &managed{proc: NewProc(key)}
		m.cores[key] = mc
	}
	return mc
}

// acquire returns the managed record for key, holding m.mu only for the map
// lookup. The returned pointer stays valid after Remove deletes the key, so a
// lifecycle operation already in flight finishes against its own record.
func (m *Manager) acquire(key string) *managed {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.slot(key)
}

// sweepOrphans kills tunnel cores left running by a previous x-ui run, exactly
// once per process lifetime and before any of our own cores are started. The
// panel owns every one of these binaries, so anything alive at this point is a
// survivor of an ungraceful shutdown that is still holding an inbound port.
func (m *Manager) sweepOrphans() {
	m.mu.Lock()
	if m.swept || !m.sweepEnabled {
		m.mu.Unlock()
		return
	}
	m.swept = true
	m.mu.Unlock()

	paths := make([]string, 0, len(All()))
	for _, core := range All() {
		paths = append(paths, core.BinaryPath())
	}
	if n := killStrayTunnelProcesses(paths); n > 0 {
		logger.Warningf("tunnel: terminated %d orphaned sidecar process(es) from a previous run", n)
	}
}

// Ensure converges one core toward the desired instance: disabled -> stopped
// (config preserved); enabled with a changed fingerprint -> restarted;
// enabled but dead (crash or panel restart) -> started. It is the single
// entry point used by the reconcile job, the save flow and manual start.
func (m *Manager) Ensure(inst Instance) error {
	if !inst.Core.Valid() {
		return fmt.Errorf("tunnel: unknown core %q", inst.Core)
	}
	m.sweepOrphans()
	key := inst.ManageKey()
	mc := m.acquire(key)
	mc.opMu.Lock()
	defer mc.opMu.Unlock()

	if !inst.Enabled {
		if mc.proc != nil && mc.proc.IsRunning() {
			if err := mc.proc.Stop(); err != nil {
				return fmt.Errorf("tunnel: stop %s: %w", key, err)
			}
		}
		mc.fp = ""
		return nil
	}

	if err := writeConfigFile(inst); err != nil {
		return err
	}
	fp := inst.Fingerprint()
	if mc.proc != nil && mc.proc.IsRunning() {
		if fp == mc.fp {
			return nil
		}
		if err := mc.proc.Stop(); err != nil {
			logger.Warningf("tunnel: stop before restart of %s failed: %v", key, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := m.start(inst, mc); err != nil {
		return err
	}
	mc.fp = fp
	// qWDTT routeThroughXray: binary always installs MASQUERADE; override
	// with policy routing once the process (and wdtt0) is up.
	if inst.RouteThroughXray {
		go ensureQwdttXrayRouting(inst)
	}
	return nil
}

// EnsureQwdttRouting re-applies policy routes for a running routed qWDTT
// instance (reconcile cron + post-Xray-restart). No-op when not routed or
// process down.
func (m *Manager) EnsureQwdttRouting(inst Instance) {
	if !inst.RouteThroughXray || !inst.Enabled {
		return
	}
	if m.IsRunningKey(inst.ManageKey()) {
		ensureQwdttXrayRouting(inst)
	}
}

// Remove stops and forgets a managed key (inbound delete). For multi-instance
// inbound cores (trusttunnel-N, mieru-N, naive-N) companion config files and
// the data dir are removed so a re-created inbound does not leave orphans
// (tester bravn, lucx.122). Legacy single-key cores keep files on disk.
func (m *Manager) Remove(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	m.mu.Lock()
	mc, ok := m.cores[key]
	delete(m.cores, key)
	m.mu.Unlock()
	if ok {
		// The record is already out of the map, but a concurrent Ensure may
		// still hold it; opMu makes sure we stop the process after that
		// operation finished rather than in the middle of its exec.
		mc.opMu.Lock()
		if mc.proc != nil && mc.proc.IsRunning() {
			if err := mc.proc.Stop(); err != nil {
				logger.Warningf("tunnel: remove stop %s: %v", key, err)
			}
		}
		mc.fp = ""
		mc.opMu.Unlock()
	}
	removeManagedFiles(key)
}

// removeManagedFiles deletes on-disk configs/data for multi-instance keys.
// No-op for legacy single-core keys (naive / olcrtc / qwdtt without suffix).
func removeManagedFiles(key string) {
	if !isMultiInstanceKey(key) {
		return
	}
	var core Name
	switch {
	case strings.HasPrefix(key, "trusttunnel-"):
		core = TrustTunnel
	case strings.HasPrefix(key, "mieru-"):
		core = Mieru
	case strings.HasPrefix(key, "naive-"):
		core = Naive
	case strings.HasPrefix(key, "olcrtc-"):
		core = Olcrtc
	default:
		return
	}
	paths := []string{configPathFor(key, core)}
	if core == TrustTunnel {
		paths = append(paths,
			filepath.Join(workDir(), trustTunnelHostsFileName(key)),
			filepath.Join(workDir(), key+"-credentials.toml"),
			filepath.Join(workDir(), key+"-rules.toml"),
		)
	}
	paths = append(paths, dataDirFor(key, core))
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil && !os.IsNotExist(err) {
			logger.Warningf("tunnel: remove %s: %v", p, err)
		}
	}
}

func isMultiInstanceKey(key string) bool {
	for _, p := range []string{"trusttunnel-", "mieru-", "naive-", "olcrtc-"} {
		if strings.HasPrefix(key, p) && len(key) > len(p) {
			return true
		}
	}
	return false
}

// sweepOrphanConfigFiles removes on-disk multi-instance configs whose key is
// not in want (covers leftovers from older Removes that only stopped the proc).
func sweepOrphanConfigFiles(prefix string, want map[string]struct{}) {
	dir := workDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	seen := map[string]struct{}{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		key := orphanKeyFromFilename(prefix, name)
		if key == "" {
			continue
		}
		if _, ok := want[key]; ok {
			continue
		}
		if _, done := seen[key]; done {
			continue
		}
		seen[key] = struct{}{}
		removeManagedFiles(key)
	}
}

// orphanKeyFromFilename maps trusttunnel-3.toml / trusttunnel-3-hosts.toml →
// trusttunnel-3; mieru-5.json → mieru-5.
func orphanKeyFromFilename(prefix, name string) string {
	rest := strings.TrimPrefix(name, prefix)
	if rest == "" || rest == name {
		return ""
	}
	// "3.toml" / "3-hosts.toml" / "3-data" / "3.caddyfile"
	id := rest
	for _, sep := range []string{"-", "."} {
		if i := strings.Index(id, sep); i > 0 {
			id = id[:i]
			break
		}
	}
	if id == "" {
		return ""
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return prefix + id
}

// Stop terminates the core process if it is running (legacy Name API).
func (m *Manager) Stop(core Name) error {
	return m.StopKey(string(core))
}

// StopKey terminates one managed key if running.
func (m *Manager) StopKey(key string) error {
	mc := m.acquire(key)
	mc.opMu.Lock()
	defer mc.opMu.Unlock()
	mc.fp = ""
	if mc.proc == nil || !mc.proc.IsRunning() {
		return nil
	}
	return mc.proc.Stop()
}

// StopAll terminates every running core; wired into the panel shutdown.
// Cores are stopped concurrently: the panel shutdown path should not take
// (number of cores) x SIGTERM grace period before the process can exit.
func (m *Manager) StopAll() {
	m.mu.Lock()
	slots := make(map[string]*managed, len(m.cores))
	maps.Copy(slots, m.cores)
	m.mu.Unlock()

	var wg sync.WaitGroup
	for key, mc := range slots {
		wg.Add(1)
		go func(key string, mc *managed) {
			defer wg.Done()
			mc.opMu.Lock()
			defer mc.opMu.Unlock()
			if mc.proc != nil && mc.proc.IsRunning() {
				if err := mc.proc.Stop(); err != nil {
					logger.Warningf("tunnel: shutdown stop %s failed: %v", key, err)
				}
			}
			mc.fp = ""
		}(key, mc)
	}
	wg.Wait()
}

// IsRunning reports whether the core process is alive (legacy Name API).
func (m *Manager) IsRunning(core Name) bool {
	return m.IsRunningKey(string(core))
}

// IsRunningKey reports whether the managed key's process is alive.
func (m *Manager) IsRunningKey(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.cores[key]
	return ok && mc.proc != nil && mc.proc.IsRunning()
}

// AnyRunning reports whether any managed key carrying the prefix is alive
// (multi-instance cores on the Cores page: mieru-*, trusttunnel-*).
func (m *Manager) AnyRunning(prefix string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, mc := range m.cores {
		if strings.HasPrefix(key, prefix) && mc.proc != nil && mc.proc.IsRunning() {
			return true
		}
	}
	return false
}

// Logs returns up to max recent output lines of the core process.
func (m *Manager) Logs(core Name, max int) []string {
	return m.LogsKey(string(core), max)
}

// LogsPrefixed collects up to max recent lines of every managed key carrying
// the prefix, tagging each line with its key (multi-instance cores on the
// Cores page: mieru-*, trusttunnel-*).
func (m *Manager) LogsPrefixed(prefix string, max int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0)
	for key := range m.cores {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var out []string
	for _, key := range keys {
		if mc := m.cores[key]; mc.proc != nil {
			for _, line := range mc.proc.Lines(max) {
				out = append(out, "["+key+"] "+line)
			}
		}
	}
	return out
}

// LastLogPrefixed returns the most recent line of any instance with the
// prefix (first in sorted key order that has output).
func (m *Manager) LastLogPrefixed(prefix string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0)
	for key := range m.cores {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if mc := m.cores[key]; mc.proc != nil {
			if line := mc.proc.LastLine(); line != "" {
				return line
			}
		}
	}
	return ""
}

// StopPrefixed stops every running managed key with the prefix (binary
// deletion of a multi-instance core).
func (m *Manager) StopPrefixed(prefix string) {
	m.mu.Lock()
	var keys []string
	for key, mc := range m.cores {
		if strings.HasPrefix(key, prefix) && mc.proc != nil && mc.proc.IsRunning() {
			keys = append(keys, key)
		}
	}
	m.mu.Unlock()
	for _, key := range keys {
		if err := m.StopKey(key); err != nil {
			logger.Warningf("tunnel: stop %s: %v", key, err)
		}
	}
}

// LogsKey returns up to max recent output lines for a managed key.
func (m *Manager) LogsKey(key string, max int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.cores[key]
	if !ok || mc.proc == nil {
		return nil
	}
	return mc.proc.Lines(max)
}

// LastLog returns the most recent output line of the core process.
func (m *Manager) LastLog(core Name) string {
	return m.LastLogKey(string(core))
}

// LastLogKey returns the most recent output line for a managed key.
func (m *Manager) LastLogKey(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.cores[key]
	if !ok || mc.proc == nil {
		return ""
	}
	return mc.proc.LastLine()
}

// StatusOf probes the core: process liveness first, then (when running and a
// probe port is known) the TCP listener and the TLS handshake.
func (m *Manager) StatusOf(inst Instance) Status {
	st := Status{Running: m.IsRunningKey(inst.ManageKey())}
	if !st.Running || inst.ProbePort <= 0 {
		return st
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(inst.ProbePort))
	st.Listening = probeListening(addr)
	if st.Listening {
		st.Responding = probeResponding(addr)
	}
	return st
}

// ReconcileNaive drives every desired Naive inbound instance.
func (m *Manager) ReconcileNaive(want []Instance) {
	m.ReconcileWanted(Naive, "naive-", string(Naive), want)
}

// ReconcileOlcrtc drives every desired olcRTC inbound instance.
func (m *Manager) ReconcileOlcrtc(want []Instance) {
	m.ReconcileWanted(Olcrtc, "olcrtc-", string(Olcrtc), want)
}

// ReconcileMieru drives every desired mieru inbound instance.
func (m *Manager) ReconcileMieru(want []Instance) {
	m.ReconcileWanted(Mieru, "mieru-", string(Mieru), want)
}

// ReconcileTrustTunnel drives every desired TrustTunnel inbound instance.
func (m *Manager) ReconcileTrustTunnel(want []Instance) {
	m.ReconcileWanted(TrustTunnel, "trusttunnel-", string(TrustTunnel), want)
}

// ReconcileWanted Ensures each wanted instance of core and Removes orphan
// keys that match prefix or legacyKey but are not in want.
func (m *Manager) ReconcileWanted(core Name, prefix, legacyKey string, want []Instance) {
	wantKeys := make(map[string]struct{}, len(want))
	for _, inst := range want {
		if inst.Core != core {
			continue
		}
		key := inst.ManageKey()
		wantKeys[key] = struct{}{}
		if err := m.Ensure(inst); err != nil {
			logger.Warningf("tunnel: reconcile %s: %v", key, err)
		}
	}
	m.mu.Lock()
	var orphans []string
	for key := range m.cores {
		if key != legacyKey && !strings.HasPrefix(key, prefix) {
			continue
		}
		if _, ok := wantKeys[key]; !ok {
			orphans = append(orphans, key)
		}
	}
	m.mu.Unlock()
	for _, key := range orphans {
		m.Remove(key)
	}
	// Disk orphan sweep: files left by pre-lucx.122 Removes (process gone,
	// configs still on disk). Only multi-instance prefixes.
	if prefix != "" {
		sweepOrphanConfigFiles(prefix, wantKeys)
	}
}

func probeListening(addr string) bool {
	dialer := &net.Dialer{Timeout: 300 * time.Millisecond}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func probeResponding(addr string) bool {
	tlsDialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: time.Second},
		Config:    &tls.Config{InsecureSkipVerify: true}, // lgtm[go/disabled-certificate-check]
	}
	conn, err := tlsDialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func writeConfigFile(inst Instance) error {
	if strings.TrimSpace(inst.ConfigText) == "" && len(inst.ExtraFiles) == 0 {
		return nil
	}
	if err := os.MkdirAll(workDir(), 0o755); err != nil {
		return fmt.Errorf("tunnel: create config dir: %w", err)
	}
	if strings.TrimSpace(inst.ConfigText) != "" {
		path := configPathFor(inst.ManageKey(), inst.Core)
		if err := writeFileIfChanged(path, inst.ConfigText); err != nil {
			return fmt.Errorf("tunnel: write config: %w", err)
		}
	}
	for name, content := range inst.ExtraFiles {
		if err := writeFileIfChanged(filepath.Join(workDir(), name), content); err != nil {
			return fmt.Errorf("tunnel: write extra config %s: %w", name, err)
		}
	}
	return nil
}

func writeFileIfChanged(path, content string) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func (m *Manager) start(inst Instance, mc *managed) error {
	bin := inst.Core.BinaryPath()
	info, err := os.Stat(bin)
	if err != nil || info.IsDir() {
		return fmt.Errorf("tunnel: %s binary not found at %s", inst.Core.DisplayName(), bin)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(bin, 0o755); err != nil {
			return fmt.Errorf("tunnel: make %s executable: %w", bin, err)
		}
	}
	key := inst.ManageKey()
	if err := os.MkdirAll(dataDirFor(key, inst.Core), 0o700); err != nil {
		return fmt.Errorf("tunnel: create data dir: %w", err)
	}

	cfgPath := configPathFor(key, inst.Core)
	var args []string
	switch {
	case len(inst.Args) > 0:
		args = append(args, inst.Args...)
	case inst.Core == Naive:
		args = []string{"run", "--config", cfgPath, "--adapter", "caddyfile"}
		if extra := strings.TrimSpace(inst.ExtraArgs); extra != "" {
			args = append(args, strings.Fields(extra)...)
		}
	case inst.Core == Olcrtc:
		args = []string{cfgPath}
	case inst.Core == Mieru:
		// mita runs in the foreground ("run") and reads its JSON config from
		// MITA_CONFIG_JSON_FILE. The RPC unix socket is per-instance
		// (MITA_UDS_PATH) so several mita processes never fight over one
		// socket; MITA_INSECURE_UDS skips the chown mita:mita that would
		// fatal out (no mita system user on panel hosts).
		args = []string{"run"}
	case inst.Core == TrustTunnel:
		// trusttunnel_endpoint <vpn.toml> <hosts.toml>; companion files
		// (credentials/rules) are referenced from within vpn.toml.
		args = []string{cfgPath, filepath.Join(workDir(), trustTunnelHostsFileName(key))}
	default:
		return fmt.Errorf("tunnel: no start args for core %q", inst.Core)
	}
	env := append(os.Environ(), "XDG_DATA_HOME="+absPath(dataDirFor(key, inst.Core)))
	if inst.Core == Mieru {
		// Absolute paths: mita gRPC rejects relative MITA_UDS_PATH (authority=bin).
		env = append(env,
			"MITA_CONFIG_JSON_FILE="+absPath(cfgPath),
			"MITA_UDS_PATH="+absPath(filepath.Join(dataDirFor(key, inst.Core), "mita.sock")),
			"MITA_INSECURE_UDS=1",
		)
	}

	if err := mc.proc.Start(bin, args, env); err != nil {
		return fmt.Errorf("tunnel: start %s: %w", key, err)
	}
	return nil
}
