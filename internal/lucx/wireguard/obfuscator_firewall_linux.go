// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package wireguard

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// wgoFirewallOwner marks the owner-commented iptables rules that hide the
// direct kernel WireGuard UDP port while wgobfs obfuscation is active.
const wgoFirewallOwner = "lucx-wgo"

// wgoIptables runs iptables; a variable so tests can fake it.
var wgoIptables = func(args ...string) ([]byte, error) {
	return exec.Command(args[0], args[1:]...).CombinedOutput()
}

// wgoDropArgs builds the owner-commented DROP rule argv for one operation
// (-C check / -I insert / -D delete): external UDP to the WG port dies,
// loopback (the wgobfs forwarder) stays allowed.
func wgoDropArgs(st WgoFirewallState, op string) []string {
	args := []string{"iptables", "-w", "5", op, "INPUT"}
	if op == "-I" {
		args = append(args, "1")
	}
	return append(args,
		"-p", "udp",
		"--dport", strconv.Itoa(st.WGPort),
		"!", "-i", "lo",
		"-m", "comment", "--comment", fmt.Sprintf("%s-%d", wgoFirewallOwner, st.ID),
		"-j", "DROP",
	)
}

var wgoCommentRe = regexp.MustCompile(`--comment "?` + wgoFirewallOwner + `-(\d+)"?`)

// parseWgoRules extracts (inbound id, rule spec) pairs of our owner-marked
// rules from `iptables -S INPUT` output. The spec (without the -A INPUT
// prefix) is reusable verbatim as the -D argument.
func parseWgoRules(dump string) map[int]string {
	rules := map[int]string{}
	for _, line := range strings.Split(dump, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "-A INPUT ")
		if !ok {
			continue
		}
		m := wgoCommentRe.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		id, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		rules[id] = rest
	}
	return rules
}

// wgoDeleteArgs rebuilds a -D argv from one parsed rule spec, stripping the
// optional quotes iptables -S puts around the comment value.
func wgoDeleteArgs(spec string) []string {
	parts := strings.Fields(spec)
	for i, p := range parts {
		parts[i] = strings.Trim(p, `"`)
	}
	return append([]string{"iptables", "-w", "5", "-D", "INPUT"}, parts...)
}

// EnsureWgoFirewall adds or removes the direct-port DROP rule of one
// inbound to match st.Block. Removing a missing rule is success.
func EnsureWgoFirewall(st WgoFirewallState) error {
	if st.Block {
		if _, err := wgoIptables(wgoDropArgs(st, "-C")...); err == nil {
			return nil
		}
		_, err := wgoIptables(wgoDropArgs(st, "-I")...)
		return err
	}
	wgoIptables(wgoDropArgs(st, "-D")...)
	return nil
}

// SyncWgoFirewall ensures every desired state and sweeps rules whose
// inbound no longer exists (stale lucx-wgo-<id> comments).
func SyncWgoFirewall(states []WgoFirewallState) {
	active := map[int]bool{}
	for _, st := range states {
		active[st.ID] = true
		if err := EnsureWgoFirewall(st); err != nil {
			logger.Warningf("wgo: firewall rule for inbound %d: %v", st.ID, err)
		}
	}
	out, err := wgoIptables("iptables", "-w", "5", "-S", "INPUT")
	if err != nil {
		return
	}
	for id, spec := range parseWgoRules(string(out)) {
		if active[id] {
			continue
		}
		if _, err := wgoIptables(wgoDeleteArgs(spec)...); err != nil {
			logger.Warningf("wgo: sweep stale rule for inbound %d: %v", id, err)
		}
	}
}
