// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package plugin

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
)

const enforcementSupported = true

// prepareCmd applies the fail-closed Stage 2 contract to the plugin child:
// permissions.network ["none"] puts it into its own network namespace, and
// resources.maxMemoryMB becomes RLIMIT_AS via a /bin/sh ulimit wrapper that
// execs the entrypoint (env, dir, stdio and signal delivery are preserved;
// RLIMIT_AS caps virtual memory, so Go plugins that reserve big arenas may
// not survive their own limit — only declare it if the process fits).
func prepareCmd(cmd *exec.Cmd, m Manifest) error {
	if !needsEnforcement(m) {
		return nil
	}
	if cpuRequiresUnit(m) {
		return errors.New("plugin CPU capping requires the systemd unit runner")
	}
	if mb := memLimit(m); mb > 0 {
		// Go executes cmd.Path and passes cmd.Args as argv only, so the
		// ulimit wrapper must replace Path, not prepend to Args.
		wrapper := "ulimit -v " + strconv.Itoa(mb*1024) + " && exec \"$0\""
		cmd.Args = []string{"/bin/sh", "-c", wrapper, cmd.Path}
		cmd.Path = "/bin/sh"
	}
	if denyNetwork(m) {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}
	return nil
}
