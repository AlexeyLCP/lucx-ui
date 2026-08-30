//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func (m *Manager) ensurePortForwards(inst Instance) {
	comment := portFwdComment(inst.Id)
	flushIptablesComment("nat", "PREROUTING", comment)
	flushIptablesComment("filter", "FORWARD", comment)
	if inst.RouteThroughXray {
		return
	}
	for _, peer := range inst.Peers {
		dest := firstIPv4Host(peer.AllowedIPs)
		ports := parseForwardedPorts(peer.ForwardedPorts)
		if dest == "" || len(ports) == 0 {
			continue
		}
		ext := defaultRouteInterface()
		if !validIptablesIface(ext) {
			ext = ""
		}
		for _, port := range ports {
			ps := strconvPort(port)
			for _, proto := range []string{"tcp", "udp"} {
				dnat := []string{"-t", "nat", "-A", "PREROUTING", "-p", proto, "--dport", ps, "-m", "comment", "--comment", comment, "-j", "DNAT", "--to-destination", dest + ":" + ps}
				if ext != "" {
					dnat = []string{"-t", "nat", "-A", "PREROUTING", "-i", ext, "-p", proto, "--dport", ps, "-m", "comment", "--comment", comment, "-j", "DNAT", "--to-destination", dest + ":" + ps}
				}
				fwd := []string{"-t", "filter", "-A", "FORWARD", "-p", proto, "-d", dest, "--dport", ps, "-m", "comment", "--comment", comment, "-j", "ACCEPT"}
				runIptables(dnat)
				runIptables(fwd)
			}
		}
	}
}

func (m *Manager) flushPortForwards(id int) {
	comment := portFwdComment(id)
	flushIptablesComment("nat", "PREROUTING", comment)
	flushIptablesComment("filter", "FORWARD", comment)
}

func strconvPort(p int) string {
	return fmt.Sprintf("%d", p)
}

func runIptables(args []string) {
	if out, err := exec.CommandContext(context.Background(), "iptables", args...).CombinedOutput(); err != nil {
		logger.Warningf("awg: portfwd iptables %s: %v (%s)", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
}

func flushIptablesComment(table, chain, comment string) {
	out, err := exec.CommandContext(context.Background(), "iptables", "-t", table, "-S", chain).Output()
	if err != nil {
		return
	}
	needle := "--comment " + comment
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		spec := strings.TrimSpace(line)
		if !strings.HasPrefix(spec, "-A ") {
			continue
		}
		args := append([]string{"-t", table}, strings.Fields(strings.Replace(spec, "-A ", "-D ", 1))...)
		_ = exec.CommandContext(context.Background(), "iptables", args...).Run()
	}
}
