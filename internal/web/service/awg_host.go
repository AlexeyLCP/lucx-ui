// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func fillAwgHostStatus(status *Status) {
	if status == nil {
		return
	}
	hs := awg.CollectHostStatus()
	status.Awg.ModuleLoaded = hs.ModuleLoaded
	status.Awg.ModuleAwg3 = hs.ModuleAwg3
	status.Awg.ModuleAwg31 = hs.ModuleAwg31
	status.Awg.Version = hs.Version
	status.Awg.Interfaces = hs.Interfaces
	status.Awg.Ifnames = hs.Ifnames
	status.Awg.RebuildRunning = AwgRebuildRunning()
	status.Awg.RebootNeeded = awgRebootNeeded()
	switch {
	case !hs.ModuleLoaded:
		status.Awg.State = Error
		// Frontend i18n maps this sentinel to a full install hint (Cores →
		// Rebuild AWG / install-awg-module.sh). Keep the English key stable.
		status.Awg.ErrorMsg = "module_not_loaded"
	case hs.Interfaces > 0:
		status.Awg.State = Running
		status.Awg.ErrorMsg = ""
	default:
		status.Awg.State = Stop
		status.Awg.ErrorMsg = ""
	}
}

var (
	awgRestartMu      sync.Mutex
	awgRestartRunning bool
)

// RestartAwg stops all managed AWG interfaces and immediately reconciles them
// (inbounds + outbounds) — same as the next AwgJob tick, but on demand.
func (s *ServerService) RestartAwg() error {
	awgRestartMu.Lock()
	if awgRestartRunning {
		awgRestartMu.Unlock()
		return fmt.Errorf("AWG restart already in progress")
	}
	awgRestartRunning = true
	awgRestartMu.Unlock()
	defer func() {
		awgRestartMu.Lock()
		awgRestartRunning = false
		awgRestartMu.Unlock()
	}()

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return fmt.Errorf("list inbounds: %w", err)
	}
	var desired []awg.Instance
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := awg.InstanceFromInbound(ib); ok {
			desired = append(desired, inst)
		}
	}

	mgr := awg.GetManager()
	mgr.StopAll()
	mgr.Reconcile(desired)

	// Outbound clients (awgo-N)
	svc := &AwgOutboundService{}
	outbounds, oerr := svc.GetOutbounds()
	if oerr != nil {
		logger.Warning("awg restart: list outbounds:", oerr)
		return nil
	}
	for _, o := range outbounds {
		ifname := "awgo-" + strconv.Itoa(o.Id)
		if !o.Enable {
			_ = mgr.RemoveClient(ifname)
			continue
		}
		if ci, ok := awg.ClientInstanceFromOutbound(o); ok {
			if err := mgr.EnsureClient(ci); err != nil {
				logger.Warning("awg restart: outbound", o.Tag, err)
			}
		}
	}
	return nil
}

var (
	awgRebuildMu      sync.Mutex
	awgRebuildRunning bool
)

// awgRebuildTailBytes is how much of a failed rebuild's output is kept for the
// error log. The interesting part of a DKMS failure is always the end.
const awgRebuildTailBytes = 64 << 10

// boundedTail is an io.Writer that streams whole lines to the panel log and
// retains only the last limit bytes, so a long-running build cannot pin an
// unbounded amount of memory just to produce one error message.
type boundedTail struct {
	mu      sync.Mutex
	limit   int
	tail    []byte
	partial []byte
}

func (b *boundedTail) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tail = append(b.tail, p...)
	if len(b.tail) > b.limit {
		b.tail = b.tail[len(b.tail)-b.limit:]
	}
	rest := p
	for {
		i := bytes.IndexByte(rest, '\n')
		if i < 0 {
			b.partial = append(b.partial, rest...)
			break
		}
		line := append(b.partial, rest[:i]...)
		b.partial = nil
		if trimmed := strings.TrimSpace(string(line)); trimmed != "" {
			logger.Info("awg: rebuild | ", trimmed)
		}
		rest = rest[i+1:]
	}
	return len(p), nil
}

// String returns the retained tail of the output.
func (b *boundedTail) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.tail)
}

// RebuildAwgModule starts bin/install-awg-module.sh --force-rebuild in the
// background (DKMS rebuild from upstream master). Returns immediately; the
// operator watches logs / host status. Linux-only.
func (s *ServerService) RebuildAwgModule() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("AWG module rebuild is only supported on Linux")
	}
	awgRebuildMu.Lock()
	if awgRebuildRunning {
		awgRebuildMu.Unlock()
		return fmt.Errorf("AWG module rebuild already in progress")
	}
	awgRebuildRunning = true
	awgRebuildMu.Unlock()

	script, err := awgModuleScriptPath()
	if err != nil {
		awgRebuildMu.Lock()
		awgRebuildRunning = false
		awgRebuildMu.Unlock()
		return err
	}

	go func() {
		defer func() {
			awgRebuildMu.Lock()
			awgRebuildRunning = false
			awgRebuildMu.Unlock()
		}()
		// LUCX-HOOK: bring every AWG interface down before the script runs so
		// its rmmod can unload the module. A live awgN/awgo-N keeps the module
		// busy, and AwgJob's reconcile tick (skipped while this rebuild is in
		// flight) would otherwise bring them back up mid-build. RestartAwg
		// re-creates them after the script finishes.
		awg.SetRebuildPause(true)
		defer awg.SetRebuildPause(false)
		mgr := awg.GetManager()
		mgr.StopAll()
		mgr.StopAllClients()
		logger.Info("awg: starting module rebuild via ", script)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", script, "--force-rebuild", "--no-kernel-upgrade")
		cmd.Dir = filepath.Dir(script)
		cmd.Env = awgModuleScriptEnv()
		tail := &boundedTail{limit: awgRebuildTailBytes}
		cmd.Stdout = tail
		cmd.Stderr = tail
		if err := cmd.Run(); err != nil {
			logger.Warningf("awg: module rebuild failed: %v\n%s", err, tail.String())
			return
		}
		logger.Info("awg: module rebuild finished OK")
		awg.SetRebuildPause(false)
		time.Sleep(2 * time.Second)
		if err := s.RestartAwg(); err != nil {
			logger.Warning("awg: post-rebuild restart interfaces:", err)
		}
	}()
	return nil
}

func awgModuleScriptPath() (string, error) {
	script := filepath.Join(config.GetBinFolderPath(), "install-awg-module.sh")
	if abs, err := filepath.Abs(script); err == nil {
		script = abs
	}
	if _, err := os.Stat(script); err != nil {
		return "", fmt.Errorf("install-awg-module.sh not found at %s", script)
	}
	return script, nil
}

func awgModuleScriptEnv() []string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	} else {
		path = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:" + path
	}
	return append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
		"NEEDRESTART_MODE=a",
		"NEEDRESTART_SUSPEND=1",
		"PATH="+path,
	)
}

func (s *ServerService) UninstallAwgModule() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("AWG module uninstall is only supported on Linux")
	}
	awgRebuildMu.Lock()
	if awgRebuildRunning {
		awgRebuildMu.Unlock()
		return fmt.Errorf("AWG module rebuild already in progress")
	}
	awgRebuildMu.Unlock()

	script, err := awgModuleScriptPath()
	if err != nil {
		return err
	}
	logger.Info("awg: uninstalling module via ", script)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", script, "--uninstall")
	cmd.Dir = filepath.Dir(script)
	cmd.Env = awgModuleScriptEnv()
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				logger.Info("awg: uninstall | ", trimmed)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("uninstall AWG module: %w", err)
	}
	logger.Info("awg: module uninstall finished OK")
	return nil
}

// AwgRebuildRunning reports whether a background module rebuild is in flight.
func AwgRebuildRunning() bool {
	awgRebuildMu.Lock()
	defer awgRebuildMu.Unlock()
	return awgRebuildRunning
}

// awgRebootNeeded reports whether install-awg-module.sh / update.sh left a
// reboot flag (the module was built for a newer kernel, or rmmod stayed busy
// during a hot swap). The Cores card surfaces it as a hint.
func awgRebootNeeded() bool {
	_, err := os.Stat("/etc/x-ui/.awg-reboot-needed")
	return err == nil
}
