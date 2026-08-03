//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// defaultRouteInterface returns the name of the interface holding the default
// route (the one that would carry outbound traffic to the internet). Used as
// the -o target for the MASQUERADE rule in PostUp. Returns empty when no
// default route exists (an unusual server, but we degrade gracefully: PostUp
// uses the rule but iptables will simply fail to match, which is logged but
// non-fatal).
func defaultRouteInterface() string {
	out, err := exec.CommandContext(context.Background(), "ip", "-o", "-4", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	return parseDefaultRouteInterface(string(out))
}

// killStrayAwgInterfaces removes AWG kernel interfaces left over from a
// previous x-ui run and returns how many were removed. A survivor holds the
// inbound's UDP port with stale obfuscation, so new clients cannot connect.
// x-ui is the sole owner of awgN interfaces, so any "awg*" interface at
// startup is an orphan and is safe to delete. Routing of decrypted traffic
// into Xray is via an injected TUN inbound (no tun2socks daemon), so there
// are no userspace orphans to sweep — the TUN device is owned by Xray and
// dies with it.
//
// IMPORTANT: this sweep must NOT touch "awgo-*" interfaces — those belong to
// the AWG outbound subsystem (client-mode tunnels named "awgo-{Id}"). The
// outbound reconcile job only runs on the first EnsureClient(), which happens
// AFTER the inbound Ensure() that calls this sweep; if we deleted awgo-*
// here, every panel restart would tear down the operator's outbound tunnels
// and then re-create them a moment later, causing a brief traffic outage on
// every restart. The filter therefore matches inbound interfaces only —
// "awg" followed by a digit (awg1, awg2, ...) — and explicitly excludes the
// "awgo-" prefix.
func killStrayAwgInterfaces() int {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		name := e.Name()
		if !isInboundAwgInterface(name) {
			continue
		}
		if err := exec.CommandContext(context.Background(), "ip", "link", "del", name).Run(); err == nil {
			killed++
		}
	}
	return killed
}

// isInboundAwgInterface reports whether name is an inbound AWG interface
// (e.g. "awg1") and NOT an outbound one (e.g. "awgo-1"). The rule: the name
// starts with "awg" and the next character is a digit. The "awgo-" prefix
// is rejected because 'o' is not a digit. This is deliberately stricter than
// `strings.HasPrefix(name, "awg")` so the inbound orphan sweep can never
// collide with the outbound interface namespace.
func isInboundAwgInterface(name string) bool {
	if !strings.HasPrefix(name, "awg") {
		return false
	}
	rest := name[len("awg"):]
	if rest == "" {
		return false
	}
	r := rune(rest[0])
	return r >= '0' && r <= '9'
}

var (
	moduleAwg3Checked   bool
	moduleAwg3Supported bool
)

// ModuleSupportsAwg3 reports whether the installed amneziawg kernel module
// is v3.x (which parses ContentPaddingAddition/RekeyAfterTime/etc/
// HeaderProtectionKey in setconf). Cached after the first call. Returns
// false on non-Linux or when modinfo fails (dev/build hosts, module not
// loaded yet) so renderers degrade AWG3 fields gracefully — never emit a
// v3-only line into a .conf unless the running kernel will accept it.
// Without this gate, an operator who picks awgVersion "3" on a host still
// running the v1.x module gets "Line unrecognized: ContentPaddingAddition=64"
// from awg setconf, awg-quick deletes the half-built interface, and every
// reconcile fails with "Device <awgN> does not exist".
func ModuleSupportsAwg3() bool {
	if moduleSupportsAwg3Override != nil {
		return *moduleSupportsAwg3Override
	}
	if moduleAwg3Checked {
		return moduleAwg3Supported
	}
	moduleAwg3Checked = true
	out, err := exec.CommandContext(context.Background(), "modinfo", "-F", "version", "amneziawg").Output()
	if err != nil {
		return false
	}
	ver := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if ver == "" {
		return false
	}
	fields := strings.Split(ver, ".")
	if len(fields) < 1 {
		return false
	}
	major := strings.TrimPrefix(fields[0], "v")
	moduleAwg3Supported = strings.HasPrefix(major, "3")
	return moduleAwg3Supported
}

// SetModuleSupportsAwg3 overrides the module-support probe for tests.
// Pass nil to clear the override (restore real probing). Test-only helper.
func SetModuleSupportsAwg3(supported *bool) {
	moduleSupportsAwg3Override = supported
}

var moduleSupportsAwg3Override *bool
