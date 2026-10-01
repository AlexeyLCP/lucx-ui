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
	qwdttSubnetWG       = "10.68.0.0/16"
	qwdttSubnetWGLegacy = "10.66.0.0/16"
	qwdttSubnetRaw      = "10.70.0.0/16"

	csqttIface      = "csqtt1"
	csqttSubnet     = "10.66.67.0/24"
	csqttRouteTable = 1910
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

	waitIface := qwdttIfaceWG
	if len(ifaces) > 0 {
		waitIface = ifaces[0]
	}
	deadline := time.Now().Add(3 * time.Second)
	for exec.CommandContext(context.Background(), "ip", "link", "show", waitIface).Run() != nil && time.Now().Before(deadline) {
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

	stripQwdttMasquerade()
	if inst.Core == Csqtt {
		stripMasqueradeSubnet(csqttSubnet)
		applyCsqttFirewall(csqttXrayFirewall(tun))
	}
}

// EnsureCsqttDirect is the routeThroughXray=off path. The binary creates
// csqtt1 but does not install NAT (that lives in upstream deploy.sh, which
// LucX does not run). Also drops a leftover iif rule so packets are not
// sent into a tun we are no longer bridging.
func EnsureCsqttDirect() {
	if runtime.GOOS != "linux" {
		return
	}
	clearQwdttXrayRouting(csqttRouteTable, []string{csqttIface})
	applyCsqttFirewall(csqttDirectFirewall())
}

type csqttFirewallSpec struct {
	table string
	chain string
	spec  []string
}

func csqttIfaceRules(iface string) []csqttFirewallSpec {
	return []csqttFirewallSpec{
		{"", "FORWARD", []string{"-i", iface, "-j", "ACCEPT"}},
		{"", "FORWARD", []string{"-o", iface, "-j", "ACCEPT"}},
		{"mangle", "FORWARD", []string{"-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-i", iface, "-j", "TCPMSS", "--clamp-mss-to-pmtu"}},
		{"mangle", "FORWARD", []string{"-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-o", iface, "-j", "TCPMSS", "--clamp-mss-to-pmtu"}},
	}
}

func csqttXrayFirewall(tun string) []csqttFirewallSpec {
	rules := csqttIfaceRules(csqttIface)
	if tun != "" {
		rules = append(rules, csqttIfaceRules(tun)...)
	}
	return rules
}

func csqttDirectFirewall() []csqttFirewallSpec {
	return append(csqttIfaceRules(csqttIface), csqttFirewallSpec{
		table: "nat", chain: "POSTROUTING",
		spec: []string{"-s", csqttSubnet, "-j", "MASQUERADE"},
	})
}

func applyCsqttFirewall(rules []csqttFirewallSpec) {
	if runtime.GOOS != "linux" {
		return
	}
	runQuiet("sysctl", "-qw", "net.ipv4.ip_forward=1")
	runQuiet("sysctl", "-qw", "net.ipv4.conf."+csqttIface+".rp_filter=2")
	for _, r := range rules {
		ensureIptables(r.table, r.chain, r.spec...)
	}
}

func ensureIptables(table, chain string, spec ...string) {
	if exec.CommandContext(context.Background(), "iptables", iptablesOp(table, "-C", chain, spec)...).Run() == nil {
		return
	}
	// -I 1 so a UFW reject later in FORWARD cannot drop the new flow.
	if chain == "FORWARD" {
		runQuiet("iptables", iptablesOp(table, "-I", chain, append([]string{"1"}, spec...))...)
		return
	}
	runQuiet("iptables", iptablesOp(table, "-A", chain, spec)...)
}

func iptablesOp(table, op, chain string, spec []string) []string {
	args := make([]string, 0, 6+len(spec))
	if table != "" && table != "filter" {
		args = append(args, "-t", table)
	}
	args = append(args, op, chain)
	return append(args, spec...)
}

func CsqttTunName(inboundID int) string {
	return "tun" + strconv.Itoa(inboundID)
}

func CsqttTunGateway(inboundID int) string {
	if inboundID >= 1 && inboundID < 254 {
		return "10.252." + strconv.Itoa(inboundID) + ".1/30"
	}
	return "10.250." + strconv.Itoa((inboundID%253)+1) + ".1/30"
}

func clearQwdttRoutingForKey(key string) {
	if key == CsqttKey {
		clearQwdttXrayRouting(csqttRouteTable, []string{csqttIface})
		return
	}
	const p = "qwdtt-"
	if !strings.HasPrefix(key, p) {
		return
	}
	id, err := strconv.Atoi(key[len(p):])
	if err != nil || id <= 0 {
		return
	}
	clearQwdttXrayRouting(QwdttRouteTable(id), nil)
}

func clearQwdttXrayRouting(table int, ifaces []string) {
	if runtime.GOOS != "linux" || table <= 0 {
		return
	}
	if len(ifaces) == 0 {
		ifaces = []string{qwdttIfaceWG, qwdttIfaceRaw}
	}
	ts := strconv.Itoa(table)
	for _, iface := range ifaces {
		_ = exec.CommandContext(context.Background(), "ip", "rule", "del", "iif", iface, "lookup", ts).Run()
	}
	_ = exec.CommandContext(context.Background(), "ip", "route", "flush", "table", ts).Run()
}

func stripQwdttMasquerade() {
	for _, subnet := range []string{qwdttSubnetWG, qwdttSubnetWGLegacy, qwdttSubnetRaw} {
		stripMasqueradeSubnet(subnet)
	}
}

func stripMasqueradeSubnet(subnet string) {
	for i := 0; i < 4; i++ {
		out, err := exec.CommandContext(context.Background(), "iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", subnet, "-j", "MASQUERADE").CombinedOutput()
		if err != nil {
			_ = out
			return
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
