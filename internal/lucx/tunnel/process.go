// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"bytes"
	"context"
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

const (
	gracefulStopTimeout = 5 * time.Second
	forceStopTimeout    = 2 * time.Second
	maxLogLines         = 500
	// maxPartialLine caps the unterminated tail procLogWriter holds while it
	// waits for a newline. A core that writes a progress bar with \r only, a
	// single-line JSON dump, or binary garbage after a bad download would
	// otherwise grow this string without bound — the ring buffer above only
	// caps whole lines. At the cap the tail is flushed as its own line.
	maxPartialLine = 64 << 10
)

// ring is a bounded, concurrency-safe line buffer for the child's output.
type ring struct {
	mu   sync.Mutex
	rows []string
}

func (r *ring) push(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.rows) >= maxLogLines {
		copy(r.rows, r.rows[len(r.rows)-maxLogLines+1:])
		r.rows = r.rows[:maxLogLines-1]
	}
	r.rows = append(r.rows, line)
}

func (r *ring) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.rows) == 0 {
		return ""
	}
	return r.rows[len(r.rows)-1]
}

func (r *ring) all(max int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if max <= 0 || len(r.rows) <= max {
		out := make([]string, len(r.rows))
		copy(out, r.rows)
		return out
	}
	out := make([]string, max)
	copy(out, r.rows[len(r.rows)-max:])
	return out
}

// procLogWriter funnels the child's stdout/stderr into the ring buffer and
// forwards each line to the panel log (prefix "tunnel: <label> | <line>",
// matching the mtproto/AWG sidecar convention) so startup failures are
// visible in journald and the panel log viewer.
type procLogWriter struct {
	mu    sync.Mutex
	label string
	buf   strings.Builder
	ring  *ring
}

func (w *procLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	consumed := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			w.buf.Write(p)
			break
		}
		w.buf.Write(p[:i])
		w.emitLocked(w.takeBufLocked())
		p = p[i+1:]
	}
	if w.buf.Len() >= maxPartialLine {
		w.emitLocked(w.takeBufLocked())
	}
	return consumed, nil
}

// takeBufLocked returns the buffered partial line and resets the builder.
func (w *procLogWriter) takeBufLocked() string {
	line := w.buf.String()
	w.buf.Reset()
	return line
}

// flush emits any buffered partial line; called once the process exits so a
// final un-terminated error line is not lost.
func (w *procLogWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		w.emitLocked(w.takeBufLocked())
	}
}

func (w *procLogWriter) emitLocked(line string) {
	trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trimmed == "" {
		return
	}
	w.ring.push(trimmed)
	logger.Infof("tunnel: %s | %s", w.label, trimmed)
}

// Proc supervises one tunnel-core process.
type Proc struct {
	label string

	mu    sync.RWMutex
	cmd   *exec.Cmd
	done  chan struct{}
	log   *procLogWriter
	lines *ring

	exitErr         error
	intentionalStop atomic.Bool
}

// NewProc returns an idle supervised process for the given log label.
func NewProc(label string) *Proc {
	lines := &ring{}
	return &Proc{
		label: label,
		log:   &procLogWriter{label: label, ring: lines},
		lines: lines,
	}
}

// IsRunning reports whether the process is currently alive.
func (p *Proc) IsRunning() bool {
	p.mu.RLock()
	cmd, done := p.cmd, p.done
	p.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if done != nil {
		select {
		case <-done:
			return false
		default:
		}
	}
	return true
}

func (p *Proc) Pid() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// LastLine returns the most recent captured log line.
func (p *Proc) LastLine() string { return p.lines.last() }

// Lines returns up to max recent captured log lines.
func (p *Proc) Lines(max int) []string { return p.lines.all(max) }

// ExitError returns the last unexpected exit error, if any.
func (p *Proc) ExitError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitErr
}

// Start launches the binary with the given arguments and environment.
func (p *Proc) Start(bin string, args []string, env []string) error {
	if p.IsRunning() {
		return errors.New("tunnel: process is already running")
	}
	cmd := exec.CommandContext(context.Background(), bin, args...)
	cmd.Stdout = p.log
	cmd.Stderr = p.log
	if len(env) > 0 {
		cmd.Env = env
	}
	done := make(chan struct{})
	p.mu.Lock()
	p.cmd = cmd
	p.done = done
	p.exitErr = nil
	p.mu.Unlock()
	p.intentionalStop.Store(false)
	prepareCmd(cmd)
	if err := cmd.Start(); err != nil {
		close(done)
		p.mu.Lock()
		p.cmd = nil
		p.mu.Unlock()
		return err
	}
	attachChildLifetime(cmd)
	logger.Infof("tunnel: %s started (pid %d)", p.label, cmd.Process.Pid)
	go p.wait(cmd, done)
	return nil
}

func (p *Proc) wait(cmd *exec.Cmd, done chan struct{}) {
	defer close(done)
	err := cmd.Wait()
	p.log.flush()
	if err == nil || p.intentionalStop.Load() {
		return
	}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(err.Error()), "exit status 1") {
		p.setExitErr(err)
		return
	}
	logger.Errorf("tunnel: %s process exited: %v", p.label, err)
	p.setExitErr(err)
}

func (p *Proc) setExitErr(err error) {
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
}

// Stop terminates the process gracefully, falling back to a kill.
func (p *Proc) Stop() error {
	if !p.IsRunning() {
		return errors.New("tunnel: process is not running")
	}
	p.intentionalStop.Store(true)
	p.mu.RLock()
	cmd, done := p.cmd, p.done
	p.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return errors.New("tunnel: process is not running")
	}

	if runtime.GOOS == "windows" {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return waitForExit(done, forceStopTimeout)
	}

	pid := cmd.Process.Pid
	if err := signalGroup(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return waitForExit(done, forceStopTimeout)
		}
		return err
	}
	if err := waitForExit(done, gracefulStopTimeout); err == nil {
		logger.Infof("tunnel: %s stopped", p.label)
		return nil
	}

	logger.Warningf("tunnel: %s did not stop after SIGTERM, killing process group", p.label)
	if err := signalGroup(pid, syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return waitForExit(done, forceStopTimeout)
}

func waitForExit(done <-chan struct{}, timeout time.Duration) error {
	if done == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("tunnel: timed out waiting for process to stop after %s", timeout)
	}
}
