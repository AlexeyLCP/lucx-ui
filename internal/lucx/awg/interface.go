// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SetupAWGInterface creates and brings up an AWG network interface.
// Idempotent — safe to call if the interface already exists.
func SetupAWGInterface(name string) error {
	if interfaceExists(name) {
		// Already exists — just ensure it's up
		exec.Command("ip", "link", "set", name, "up").Run()
		return nil
	}
	// Create AWG interface
	if out, err := exec.Command("ip", "link", "add", name, "type", "amneziawg").CombinedOutput(); err != nil {
		return fmt.Errorf("create %s: %w\n%s", name, err, string(out))
	}
	return nil
}

// DeleteAWGInterface removes an AWG network interface.
// Idempotent — safe to call if the interface doesn't exist.
func DeleteAWGInterface(name string) {
	exec.Command("ip", "link", "del", name).Run()
}

// interfaceExists checks if a network interface exists.
func interfaceExists(name string) bool {
	_, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", name))
	return err == nil
}

// interfaceUp checks if a network interface is UP.
func interfaceUp(name string) bool {
	out, err := exec.Command("ip", "link", "show", name).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "UP")
}

// EnableForwarding writes 1 to /proc/sys/net/ipv4/ip_forward.
func EnableForwarding() {
	os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}
