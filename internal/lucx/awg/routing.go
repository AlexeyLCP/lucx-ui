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

// RoutingConfig holds all parameters for TUN routing setup.
type RoutingConfig struct {
	AWGInterface string
	TUNInterface string
	AWGServerIP  string
	AWGSubnet    string
	RouteTable   string
	RoutePref    int
	MTU          int
}

// SetupTUNRouting creates AWG interface and sets up all routing for the TUN child.
// Idempotent — safe to call multiple times.
func SetupTUNRouting(cfg RoutingConfig) error {
	// 1. Create AWG interface (ignore if exists)
	runCmd("ip", "link", "add", cfg.AWGInterface, "type", "amneziawg")

	// 2. Assign IP and bring up
	runCmd("ip", "addr", "add", cfg.AWGServerIP+"/24", "dev", cfg.AWGInterface)
	runCmd("ip", "link", "set", cfg.AWGInterface, "mtu", fmt.Sprintf("%d", cfg.MTU))
	runCmd("ip", "link", "set", cfg.AWGInterface, "up")

	// 3. Enable kernel forwarding
	os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)

	// 4. TUN interface always up
	runCmd("ip", "link", "set", cfg.TUNInterface, "mtu", fmt.Sprintf("%d", cfg.MTU))
	runCmd("ip", "link", "set", cfg.TUNInterface, "up")

	// 5. Policy routing
	runCmd("ip", "rule", "add", "from", cfg.AWGSubnet, "table", cfg.RouteTable, "pref", fmt.Sprintf("%d", cfg.RoutePref))
	runCmd("ip", "route", "add", "default", "dev", cfg.TUNInterface, "table", cfg.RouteTable)

	// 6. Firewall (ensure rules exist)
	ensureIptables("FORWARD", "-i", cfg.AWGInterface, "-o", cfg.TUNInterface, "-j", "ACCEPT")
	ensureIptables("FORWARD", "-i", cfg.TUNInterface, "-o", cfg.AWGInterface, "-j", "ACCEPT")
	ensureIptablesNat("POSTROUTING", "-s", cfg.AWGSubnet, "-o", cfg.TUNInterface, "-j", "MASQUERADE")

	return nil
}

// CleanupTUNRouting removes all routing and firewall rules for an AWG interface.
// Idempotent — safe to call even if partially cleaned.
func CleanupTUNRouting(cfg RoutingConfig) {
	runCmd("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", cfg.AWGSubnet, "-o", cfg.TUNInterface, "-j", "MASQUERADE")
	runCmd("iptables", "-D", "FORWARD", "-i", cfg.AWGInterface, "-o", cfg.TUNInterface, "-j", "ACCEPT")
	runCmd("iptables", "-D", "FORWARD", "-i", cfg.TUNInterface, "-o", cfg.AWGInterface, "-j", "ACCEPT")

	runCmd("ip", "route", "del", "default", "dev", cfg.TUNInterface, "table", cfg.RouteTable)
	runCmd("ip", "rule", "del", "from", cfg.AWGSubnet, "table", cfg.RouteTable, "pref", fmt.Sprintf("%d", cfg.RoutePref))

	runCmd("ip", "link", "del", cfg.AWGInterface)
}

// EnsureTUNRouting checks if routing is correctly set up and repairs if needed.
func EnsureTUNRouting(cfg RoutingConfig) (repaired bool) {
	if !interfaceExists(cfg.AWGInterface) {
		_ = SetupTUNRouting(cfg)
		return true
	}
	if !interfaceUp(cfg.TUNInterface) {
		runCmd("ip", "link", "set", cfg.TUNInterface, "up")
		repaired = true
	}
	b, _ := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if string(b) != "1\n" && string(b) != "1" {
		os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
		repaired = true
	}
	return repaired
}

func runCmd(args ...string) {
	exec.Command(args[0], args[1:]...).Run()
}

func interfaceExists(name string) bool {
	_, err := os.Stat(fmt.Sprintf("/sys/class/net/%s", name))
	return err == nil
}

func interfaceUp(name string) bool {
	out, err := exec.Command("ip", "link", "show", name).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "UP")
}

func ensureIptables(args ...string) {
	check := append([]string{"-C"}, args...)
	add := append([]string{"-A"}, args...)
	if exec.Command("iptables", check...).Run() != nil {
		exec.Command("iptables", add...).Run()
	}
}

func ensureIptablesNat(args ...string) {
	check := append([]string{"-t", "nat", "-C"}, args...)
	add := append([]string{"-t", "nat", "-A"}, args...)
	if exec.Command("iptables", check...).Run() != nil {
		exec.Command("iptables", add...).Run()
	}
}
