// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DiagCheck is one line of the AWG runtime diagnostic: a named probe, whether
// it passed, and a detail string carrying the evidence (command output, peer
// counts, handshake age) so a support screenshot tells the whole story.
type DiagCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Diagnostics is the runtime health report for one AWG inbound: which
// interface it owns, which routing mode it is in, and the ordered probe
// results. Mode is "kernel-nat" (plain kernel forwarding) or "xray-tun"
// (routeThroughXray — policy routing into the Xray TUN inbound).
type Diagnostics struct {
	Ifname string      `json:"ifname"`
	Mode   string      `json:"mode"`
	Checks []DiagCheck `json:"checks"`
}

// awg3SupportCheckName is the DiagCheck.Name of the AWG3 capability line,
// which Healthy() treats as informational.
const awg3SupportCheckName = "awg3 support"

// awg31SupportCheckName is the DiagCheck.Name of the AWG 3.1 capability line,
// also informational (a pre-3.1 host just omits RandomTrailers/DisableCookies).
const awg31SupportCheckName = "awg3.1 support"

// Healthy reports whether every probe passed. The "awg3 support"/"awg3.1
// support" lines are capability reports, not faults — a pre-AWG3 host simply
// renders configs without those fields and serves traffic fine — so they are
// excluded.
func (d Diagnostics) Healthy() bool {
	for _, c := range d.Checks {
		if !c.OK && c.Name != awg3SupportCheckName && c.Name != awg31SupportCheckName {
			return false
		}
	}
	return true
}

// prober runs a diagnostic command and returns its combined output. Abstracted
// so tests can replay recorded outputs instead of touching the host network.
type prober interface {
	Run(name string, args ...string) (string, error)
}

type execProber struct{}

func (execProber) Run(name string, args ...string) (string, error) {
	ifname := probedIfname(name, args)
	if err := stuckReadErr(ifname); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), awgShowTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, awgBin(name), args...).CombinedOutput()
	if ifname != "" {
		noteAwgRead(ctx, ifname, err)
	}
	return string(out), err
}

// probedIfname names the device an `awg show <if> …` probe reads, so its stuck
// warning shares the latch with the traffic scrapers; "" for host-wide probes.
func probedIfname(name string, args []string) string {
	if name == "awg" && len(args) >= 2 && args[0] == "show" {
		return args[1]
	}
	return ""
}

// Diagnose probes the live kernel state of an instance (interface, forwarding,
// peers, and the mode-specific routing rules) and returns the report rendered
// by the panel's AWG diagnostics view. It never mutates state — fixes belong
// to the reconcile loop (ensureNatRules / ensureXrayRouting); this only makes
// their failures visible.
func Diagnose(inst Instance) Diagnostics {
	return diagnose(inst, execProber{}, time.Now)
}

func diagnose(inst Instance, p prober, now func() time.Time) Diagnostics {
	mode := "kernel-nat"
	if inst.RouteThroughXray {
		mode = "xray-tun"
	}
	d := Diagnostics{Ifname: inst.Ifname, Mode: mode}

	out, err := p.Run("ip", "link", "show", inst.Ifname)
	if err != nil {
		d.Checks = append(d.Checks, DiagCheck{
			"interface " + inst.Ifname, false,
			"missing — sidecar not running (awg-quick up never happened or interface was deleted): " + oneLine(out),
		})
		return d
	}
	if strings.Contains(out, "state UP") || strings.Contains(out, ",UP,") || strings.Contains(out, " UP ") {
		d.Checks = append(d.Checks, DiagCheck{"interface " + inst.Ifname, true, "up"})
	} else {
		d.Checks = append(d.Checks, DiagCheck{
			"interface " + inst.Ifname, false,
			"exists but not UP: " + oneLine(out),
		})
	}

	out, err = p.Run("sysctl", "-n", "net.ipv4.ip_forward")
	fwd := err == nil && strings.TrimSpace(out) == "1"
	d.Checks = append(d.Checks, DiagCheck{
		"ip_forward", fwd,
		fmt.Sprintf("net.ipv4.ip_forward=%s (client packets are dropped without it)", strings.TrimSpace(out)),
	})

	d.Checks = append(d.Checks, awg3CapabilityCheck(p))
	d.Checks = append(d.Checks, awg31CapabilityCheck(p))

	peersOut, peersErr := p.Run("awg", "show", inst.Ifname, "peers")
	hsOut, hsErr := p.Run("awg", "show", inst.Ifname, "latest-handshakes")
	if peersErr != nil || hsErr != nil {
		d.Checks = append(d.Checks, DiagCheck{
			"wireguard peers", false,
			"awg show failed — amneziawg tools missing or interface not a wg device: " + oneLine(peersOut+hsOut),
		})
	} else {
		peers := countLines(peersOut)
		live, latest := parseLatestHandshakes(hsOut, now)
		detail := fmt.Sprintf("%d peer(s) configured, %d with handshake", peers, live)
		if latest >= 0 {
			detail += fmt.Sprintf(", latest %s ago", (time.Duration(latest) * time.Second).Round(time.Second))
		}
		d.Checks = append(d.Checks, DiagCheck{"wireguard peers", peers > 0, detail})
	}

	if inst.RouteThroughXray {
		d.Checks = append(d.Checks, diagnoseXrayTun(inst, p)...)
	} else {
		d.Checks = append(d.Checks, diagnoseKernelNAT(inst, p)...)
	}
	return d
}

// diagnoseXrayTun probes the routeThroughXray chain: Xray-owned tunN device,
// iif policy rule into the per-inbound table, and the table's default route.
func diagnoseXrayTun(inst Instance, p prober) []DiagCheck {
	var checks []DiagCheck
	tunName := tunNameFor(inst.Id)
	table := awgRouteTable(inst.Id)

	out, err := p.Run("ip", "link", "show", tunName)
	if err != nil {
		checks = append(checks, DiagCheck{
			"xray tun " + tunName, false,
			"missing — Xray is down, restarting, or the TUN inbound is not in its config (needRestart did not fire): " + oneLine(out),
		})
	} else {
		checks = append(checks, DiagCheck{"xray tun " + tunName, true, "present"})
	}

	out, err = p.Run("ip", "rule", "show", "iif", inst.Ifname)
	if err != nil || ruleMissing(out, table) {
		checks = append(checks, DiagCheck{
			"policy rule", false,
			fmt.Sprintf("no 'iif %s lookup %d' — client packets never reach the table: %s", inst.Ifname, table, oneLine(out)),
		})
	} else {
		checks = append(checks, DiagCheck{
			"policy rule", true,
			fmt.Sprintf("iif %s lookup %d", inst.Ifname, table),
		})
	}

	out, err = p.Run("ip", "route", "show", "table", strconv.Itoa(table))
	if err != nil || !strings.Contains(out, "default") {
		checks = append(checks, DiagCheck{
			"route table " + strconv.Itoa(table), false,
			fmt.Sprintf("no default route (dies with %s on every Xray restart; reconcile re-adds within 10s): %s", tunName, oneLine(out)),
		})
	} else {
		checks = append(checks, DiagCheck{"route table " + strconv.Itoa(table), true, oneLine(out)})
	}
	return checks
}

// diagnoseKernelNAT probes the plain-routing chain: packets from awgN are
// MARKed in mangle/PREROUTING and MASQUERADEd by mark out the default-route
// interface (lucx.84+; not -s <subnet>, so peers outside the server Address/24
// still NAT). Plus FORWARD accepts on both awgN legs.
func diagnoseKernelNAT(inst Instance, p prober) []DiagCheck {
	var checks []DiagCheck
	routeOut, _ := p.Run("ip", "-o", "-4", "route", "show", "default")
	extIface := parseDefaultRouteInterface(routeOut)
	if inst.Ifname == "" || extIface == "" {
		return append(checks, DiagCheck{
			"masquerade", false,
			fmt.Sprintf("cannot derive NAT parameters (ifname=%q, default-route iface=%q)", inst.Ifname, extIface),
		})
	}

	mark := strconv.Itoa(awgNatMark(inst.Id))
	subnet := clientSubnet(inst.Address)
	markArgs := []string{"-t", "mangle", "-C", "PREROUTING", "-i", inst.Ifname, "-j", "MARK", "--set-mark", mark}
	subnetArgs := []string{"-t", "nat", "-C", "POSTROUTING", "-s", subnet, "-o", extIface, "-j", "MASQUERADE"}
	masqArgs := []string{"-t", "nat", "-C", "POSTROUTING", "-m", "mark", "--mark", mark, "-o", extIface, "-j", "MASQUERADE"}
	_, markErr := p.Run("iptables", markArgs...)
	_, subnetErr := p.Run("iptables", subnetArgs...)
	_, masqErr := p.Run("iptables", masqArgs...)
	if subnetErr == nil || masqErr == nil {
		detail := fmt.Sprintf("MASQUERADE -o %s", extIface)
		if subnetErr == nil && subnet != "" {
			detail = fmt.Sprintf("MASQUERADE -s %s -o %s", subnet, extIface)
		}
		if masqErr == nil {
			detail += fmt.Sprintf(" + mark=%s", mark)
		}
		if markErr != nil {
			detail += " (mangle MARK missing — out-of-subnet peers may leak)"
		}
		checks = append(checks, DiagCheck{"masquerade", true, detail})
	} else {
		checks = append(checks, DiagCheck{
			"masquerade", false,
			fmt.Sprintf("missing POSTROUTING MASQUERADE -s %s / mark=%s -o %s (flushed? fail2ban/docker reload?) — reconcile re-adds within 10s",
				subnet, mark, extIface),
		})
	}

	missing := []string{}
	for _, dir := range []string{"-i", "-o"} {
		if _, err := p.Run("iptables", "-C", "FORWARD", dir, inst.Ifname, "-j", "ACCEPT"); err != nil {
			missing = append(missing, dir+" "+inst.Ifname)
		}
	}
	if len(missing) > 0 {
		checks = append(checks, DiagCheck{
			"forward chain", false,
			"missing FORWARD ACCEPT for " + strings.Join(missing, ", "),
		})
	} else {
		checks = append(checks, DiagCheck{"forward chain", true, "FORWARD ACCEPT both legs of " + inst.Ifname})
	}
	return checks
}

// parseDefaultRouteInterface extracts the dev name from `ip -o -4 route show
// default` output ("default via 10.0.0.1 dev eth0 proto static").
func parseDefaultRouteInterface(out string) string {
	fields := strings.Fields(strings.TrimSpace(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// parseLatestHandshakes parses `awg show <iface> latest-handshakes` output
// ("<pubkey>\t<unix-ts>" per line) into the count of peers with a completed
// handshake and the age in seconds of the most recent one (-1 when none).
func parseLatestHandshakes(out string, now func() time.Time) (live int, latestAge int64) {
	latestAge = -1
	var maxTS int64
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		ts, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || ts <= 0 {
			continue
		}
		live++
		if ts > maxTS {
			maxTS = ts
		}
	}
	if maxTS > 0 {
		latestAge = now().Unix() - maxTS
		if latestAge < 0 {
			latestAge = 0
		}
	}
	return live, latestAge
}

func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// oneLine collapses command output to a single trimmed line for Detail strings.
func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}
