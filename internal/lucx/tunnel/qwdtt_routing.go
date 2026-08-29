// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// qWDTT kernel ifaces created by the SpaceNeuroX binary (WG path + optional raw).
const (
	qwdttIfaceWG  = "wdtt0"
	qwdttIfaceRaw = "wdttraw0"
	// Subnets claimed by the binary for client addresses (server.go).
	qwdttSubnetWG  = "10.66.0.0/16"
	qwdttSubnetRaw = "10.70.0.0/16"
)

// QwdttTunName returns the Xray TUN device name for a qWDTT inbound id
// (mirrors AWG tun{N}).
func QwdttTunName(inboundID int) string {
	return "tun" + strconv.Itoa(inboundID)
}

// QwdttRouteTable is the policy-routing table for qWDTT → Xray TUN.
// Offset 1900 keeps clear of AWG's 1000+N and common admin tables.
func QwdttRouteTable(inboundID int) int {
	return 1900 + inboundID
}

// QwdttTunGateway is the /30 gateway on the Xray TUN (outside AWG 10.254.N and
// qWDTT client subnets).
func QwdttTunGateway(inboundID int) string {
	if inboundID >= 1 && inboundID < 254 {
		return "10.253." + strconv.Itoa(inboundID) + ".1/30"
	}
	return "10.251." + strconv.Itoa((inboundID%253)+1) + ".1/30"
}

// ensureQwdttXrayRouting converges kernel state so traffic from wdtt0/wdttraw0
// enters the Xray TUN instead of the binary's MASQUERADE-to-eth0 path.
// Idempotent; safe while Xray/tun is down (no-op until link exists).
func ensureQwdttXrayRouting(inst Instance) {
	if runtime.GOOS != "linux" || !inst.RouteThroughXray {
		return
	}
	tun := strings.TrimSpace(inst.TunName)
	if tun == "" {
		return
	}
	table := inst.RouteTable
	if table <= 0 {
		return
	}
	ifaces := inst.RouteIfaces
	if len(ifaces) == 0 {
		ifaces = []string{qwdttIfaceWG, qwdttIfaceRaw}
	}

	// Wait briefly for wdtt0 after process start.
	deadline := time.Now().Add(3 * time.Second)
	for exec.CommandContext(context.Background(), "ip", "link", "show", qwdttIfaceWG).Run() != nil && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}

	if exec.CommandContext(context.Background(), "ip", "link", "show", tun).Run() != nil {
		return
	}

	runQuiet("ip", "route", "replace", "default", "dev", tun, "table", strconv.Itoa(table))
	runQuiet("sysctl", "-qw", "net.ipv4.conf."+tun+".rp_filter=2")
	runQuiet("sysctl", "-qw", "net.ipv4.ip_forward=1")

	for _, iface := range ifaces {
		if exec.CommandContext(context.Background(), "ip", "link", "show", iface).Run() != nil {
			continue
		}
		out, err := exec.CommandContext(context.Background(), "ip", "rule", "show", "iif", iface).Output()
		if err != nil || ruleMissingLookup(string(out), table) {
			if o2, err2 := exec.CommandContext(context.Background(), "ip", "rule", "add", "iif", iface, "lookup", strconv.Itoa(table)).CombinedOutput(); err2 != nil {
				logger.Warningf("tunnel: qwdtt rule iif %s lookup %d: %v\n%s", iface, table, err2, string(o2))
			}
		}
	}

	// Drop binary-installed MASQUERADE so replies are not double-NATed via eth0.
	stripQwdttMasquerade()
}

func stripQwdttMasquerade() {
	for _, subnet := range []string{qwdttSubnetWG, qwdttSubnetRaw} {
		// Delete repeatedly until gone (binary may have added one rule).
		for i := 0; i < 4; i++ {
			out, err := exec.CommandContext(context.Background(), "iptables", "-t", "nat", "-D", "POSTROUTING",
				"-s", subnet, "-j", "MASQUERADE").CombinedOutput()
			if err != nil {
				_ = out
				break
			}
		}
	}
}

func ruleMissingLookup(ruleOutput string, table int) bool {
	needle := "lookup " + strconv.Itoa(table)
	for _, line := range strings.Split(ruleOutput, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), needle) {
			return false
		}
	}
	return true
}

func runQuiet(name string, args ...string) {
	if out, err := exec.CommandContext(context.Background(), name, args...).CombinedOutput(); err != nil {
		logger.Warningf("tunnel: qwdtt routing %s %s: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
}
