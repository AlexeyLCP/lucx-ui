// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build !linux

package plugin

import (
	"errors"
	"os/exec"
)

const enforcementSupported = false

// prepareCmd refuses enforcement on unsupported platforms: a manifest that
// asks for isolation or limits must not start silently unsandboxed.
func prepareCmd(_ *exec.Cmd, m Manifest) error {
	if needsEnforcement(m) {
		if cpuRequiresUnit(m) {
			return errors.New("plugin CPU capping requires the systemd unit runner, which needs Linux with systemd")
		}
		return errors.New("plugin network isolation and resource limits require Linux")
	}
	return nil
}
