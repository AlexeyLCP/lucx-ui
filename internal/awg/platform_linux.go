//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func renameAwgInterface(oldName, newName string) error {
	return renameAwgInterfaceSeq(ipLinkSet, oldName, newName)
}

func ipLinkSet(args ...string) error {
	cmdArgs := append([]string{"link", "set"}, args...)
	out, err := exec.CommandContext(context.Background(), "ip", cmdArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip link set %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func defaultRouteInterface() string {
	out, err := exec.CommandContext(context.Background(), "ip", "-o", "-4", "route", "show", "default").Output()
	if err != nil {
		out = nil
	}
	return stickyDefaultRoute(parseDefaultRouteInterface(string(out)))
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
		cleanupTproxyConfig(filepath.Join(awgConfigDir, name+".conf"))
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
// Functional probe (lucx.58): sysfs version is 1.0.0 on every tree, so it is not consulted.
// kallsyms wins; when the v3.10 sandbox masks it, the running kernel's DKMS .ko is scanned.
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

// kallsymsPath, kernelReleasePath and dkmsModuleRoot are package vars so tests
// can point them at fixtures. The real procfs and DKMS tree cannot be stubbed.
var (
	kallsymsPath      = "/proc/kallsyms"
	kernelReleasePath = "/proc/sys/kernel/osrelease"
	dkmsModuleRoot    = "/var/lib/dkms/amneziawg"
)

const headerProtectionSymbol = "awg_header_protection_set_key"

// kernelExportsHeaderProtection reports an AWG3 module (header_protection.c symbol).
// Readable kallsyms wins. A masked one (v3.10 sandbox) scans the DKMS .ko, never sysfs 1.0.0.
func kernelExportsHeaderProtection() bool {
	f, err := os.Open(kallsymsPath)
	if err == nil {
		defer f.Close()
		found, readable := kallsymsProbe(f, headerProtectionSymbol)
		if readable {
			return found
		}
	}
	found, ok := dkmsKoExports(headerProtectionSymbol)
	return ok && found
}

// kallsymsProbe scans kallsyms for symbol. readable is false when the stream is empty:
// the v3.10 sandbox opens /proc/kallsyms and returns no lines, which is not "symbol absent".
func kallsymsProbe(r io.Reader, symbol string) (found, readable bool) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		readable = true
		if strings.Contains(sc.Text(), symbol) {
			return true, true
		}
	}
	return false, readable
}

// dkmsKoExports scans the running kernel's DKMS amneziawg.ko for symbol.
// ok is false when no .ko was read. shortcut: uncompressed .ko only, upgrade for .ko.zst.
func dkmsKoExports(symbol string) (found, ok bool) {
	relRaw, err := os.ReadFile(kernelReleasePath)
	if err != nil {
		return false, false
	}
	rel := strings.TrimSpace(string(relRaw))
	if rel == "" {
		return false, false
	}
	saw := false
	_ = filepath.WalkDir(dkmsModuleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "amneziawg.ko" || !pathForKernel(path, rel) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		saw = true
		if bytes.Contains(data, []byte(symbol)) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found, saw
}

// pathForKernel matches a DKMS directory named exactly the running release
// (or "<release>-<arch>"). A bare substring would treat 5.15.0-1 as 5.15.0-100.
func pathForKernel(path, rel string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		if seg == rel || strings.HasPrefix(seg, rel+"-") {
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

// moduleVersionPath is a package var so tests can point it at a fixture
// file — the real sysfs file cannot be downgraded on a live module.
var moduleVersionPath = "/sys/module/amneziawg/version"

// moduleVersionAtLeast checks the LOADED module, not the awg tools — the two
// upgrade independently, so a stale module still rejects RandomTrailers/DisableCookies.
func moduleVersionAtLeast(wantMajor, wantMinor int) bool {
	data, err := os.ReadFile(moduleVersionPath)
	if err != nil {
		return false
	}
	major, minor := parseAwgToolsVersion("v" + strings.TrimSpace(string(data)))
	if major < 0 {
		return false
	}
	if major != wantMajor {
		return major > wantMajor
	}
	return minor >= wantMinor
}

// ModuleSupportsAwg31 needs module AND tools at v3.1+: a stale module accepts
// the .conf line without the semantics; older tools reject it outright ("Line unrecognized").
func ModuleSupportsAwg31() bool {
	if moduleSupportsAwg31Override != nil {
		return *moduleSupportsAwg31Override
	}
	if moduleAwg31Checked {
		return moduleAwg31Supported
	}
	if !moduleVersionAtLeast(3, 1) {
		return false
	}
	out, err := exec.CommandContext(context.Background(), awgBin("awg"), "version").Output()
	if err != nil {
		return false
	}
	supported := awgToolsAtLeast(string(out), 3, 1)
	// Cache only success — a miss may be a transient mid-upgrade race, so
	// the next call retries instead of latching a false result.
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

// awg31CapabilityCheck reports informational readiness only — a failing line
// does not mark the inbound unhealthy (Healthy skips it); see ModuleSupportsAwg31.
func awg31CapabilityCheck(p prober) DiagCheck {
	moduleOK := moduleVersionAtLeast(3, 1)
	toolsOut, err := p.Run("awg", "version")
	toolsOK := err == nil && awgToolsAtLeast(toolsOut, 3, 1)
	detail := fmt.Sprintf("module: %s; tools: %s", yesNo(moduleOK), oneLine(strings.TrimSpace(toolsOut)))
	if err != nil {
		detail = "module: " + yesNo(moduleOK) + "; tools: awg version failed"
	}
	supported := moduleOK && toolsOK
	if !supported {
		detail += " — RandomTrailers/DisableCookies are omitted in rendered configs on this host"
	}
	return DiagCheck{awg31SupportCheckName, supported, detail}
}

func kernelAvailable() bool {
	if _, err := os.Stat("/sys/module/amneziawg"); err != nil {
		return false
	}
	// awgBin already LookPaths then /usr/bin|/usr/local/bin|sbin. A
	// LookPath-only probe missed tools on a thin systemd PATH and, with the
	// old sync.Once cache, locked the process into userspace forever.
	return awgBin("awg-quick") != "awg-quick"
}

func yesNo(b bool) string {
	if b {
		return "present"
	}
	return "absent"
}
