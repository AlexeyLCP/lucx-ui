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
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// configPathForID returns the .conf path for an inbound. Mirrors the mtproto
// sidecar's configPathForID but under the AWG tools' conventional path.
func configPathForID(id int) string {
	return fmt.Sprintf("%s/awg%d.conf", awgConfigDir, id)
}

var (
	gracefulStopTimeout = 5 * time.Second
	forceStopTimeout    = 2 * time.Second
)

// procLogWriter consumes awg-quick / tun2socks child output and forwards lines
// to the x-ui log so operator-visible messages reach the panel log viewer.
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
// exits. We track the interface's liveness instead of a process PID. An
// optional tun2socks child process (for TUN→SOCKS routing) is a real daemon
// and gets full process supervision.
type Process struct {
	ifname     string
	configPath string
	tun2socks  *exec.Cmd
	tunDone    chan struct{}
	logWriter  *procLogWriter
	intentionalStop atomic.Bool
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

// Start brings the AWG interface up via awg-quick and, when configured, starts
// the tun2socks daemon for TUN→SOCKS routing.
func (p *Process) Start() error {
	if p.IsRunning() {
		return errors.New("awg interface already up: " + p.ifname)
	}
	if err := os.MkdirAll(awgConfigDir, 0o750); err != nil {
		return err
	}
	cmd := exec.Command("awg-quick", "up", p.configPath)
	out, err := cmd.CombinedOutput()
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

// Stop tears the AWG interface down and stops the tun2socks daemon if any.
func (p *Process) Stop() error {
	p.intentionalStop.Store(true)
	// Stop tun2socks first so it releases the TUN device before awg-quick
	// deletes the interface.
	p.stopTun2socks()
	if !p.IsRunning() {
		return nil
	}
	cmd := exec.Command("awg-quick", "down", p.configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Interface may already be gone; treat as best-effort.
		logger.Warningf("awg: awg-quick down %s: %v\n%s", p.ifname, err, string(out))
	}
	logger.Infof("awg: interface %s brought down", p.ifname)
	return nil
}

// startTun2socks launches the tun2socks daemon for this AWG's TUN→SOCKS
// routing. tunDev is the TUN device name (e.g. "tun1"), socksPort the hidden
// SOCKS5 port to dial through.
func (p *Process) startTun2socks(tunDev string, socksPort int) {
	if tunDev == "" || socksPort == 0 {
		return
	}
	// Already running for this TUN device?
	if exec.Command("pgrep", "-f", "tun2socks.*-device "+tunDev).Run() == nil {
		return
	}
	proxy := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	p.tun2socks = exec.Command("tun2socks",
		"-device", tunDev,
		"-proxy", proxy,
		"-loglevel", "silent")
	p.tun2socks.Stdout = p.logWriter
	p.tun2socks.Stderr = p.logWriter
	p.tunDone = make(chan struct{})
	if err := p.tun2socks.Start(); err != nil {
		logger.Warningf("awg: tun2socks start failed dev=%s port=%d: %v", tunDev, socksPort, err)
		p.tun2socks = nil
		return
	}
	go p.waitTun2socks()
	logger.Infof("awg: tun2socks started pid=%d dev=%s port=%d", p.tun2socks.Process.Pid, tunDev, socksPort)
}

func (p *Process) waitTun2socks() {
	defer close(p.tunDone)
	if p.tun2socks == nil || p.tun2socks.Process == nil {
		return
	}
	err := p.tun2socks.Wait()
	p.logWriter.Flush()
	if err == nil || p.intentionalStop.Load() {
		return
	}
	logger.Errorf("awg: tun2socks exited: %v", err)
}

func (p *Process) stopTun2socks() {
	if p.tun2socks == nil || p.tun2socks.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = p.tun2socks.Process.Kill()
	} else {
		_ = p.tun2socks.Process.Signal(syscall.SIGTERM)
	}
	p.waitForTunExit(forceStopTimeout)
}

func (p *Process) waitForTunExit(timeout time.Duration) {
	if p.tunDone == nil {
		return
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-p.tunDone:
	case <-t.C:
		if p.tun2socks != nil && p.tun2socks.Process != nil {
			_ = p.tun2socks.Process.Kill()
		}
	}
}