// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func (inst Instance) UsesTproxy() bool {
	return inst.RouteThroughXray && inst.XrayRoutingMode == "tproxy"
}

func ValidateTproxySettings(settings string) error {
	var s struct {
		RouteThroughXray bool   `json:"routeThroughXray"`
		XrayRoutingMode  string `json:"xrayRoutingMode"`
		TproxyPort       int    `json:"tproxyPort"`
		Address          string `json:"address"`
		Clients          []struct {
			AllowedIPs []string `json:"allowedIPs"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return err
	}
	if s.XrayRoutingMode != "" && s.XrayRoutingMode != "tun" && s.XrayRoutingMode != "tproxy" {
		return fmt.Errorf("awg: unknown Xray routing mode")
	}
	inst := Instance{
		Id: 1, Ifname: "awg1", RouteThroughXray: s.RouteThroughXray,
		XrayRoutingMode: s.XrayRoutingMode, TproxyPort: s.TproxyPort, Address: s.Address,
	}
	if !inst.UsesTproxy() {
		return nil
	}
	for _, c := range s.Clients {
		inst.Peers = append(inst.Peers, PeerSpec{AllowedIPs: strings.Join(c.AllowedIPs, ",")})
	}
	return inst.validateTproxy()
}

func (inst Instance) validateTproxy() error {
	if inst.Id < 1 || inst.Id > 65535 || inst.Ifname != ifnameFor(inst.Id) {
		return fmt.Errorf("awg: TPROXY requires an inbound ID in 1..65535 and its managed interface")
	}
	if inst.TproxyPort < 1024 || inst.TproxyPort > 65535 {
		return fmt.Errorf("awg: TPROXY port must be in 1024..65535")
	}
	subnet, err := netip.ParsePrefix(inst.Address)
	if err != nil || !subnet.Addr().Is4() || subnet.Bits() < 8 || subnet.Bits() > 30 {
		return fmt.Errorf("awg: TPROXY requires an IPv4 server subnet /8../30")
	}
	for _, peer := range inst.Peers {
		for _, raw := range strings.Split(peer.AllowedIPs, ",") {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			p, err := netip.ParsePrefix(strings.TrimSpace(raw))
			if err != nil || !p.Addr().Is4() || p.Bits() < subnet.Bits() || !subnet.Masked().Contains(p.Addr()) {
				return fmt.Errorf("awg: TPROXY peer AllowedIPs must be IPv4 prefixes inside the server subnet")
			}
		}
	}
	return nil
}

func TproxyInbound(tag string, port int) xray.InboundConfig {
	return xray.InboundConfig{
		Listen: json_util.RawMessage(`"127.0.0.1"`), Port: port, Protocol: "dokodemo-door", Tag: tag,
		Settings:       json_util.RawMessage(`{"allowedNetwork":"tcp,udp","followRedirect":true}`),
		StreamSettings: json_util.RawMessage(`{"sockopt":{"tproxy":"tproxy"}}`),
		Sniffing:       json_util.RawMessage(`{"enabled":true,"destOverride":["http","tls","quic"],"routeOnly":true}`),
	}
}

type tproxyRule struct{ bin, table, chain, spec string }

func tproxyNames(id int) (chain, mark, table string) {
	return fmt.Sprintf("LXAT%d", id), fmt.Sprintf("0x%x/0xffffffff", 0x40000000+id), fmt.Sprint(40000 + id)
}

func tproxyRules(inst Instance) []tproxyRule {
	chain, mark, _ := tproxyNames(inst.Id)
	iface := inst.Ifname
	rules := []tproxyRule{
		{"iptables", "mangle", chain, "-d " + clientSubnet(inst.Address) + " -j RETURN"},
	}
	for _, proto := range []string{"tcp", "udp"} {
		rules = append(rules, tproxyRule{
			"iptables", "mangle", chain,
			fmt.Sprintf("-p %s -j TPROXY --on-ip 127.0.0.1 --on-port %d --tproxy-mark %s", proto, inst.TproxyPort, mark),
		})
	}
	rules = append(rules,
		tproxyRule{"iptables", "filter", "INPUT", "-i " + iface + " -m mark --mark " + mark + " -j ACCEPT"},
	)
	if inst.P2P {
		rules = append(rules, tproxyRule{"iptables", "filter", "FORWARD", "-i " + iface + " -o " + iface + " -j ACCEPT"})
	}
	drop := "-i " + iface + " -j DROP"
	if inst.P2P {
		drop = "-i " + iface + " ! -o " + iface + " -j DROP"
	}
	rules = append(rules,
		tproxyRule{"iptables", "filter", "FORWARD", drop},
		tproxyRule{"ip6tables", "filter", "FORWARD", "-i " + iface + " -j DROP"},
		tproxyRule{"ip6tables", "filter", "INPUT", "-i " + iface + " -j DROP"},
		tproxyRule{"iptables", "mangle", "PREROUTING", "-i " + iface + " -j " + chain},
	)
	return rules
}

func (r tproxyRule) command(op string) string {
	return fmt.Sprintf("%s -w 5 -t %s %s %s %s -m comment --comment lucx-awg-tproxy", r.bin, r.table, op, r.chain, r.spec)
}

func tproxyPostUpPostDown(inst Instance) (string, string) {
	if err := inst.validateTproxy(); err != nil {
		return "false", "true"
	}
	chain, mark, table := tproxyNames(inst.Id)
	selector := fmt.Sprintf("iif %s fwmark %s lookup %s", inst.Ifname, mark, table)
	up := []string{
		"(all=$(ip -4 route show table " + table + " 2>/dev/null | wc -l); owned=$(ip -4 route show table " + table + " proto 242 2>/dev/null | wc -l); [ \"$all\" = \"$owned\" ])",
		"sysctl -qw net.ipv4.ip_forward=1",
		"sysctl -qw net.ipv4.conf." + inst.Ifname + ".rp_filter=2",
		"ip -4 route replace local default dev lo table " + table + " proto 242",
		"(rules=$(ip -4 rule show " + selector + ") && { [ -n \"$rules\" ] || ip -4 rule add pref 10000 " + selector + "; })",
		"(iptables -w 5 -t mangle -S " + chain + " >/dev/null 2>&1 || iptables -w 5 -t mangle -N " + chain + ")",
		"[ -z \"$(iptables -w 5 -t mangle -S " + chain + " | grep -v -e '^-N ' -e 'lucx-awg-tproxy')\" ]",
	}
	rules := tproxyRules(inst)
	for i := len(rules) - 2; i >= 3; i-- {
		r := rules[i]
		up = append(up, "("+r.command("-C")+" 2>/dev/null || "+r.command("-I")+")")
	}
	for _, r := range append(rules[:3:3], rules[len(rules)-1]) {
		op := "-A"
		if r.chain == "PREROUTING" || strings.HasSuffix(r.spec, "-j RETURN") {
			op = "-I"
		}
		up = append(up, "("+r.command("-C")+" 2>/dev/null || "+r.command(op)+")")
	}
	return "(" + strings.Join(up, " && ") + ")", tproxyCleanup(inst.Id)
}

func tproxyCleanup(id int) string {
	if id < 1 || id > 65535 {
		return "true"
	}
	chain, mark, table := tproxyNames(id)
	iface := ifnameFor(id)
	rules := []tproxyRule{
		{"iptables", "mangle", "PREROUTING", "-i " + iface + " -j " + chain},
		{"iptables", "filter", "INPUT", "-i " + iface + " -m mark --mark " + mark + " -j ACCEPT"},
		{"iptables", "filter", "FORWARD", "-i " + iface + " -o " + iface + " -j ACCEPT"},
		{"iptables", "filter", "FORWARD", "-i " + iface + " -j DROP"},
		{"iptables", "filter", "FORWARD", "-i " + iface + " ! -o " + iface + " -j DROP"},
		{"ip6tables", "filter", "FORWARD", "-i " + iface + " -j DROP"},
		{"ip6tables", "filter", "INPUT", "-i " + iface + " -j DROP"},
	}
	down := []string{}
	for _, r := range rules {
		down = append(down, "while "+r.command("-D")+" 2>/dev/null; do :; done")
	}
	down = append(down,
		"if [ -z \"$(iptables -w 5 -t mangle -S "+chain+" 2>/dev/null | grep -v -e '^-N ' -e 'lucx-awg-tproxy')\" ]; then iptables -w 5 -t mangle -F "+chain+" 2>/dev/null || true; iptables -w 5 -t mangle -X "+chain+" 2>/dev/null || true; fi",
		fmt.Sprintf("while ip -4 rule del pref 10000 iif %s fwmark %s lookup %s 2>/dev/null; do :; done", iface, mark, table),
		"ip -4 route del local default dev lo table "+table+" proto 242 2>/dev/null || true",
	)
	return strings.Join(down, "; ")
}

func runTproxyScript(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "sh", "-c", script).Run(); err != nil {
		return fmt.Errorf("awg: TPROXY rules failed: %w", err)
	}
	return nil
}

func (m *Manager) ensureTproxyRouting(inst Instance) {
	if err := inst.validateTproxy(); err != nil {
		logger.Warningf("awg: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if exec.CommandContext(ctx, "ip", "link", "show", inst.Ifname).Run() != nil {
		return
	}
	up, _ := tproxyPostUpPostDown(inst)
	if err := runTproxyScript(up); err != nil {
		logger.Warningf("awg: TPROXY reconcile inbound %d: %v", inst.Id, err)
	}
}

func cleanupTproxyConfig(path string) {
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), xuiManagedMarker) {
		return
	}
	id, ok := parseInboundConfName(filepath.Base(path))
	chain, _, _ := tproxyNames(id)
	if !ok || !strings.Contains(string(data), "-j "+chain+" -m comment --comment lucx-awg-tproxy") {
		return
	}
	if err := runTproxyScript(tproxyCleanup(id)); err != nil {
		logger.Warningf("awg: TPROXY cleanup inbound %d: %v", id, err)
	}
}

func diagnoseTproxy(inst Instance, p prober) []DiagCheck {
	_, mark, table := tproxyNames(inst.Id)
	out, err := p.Run("ip", "-4", "rule", "show", "iif", inst.Ifname, "fwmark", mark, "lookup", table)
	checks := []DiagCheck{{"tproxy policy rule", err == nil && strings.TrimSpace(out) != "", oneLine(out)}}
	out, err = p.Run("ip", "-4", "route", "show", "table", table)
	checks = append(checks, DiagCheck{"tproxy local route", err == nil && strings.Contains(out, "local default dev lo"), oneLine(out)})
	for _, r := range tproxyRules(inst) {
		args := strings.Fields(r.command("-C"))
		_, err = p.Run(args[0], args[1:]...)
		checks = append(checks, DiagCheck{"tproxy " + r.chain, err == nil, r.spec})
	}
	for _, proto := range []string{"-lnt", "-lnu"} {
		out, err = p.Run("ss", proto, "sport", "=", fmt.Sprint(inst.TproxyPort))
		checks = append(checks, DiagCheck{"tproxy listener " + proto, err == nil && strings.Contains(out, fmt.Sprintf("127.0.0.1:%d", inst.TproxyPort)), oneLine(out)})
	}
	return checks
}
