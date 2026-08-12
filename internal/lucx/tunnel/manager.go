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
	"net"
	"os"
	"runtime"
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
	sum := sha256.Sum256([]byte(
		inst.ConfigText + "\x00" +
			inst.ExtraArgs + "\x00" +
			strings.Join(inst.Args, "\x00") + "\x00" +
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

type managed struct {
	proc *Proc
	fp   string
}

// Manager owns the supervised tunnel-core processes keyed by ManageKey
// (legacy core name or "naive-{id}" for inbound instances).
type Manager struct {
	mu           sync.Mutex
	cores        map[string]*managed
	naiveTraffic map[string]*naiveLogCursor
}

func newManager() *Manager {
	return &Manager{cores: make(map[string]*managed)}
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
	}
	return managerRef
}

func (m *Manager) slot(key string) *managed {
	mc, ok := m.cores[key]
	if !ok {
		mc = &managed{}
		m.cores[key] = mc
	}
	return mc
}

// Ensure converges one core toward the desired instance: disabled -> stopped
// (config preserved); enabled with a changed fingerprint -> restarted;
// enabled but dead (crash or panel restart) -> started. It is the single
// entry point used by the reconcile job, the save flow and manual start.
func (m *Manager) Ensure(inst Instance) error {
	if !inst.Core.Valid() {
		return fmt.Errorf("tunnel: unknown core %q", inst.Core)
	}
	key := inst.ManageKey()
	m.mu.Lock()
	defer m.mu.Unlock()
	mc := m.slot(key)

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
	if err := m.start(inst); err != nil {
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
	m.mu.Lock()
	running := false
	if mc, ok := m.cores[inst.ManageKey()]; ok && mc.proc != nil && mc.proc.IsRunning() {
		running = true
	}
	m.mu.Unlock()
	if running {
		ensureQwdttXrayRouting(inst)
	}
}

// Remove stops and forgets a managed key (inbound delete). Config files are
// left on disk for operator recovery; next Ensure of the same key reuses them.
func (m *Manager) Remove(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.cores[key]
	if !ok {
		return
	}
	if mc.proc != nil && mc.proc.IsRunning() {
		if err := mc.proc.Stop(); err != nil {
			logger.Warningf("tunnel: remove stop %s: %v", key, err)
		}
	}
	delete(m.cores, key)
}

// Stop terminates the core process if it is running (legacy Name API).
func (m *Manager) Stop(core Name) error {
	return m.StopKey(string(core))
}

// StopKey terminates one managed key if running.
func (m *Manager) StopKey(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc := m.slot(key)
	mc.fp = ""
	if mc.proc == nil || !mc.proc.IsRunning() {
		return nil
	}
	return mc.proc.Stop()
}

// StopAll terminates every running core; wired into the panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, mc := range m.cores {
		if mc.proc != nil && mc.proc.IsRunning() {
			if err := mc.proc.Stop(); err != nil {
				logger.Warningf("tunnel: shutdown stop %s failed: %v", key, err)
			}
		}
		mc.fp = ""
	}
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

// Logs returns up to max recent output lines of the core process.
func (m *Manager) Logs(core Name, max int) []string {
	return m.LogsKey(string(core), max)
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
	if strings.TrimSpace(inst.ConfigText) == "" {
		return nil
	}
	if err := os.MkdirAll(workDir(), 0o755); err != nil {
		return fmt.Errorf("tunnel: create config dir: %w", err)
	}
	path := configPathFor(inst.ManageKey(), inst.Core)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == inst.ConfigText {
		return nil
	}
	if err := os.WriteFile(path, []byte(inst.ConfigText), 0o600); err != nil {
		return fmt.Errorf("tunnel: write config: %w", err)
	}
	return nil
}

func (m *Manager) start(inst Instance) error {
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
	default:
		return fmt.Errorf("tunnel: no start args for core %q", inst.Core)
	}
	env := append(os.Environ(), "XDG_DATA_HOME="+dataDirFor(key, inst.Core))

	mc := m.slot(key)
	if mc.proc == nil {
		mc.proc = NewProc(key)
	}
	if err := mc.proc.Start(bin, args, env); err != nil {
		return fmt.Errorf("tunnel: start %s: %w", key, err)
	}
	return nil
}
