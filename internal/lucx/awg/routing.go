// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os/exec"
	"strings"
)

// SetupTUNRouting creates and configures all routing for AWG → TUN traffic.
// Fully idempotent — every operation uses 2>/dev/null || true or check-before-add.
func SetupTUNRouting(cfg RoutingConfig) error {
	logAWG("SetupTUNRouting: awg=%s tun=%s subnet=%s table=%s", cfg.AWGInterface, cfg.TUNInterface, cfg.AWGSubnet, cfg.RouteTable)

	// 1. Create and bring up AWG interface
	if err := SetupAWGInterface(cfg.AWGInterface); err != nil {
		return err
	}
	runCmd("ip", "addr", "add", cfg.AWGServerIP+"/24", "dev", cfg.AWGInterface)
	runCmd("ip", "link", "set", cfg.AWGInterface, "mtu", fmt.Sprintf("%d", cfg.MTU))
	runCmd("ip", "link", "set", cfg.AWGInterface, "up")

	// 2. Kernel forwarding
	EnableForwarding()

	// 3. TUN interface — always up, correct MTU
	runCmd("ip", "link", "set", cfg.TUNInterface, "mtu", fmt.Sprintf("%d", cfg.MTU))
	runCmd("ip", "link", "set", cfg.TUNInterface, "up")

	// 4. Policy routing (idempotent — add || true)
	runCmd("ip", "rule", "add", "from", cfg.AWGSubnet, "table", cfg.RouteTable, "pref", fmt.Sprintf("%d", cfg.RoutePref))
	runCmd("ip", "route", "add", "default", "dev", cfg.TUNInterface, "table", cfg.RouteTable)

	// 5. Firewall — check before add for idempotency
	ensureIptables("FORWARD", "-i", cfg.AWGInterface, "-o", cfg.TUNInterface, "-j", "ACCEPT")
	ensureIptables("FORWARD", "-i", cfg.TUNInterface, "-o", cfg.AWGInterface, "-j", "ACCEPT")
	ensureIptablesNat("POSTROUTING", "-s", cfg.AWGSubnet, "-o", cfg.TUNInterface, "-j", "MASQUERADE")

	return nil
}

// CleanupTUNRouting removes all routing and firewall rules for an AWG interface.
// Fully idempotent — every operation uses 2>/dev/null || true.
func CleanupTUNRouting(cfg RoutingConfig) {
	logAWG("CleanupTUNRouting: awg=%s tun=%s", cfg.AWGInterface, cfg.TUNInterface)

	// Firewall
	runCmd("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", cfg.AWGSubnet, "-o", cfg.TUNInterface, "-j", "MASQUERADE")
	runCmd("iptables", "-D", "FORWARD", "-i", cfg.AWGInterface, "-o", cfg.TUNInterface, "-j", "ACCEPT")
	runCmd("iptables", "-D", "FORWARD", "-i", cfg.TUNInterface, "-o", cfg.AWGInterface, "-j", "ACCEPT")

	// Routes
	runCmd("ip", "route", "del", "default", "dev", cfg.TUNInterface, "table", cfg.RouteTable)
	runCmd("ip", "rule", "del", "from", cfg.AWGSubnet, "table", cfg.RouteTable, "pref", fmt.Sprintf("%d", cfg.RoutePref))

	// Interface
	DeleteAWGInterface(cfg.AWGInterface)
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
	return repaired
}

// --- Internal helpers ---

func runCmd(args ...string) {
	exec.Command(args[0], args[1:]...).Run()
}

func routeExists(subnet, table string) bool {
	out, err := exec.Command("ip", "route", "show", "table", table).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), subnet)
}

func iptablesRuleExists(chain, iface1, iface2 string) bool {
	return exec.Command("iptables", "-C", chain, "-i", iface1, "-o", iface2, "-j", "ACCEPT").Run() == nil
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
