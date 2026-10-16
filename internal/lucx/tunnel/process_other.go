// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build !windows

package tunnel

import (
	"os/exec"
	"syscall"
)

func prepareCmd(cmd *exec.Cmd, uid, gid int) {
	if cmd == nil {
		return
	}
	attr := &syscall.SysProcAttr{Setpgid: true}
	if uid > 0 {
		attr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), NoSetGroups: true}
	}
	cmd.SysProcAttr = attr
}

func signalGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}

func attachChildLifetime(_ *exec.Cmd) {}
