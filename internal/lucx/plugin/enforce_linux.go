// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package plugin

import (
	"os/exec"
	"syscall"
)

const enforcementSupported = true

// applySysProcAttr prepares the plugin child process per the fail-closed
// Stage 2 contract: permissions.network ["none"] puts the child into its own
// network namespace; resources.maxMemoryMB becomes RLIMIT_AS on the child only.
func applySysProcAttr(cmd *exec.Cmd, m Manifest) error {
	if !needsEnforcement(m) {
		return nil
	}
	attr := &syscall.SysProcAttr{}
	if denyNetwork(m) {
		attr.Cloneflags |= syscall.CLONE_NEWNET
	}
	if mb := memLimit(m); mb > 0 {
		limit := uint64(mb) << 20
		attr.Rlimit = append(attr.Rlimit, syscall.Rlimit{Type: syscall.RLIMIT_AS, Cur: limit, Max: limit})
	}
	cmd.SysProcAttr = attr
	return nil
}
