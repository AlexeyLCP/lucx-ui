//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func renameAwgInterface(oldName, newName string) error {
	if oldName == "" || oldName == newName {
		return nil
	}
	out, err := exec.CommandContext(context.Background(), "ip", "link", "set", oldName, "name", newName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip link set %s name %s: %w (%s)", oldName, newName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

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
// Only interfaces whose .conf carries the x-ui ownership marker are removed.
// Foreign awg0/awg1 (awg-multi-script, toolza3, WGDashboard) share the same
// name pattern and must stay up until the operator imports them.
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
		if !strayInterfaceIsOurs(name) {
			continue
		}
		if err := exec.CommandContext(context.Background(), "ip", "link", "del", name).Run(); err == nil {
			killed++
		}
	}
	return killed
}

var (
	moduleAwg3Checked   bool
	moduleAwg3Supported bool
)

// ModuleSupportsAwg3 reports whether this host can consume AWG3 fields
// (HeaderProtectionKey + the device timers/padding). Two independent
// capabilities are required: the LOADED amneziawg kernel module must export
// the header-protection symbols (kernel side of setconf), and the installed
// awg userspace tools must parse the HeaderProtectionKey .conf line
// (amneziawg-tools v3.0+, config.c parse_key). Either one missing means the
// renderers must omit every AWG3-only line, or awg setconf / awg-quick dies
// with "Line unrecognized" + "Configuration parsing error", awg-quick rolls
// the half-built interface back, and every reconcile fails with "Device
// <awgN> does not exist".
//
// The probe is functional, not version-based. Upstream hardcodes
// PACKAGE_VERSION="1.0.0" (dkms.conf) and WIREGUARD_VERSION=1.0.0 (Makefile)
// in EVERY release, so modinfo reports the same "1.0.0" for the pre-AWG3
// tags (v1.0.20260611 …) and the AWG3 tags (v3.0.20260730 …) — the previous
// major=="3" parse never matched and silently dropped HPK on every host,
// including hosts whose module WAS rebuilt from master.
//
// Only a positive result is cached. A negative one is transient (module not
// loaded yet right after boot, tools mid-rebuild during an update), so the
// next call retries and a host upgraded to AWG3 picks the fields up within
// one reconcile tick — no panel restart needed.
func ModuleSupportsAwg3() bool {
	if moduleSupportsAwg3Override != nil {
		return *moduleSupportsAwg3Override
	}
	if moduleAwg3Checked {
		return moduleAwg3Supported
	}
	supported := kernelExportsHeaderProtection() && awgToolsParseHeaderProtectionKey()
	if supported {
		moduleAwg3Checked = true
		moduleAwg3Supported = true
	}
	return supported
}

// kernelExportsHeaderProtection reports whether the loaded amneziawg module
// is an AWG3 build. header_protection.c (merged upstream 2026-07-30, first
// released in tag v3.0.20260730) defines the non-static symbol
// awg_header_protection_set_key, and a loaded module's global symbols appear
// in /proc/kallsyms. Pre-AWG3 modules — and no module at all — leave no such
// line. Unreadable kallsyms degrades to false, which is always safe:
// renderers simply omit the AWG3 fields.
func kernelExportsHeaderProtection() bool {
	f, err := os.Open("/proc/kallsyms")
	if err != nil {
		return false
	}
	defer f.Close()
	return kallsymsHasSymbol(f, "awg_header_protection_set_key")
}

// kallsymsHasSymbol scans kallsyms output for a symbol name. Extracted for
// tests; a real /proc/kallsyms line looks like
// "ffffffffc05a8e10 T awg_header_protection_set_key\t[amneziawg]".
func kallsymsHasSymbol(r io.Reader, symbol string) bool {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if strings.Contains(sc.Text(), symbol) {
			return true
		}
	}
	return false
}

// awgToolsParseHeaderProtectionKey reports whether the installed awg tools
// accept HeaderProtectionKey in a .conf file (amneziawg-tools v3.0+; older
// tools abort awg-quick with "Line unrecognized"). `awg version` prints
// "amneziawg-tools v3.0.20260730 - https://amnezia.org" — src/version.h
// carries a floor version when git-describe fails, so every sane build
// reports one. A missing binary or an unparsable banner conservatively
// returns false.
func awgToolsParseHeaderProtectionKey() bool {
	out, err := exec.CommandContext(context.Background(), awgBin("awg"), "version").Output()
	if err != nil {
		return false
	}
	return parseMajorVersion(string(out)) >= 3
}

// parseMajorVersion extracts the major number of the first "v<digits>" token
// in s ("amneziawg-tools v3.0.20260730 - https://amnezia.org" → 3). Returns
// -1 when no such token exists.
func parseMajorVersion(s string) int {
	for i := 0; i+1 < len(s); i++ {
		if s[i] != 'v' || s[i+1] < '0' || s[i+1] > '9' {
			continue
		}
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		major, err := strconv.Atoi(s[i+1 : j])
		if err != nil {
			return -1
		}
		return major
	}
	return -1
}

// SetModuleSupportsAwg3 overrides the module-support probe for tests.
// Pass nil to clear the override (restore real probing). Test-only helper.
func SetModuleSupportsAwg3(supported *bool) {
	moduleSupportsAwg3Override = supported
}

var moduleSupportsAwg3Override *bool

var (
	moduleAwg31Checked   bool
	moduleAwg31Supported bool
)

var moduleSupportsAwg31Override *bool

// ModuleSupportsAwg31 reports whether this host can consume AWG 3.1 fields
// (RandomTrailers / DisableCookies). Tools older than v3.1 reject those
// .conf lines with "Line unrecognized" and awg-quick rolls the interface
// back — same Pattern 1d as HPK on a v1 module. Only a positive result is
// cached; a negative one is transient (tools mid-rebuild) so the next call
// retries.
func ModuleSupportsAwg31() bool {
	if moduleSupportsAwg31Override != nil {
		return *moduleSupportsAwg31Override
	}
	if moduleAwg31Checked {
		return moduleAwg31Supported
	}
	out, err := exec.CommandContext(context.Background(), awgBin("awg"), "version").Output()
	if err != nil {
		return false
	}
	supported := awgToolsAtLeast(string(out), 3, 1)
	if supported {
		moduleAwg31Checked = true
		moduleAwg31Supported = true
	}
	return supported
}

// SetModuleSupportsAwg31 overrides the 3.1 tools-support probe for tests.
func SetModuleSupportsAwg31(supported *bool) {
	moduleSupportsAwg31Override = supported
}

// awg3CapabilityCheck builds the informational diagnostics line for AWG3
// (HeaderProtectionKey) readiness: the kernel module must export the
// header-protection symbol and the awg tools must be v3.0+. A failing line
// does not make the inbound unhealthy (Healthy skips it) — it explains why
// the panel renders configs without HPK on this host. The tools probe goes
// through the prober so tests can replay it; the kernel probe reads
// /proc/kallsyms directly.
func awg3CapabilityCheck(p prober) DiagCheck {
	kernelOK := kernelExportsHeaderProtection()
	toolsOut, err := p.Run("awg", "version")
	toolsOK := err == nil && parseMajorVersion(toolsOut) >= 3
	detail := fmt.Sprintf("kernel HPK symbol: %s; tools: %s", yesNo(kernelOK), oneLine(strings.TrimSpace(toolsOut)))
	if err != nil {
		detail = "kernel HPK symbol: " + yesNo(kernelOK) + "; tools: awg version failed"
	}
	if !kernelOK || !toolsOK {
		detail += " — HPK/device fields are omitted in rendered configs on this host"
	}
	return DiagCheck{awg3SupportCheckName, kernelOK && toolsOK, detail}
}

// awg31CapabilityCheck builds the informational diagnostics line for AWG 3.1
// (RandomTrailers / DisableCookies) readiness: the awg tools must be v3.1+.
// A failing line does not make the inbound unhealthy (Healthy skips it) — it
// only explains why the panel renders configs without those fields here. The
// tools probe goes through the prober so tests can replay it.
func awg31CapabilityCheck(p prober) DiagCheck {
	toolsOut, err := p.Run("awg", "version")
	toolsOK := err == nil && awgToolsAtLeast(toolsOut, 3, 1)
	detail := "tools: " + oneLine(strings.TrimSpace(toolsOut))
	if err != nil {
		detail = "tools: awg version failed"
	}
	if !toolsOK {
		detail += " — RandomTrailers/DisableCookies are omitted in rendered configs on this host"
	}
	return DiagCheck{awg31SupportCheckName, toolsOK, detail}
}

func kernelAvailable() bool {
	if _, err := os.Stat("/sys/module/amneziawg"); err != nil {
		return false
	}
	_, err := exec.LookPath("awg-quick")
	return err == nil
}

func yesNo(b bool) string {
	if b {
		return "present"
	}
	return "absent"
}
