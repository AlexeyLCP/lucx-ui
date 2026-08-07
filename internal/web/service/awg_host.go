// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func fillAwgHostStatus(status *Status) {
	if status == nil {
		return
	}
	hs := awg.CollectHostStatus()
	status.Awg.ModuleLoaded = hs.ModuleLoaded
	status.Awg.ModuleAwg3 = hs.ModuleAwg3
	status.Awg.Version = hs.Version
	status.Awg.Interfaces = hs.Interfaces
	switch {
	case !hs.ModuleLoaded:
		status.Awg.State = Error
		status.Awg.ErrorMsg = "amneziawg kernel module not loaded"
	case hs.Interfaces > 0:
		status.Awg.State = Running
		status.Awg.ErrorMsg = ""
	default:
		status.Awg.State = Stop
		status.Awg.ErrorMsg = ""
	}
}

var (
	awgUpdateMu      sync.Mutex
	awgUpdateRunning bool
)

// UpdateAwgModule runs bin/install-awg-module.sh --force-rebuild. Blocks until
// the script exits (can take several minutes). Concurrent calls are rejected.
func (s *ServerService) UpdateAwgModule() error {
	awgUpdateMu.Lock()
	if awgUpdateRunning {
		awgUpdateMu.Unlock()
		return fmt.Errorf("AWG module update already in progress")
	}
	awgUpdateRunning = true
	awgUpdateMu.Unlock()
	defer func() {
		awgUpdateMu.Lock()
		awgUpdateRunning = false
		awgUpdateMu.Unlock()
	}()

	script, err := resolveAwgInstallScript()
	if err != nil {
		return err
	}
	// Best-effort: drop ifaces so rmmod can succeed during rebuild.
	awg.GetManager().StopAll()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", script, "--force-rebuild")
	cmd.Dir = filepath.Dir(script)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		logger.Infof("awg module update:\n%s", string(out))
	}
	if err != nil {
		return fmt.Errorf("install-awg-module.sh failed: %w", err)
	}
	return nil
}

func resolveAwgInstallScript() (string, error) {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "bin", "install-awg-module.sh"),
			filepath.Join(dir, "install-awg-module.sh"),
		)
	}
	candidates = append(candidates,
		"/usr/local/x-ui/bin/install-awg-module.sh",
		"bin/install-awg-module.sh",
	)
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("install-awg-module.sh not found")
}
