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
type Instance struct {
	Core    Name
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
}

// Fingerprint identifies the instance's rendered config plus CLI shape. Any
// change to either forces a restart on the next Ensure.
func (inst Instance) Fingerprint() string {
	sum := sha256.Sum256([]byte(
		inst.ConfigText + "\x00" +
			inst.ExtraArgs + "\x00" +
			strings.Join(inst.Args, "\x00"),
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

// Manager owns the supervised tunnel-core processes keyed by core name.
type Manager struct {
	mu    sync.Mutex
	cores map[Name]*managed
}

func newManager() *Manager {
	return &Manager{cores: make(map[Name]*managed)}
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

func (m *Manager) core(n Name) *managed {
	mc, ok := m.cores[n]
	if !ok {
		mc = &managed{}
		m.cores[n] = mc
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
	m.mu.Lock()
	defer m.mu.Unlock()
	mc := m.core(inst.Core)

	if !inst.Enabled {
		if mc.proc != nil && mc.proc.IsRunning() {
			if err := mc.proc.Stop(); err != nil {
				return fmt.Errorf("tunnel: stop %s: %w", inst.Core.DisplayName(), err)
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
			logger.Warningf("tunnel: stop before restart of %s failed: %v", inst.Core.DisplayName(), err)
		}
		// A graceful stop is given time above; a short pause keeps the port
		// free before the new process binds it (elector1337 restart pause).
		time.Sleep(200 * time.Millisecond)
	}
	if err := m.start(inst); err != nil {
		return err
	}
	mc.fp = fp
	return nil
}

// Stop terminates the core process if it is running.
func (m *Manager) Stop(core Name) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc := m.core(core)
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
	for _, mc := range m.cores {
		if mc.proc != nil && mc.proc.IsRunning() {
			if err := mc.proc.Stop(); err != nil {
				logger.Warningf("tunnel: shutdown stop failed: %v", err)
			}
		}
		mc.fp = ""
	}
}

// IsRunning reports whether the core process is alive.
func (m *Manager) IsRunning(core Name) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.cores[core]
	return ok && mc.proc != nil && mc.proc.IsRunning()
}

// Logs returns up to max recent output lines of the core process.
func (m *Manager) Logs(core Name, max int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.cores[core]
	if !ok || mc.proc == nil {
		return nil
	}
	return mc.proc.Lines(max)
}

// LastLog returns the most recent output line of the core process.
func (m *Manager) LastLog(core Name) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.cores[core]
	if !ok || mc.proc == nil {
		return ""
	}
	return mc.proc.LastLine()
}

// StatusOf probes the core: process liveness first, then (when running and a
// probe port is known) the TCP listener and the TLS handshake.
func (m *Manager) StatusOf(inst Instance) Status {
	st := Status{Running: m.IsRunning(inst.Core)}
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

// probeListening reports whether a TCP listener answers on addr.
func probeListening(addr string) bool {
	dialer := &net.Dialer{Timeout: 300 * time.Millisecond}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// probeResponding reports whether a TLS handshake completes against addr
// (certificate is not verified — the probe only checks liveness).
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

// writeConfigFile renders the instance config to disk, skipping the write
// when the content is unchanged (avoids mtime churn on every reconcile).
// CLI-only cores (empty ConfigText) are a no-op.
func writeConfigFile(inst Instance) error {
	if strings.TrimSpace(inst.ConfigText) == "" {
		return nil
	}
	if err := os.MkdirAll(workDir(), 0o755); err != nil {
		return fmt.Errorf("tunnel: create config dir: %w", err)
	}
	path := configPath(inst.Core)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == inst.ConfigText {
		return nil
	}
	if err := os.WriteFile(path, []byte(inst.ConfigText), 0o600); err != nil {
		return fmt.Errorf("tunnel: write config: %w", err)
	}
	return nil
}

// start launches the core binary against its rendered config file.
func (m *Manager) start(inst Instance) error {
	bin := inst.Core.BinaryPath()
	info, err := os.Stat(bin)
	if err != nil || info.IsDir() {
		return fmt.Errorf("tunnel: %s binary not found at %s", inst.Core.DisplayName(), bin)
	}
	if runtime.GOOS != "windows" {
		// Copies delivered by scp/zip may lack the exec bit; fix it so the
		// delivery order never matters.
		if err := os.Chmod(bin, 0o755); err != nil {
			return fmt.Errorf("tunnel: make %s executable: %w", bin, err)
		}
	}
	// 0700 for qWDTT state (passwords.json / keys); harmless for the others.
	if err := os.MkdirAll(dataDir(inst.Core), 0o700); err != nil {
		return fmt.Errorf("tunnel: create data dir: %w", err)
	}

	var args []string
	switch {
	case len(inst.Args) > 0:
		// CLI-driven cores (qWDTT) supply the full argv from the web layer.
		args = append(args, inst.Args...)
	case inst.Core == Naive:
		// caddy refuses to run without an explicit adapter for non-JSON
		// configs; the panel always renders a Caddyfile.
		args = []string{"run", "--config", configPath(inst.Core), "--adapter", "caddyfile"}
		if extra := strings.TrimSpace(inst.ExtraArgs); extra != "" {
			args = append(args, strings.Fields(extra)...)
		}
	case inst.Core == Olcrtc:
		// olcrtc accepts exactly one argument: the YAML config path
		// (upstream has no CLI flags; extra args would fail startup).
		args = []string{configPath(inst.Core)}
	default:
		return fmt.Errorf("tunnel: no start args for core %q", inst.Core)
	}
	env := append(os.Environ(), "XDG_DATA_HOME="+dataDir(inst.Core))

	mc := m.core(inst.Core)
	if mc.proc == nil {
		mc.proc = NewProc(string(inst.Core))
	}
	if err := mc.proc.Start(bin, args, env); err != nil {
		return fmt.Errorf("tunnel: start %s: %w", inst.Core.DisplayName(), err)
	}
	return nil
}
