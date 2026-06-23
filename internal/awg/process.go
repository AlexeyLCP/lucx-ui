// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// configPathForID returns the .conf path for an inbound. Mirrors the mtproto
// sidecar's configPathForID but under the AWG tools' conventional path.
func configPathForID(id int) string {
	return fmt.Sprintf("%s/awg%d.conf", awgConfigDir, id)
}

// procLogWriter consumes awg-quick child output and forwards lines to the
// x-ui log so operator-visible messages reach the panel log viewer.
type procLogWriter struct {
	mu       sync.Mutex
	label    string
	buf      string
	lastLine string
}

func (w *procLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf += string(p)
	for {
		i := strings.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		w.emitLocked(line)
	}
	return len(p), nil
}

func (w *procLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf != "" {
		line := w.buf
		w.buf = ""
		w.emitLocked(line)
	}
}

func (w *procLogWriter) emitLocked(line string) {
	trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trimmed == "" {
		return
	}
	w.lastLine = trimmed
	logger.Infof("awg: %s | %s", w.label, trimmed)
}

func (w *procLogWriter) LastLine() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastLine
}

// Process wraps a single awg-quick invocation for one AWG inbound. Unlike mtg,
// awg-quick is not a long-lived daemon: it configures the kernel interface and
// exits. We track the interface's liveness via /sys/class/net rather than a
// PID. Routing of the decrypted traffic into Xray is handled by an injected
// TUN inbound in the generated Xray config (injectAwgEgress), so no tun2socks
// daemon or SOCKS bridge is needed — symmetric with the mtproto sidecar.
type Process struct {
	ifname     string
	configPath string
	logWriter  *procLogWriter
}

func newProcess(ifname, configPath, label string) *Process {
	return &Process{
		ifname:     ifname,
		configPath: configPath,
		logWriter:  &procLogWriter{label: label},
	}
}

// IsRunning reports whether the AWG interface is up. awg-quick exits after
// setup, so we check /sys/class/net rather than a PID.
func (p *Process) IsRunning() bool {
	if p.ifname == "" {
		return false
	}
	_, err := os.Stat("/sys/class/net/" + p.ifname)
	return err == nil
}

// GetResult returns the last log line (for diagnostics).
func (p *Process) GetResult() string {
	return p.logWriter.LastLine()
}

// Start brings the AWG interface up via awg-quick.
func (p *Process) Start() error {
	if p.IsRunning() {
		return errors.New("awg interface already up: " + p.ifname)
	}
	if err := os.MkdirAll(awgConfigDir, 0o750); err != nil {
		return err
	}
	out, err := awgQuick("up", p.configPath)
	if err != nil {
		p.logWriter.Write(out)
		return fmt.Errorf("awg-quick up %s: %w\n%s", p.configPath, err, string(out))
	}
	if len(out) > 0 {
		p.logWriter.Write(out)
	}
	logger.Infof("awg: interface %s brought up", p.ifname)
	return nil
}

// Stop tears the AWG interface down.
func (p *Process) Stop() error {
	if !p.IsRunning() {
		return nil
	}
	out, err := awgQuick("down", p.configPath)
	if err != nil {
		// Interface may already be gone; treat as best-effort.
		logger.Warningf("awg: awg-quick down %s: %v\n%s", p.ifname, err, string(out))
	}
	logger.Infof("awg: interface %s brought down", p.ifname)
	return nil
}