// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

// RemoveImportCandidate stops a discovered foreign install and takes its
// config out of the scan path. Panel-managed configs and the default-route
// interface are refused. AWG .conf files are moved to x-ui-backup, not shredded.
func RemoveImportCandidate(c ImportCandidate) error {
	if c.Source == ImportSourceTproxy {
		return tunnel.RemoveTproxyInstall(c.ConfPath)
	}
	if c.ConfPath != "" && filepath.IsAbs(c.ConfPath) && ConfigPathIsManaged(c.ConfPath) {
		return fmt.Errorf("refuse to delete panel-managed config %s", c.ConfPath)
	}
	if c.Ifname != "" && c.Ifname == defaultRouteInterface() {
		return fmt.Errorf("refuse to delete default-route interface %s", c.Ifname)
	}
	if err := StopImportSource(c); err != nil {
		return err
	}
	if c.Backend == "kernel" && filepath.IsAbs(c.ConfPath) && isImportIfname(c.Ifname) {
		_, _ = awgQuick("down", c.ConfPath)
	}
	if c.ConfPath != "" && filepath.IsAbs(c.ConfPath) {
		if err := BackupForeignConf(c.ConfPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, p := range c.ExtraPaths {
		if filepath.IsAbs(p) {
			_ = os.Remove(p)
		}
	}
	return nil
}
