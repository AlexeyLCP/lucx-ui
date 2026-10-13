//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsInboundAwgInterface locks in the rule that the inbound orphan sweep
// matches only inbound AWG interfaces (awg1, awg2, ...) and never the
// outbound namespace (awgo-1, awgo-42). Regressing this caused the inbound
// sweep to ip-link-del the operator's outbound tunnels on every panel
// restart — see the gate-blocking review finding for Task 9.
func TestIsInboundAwgInterface(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"awg1", true},
		{"awg2", true},
		{"awg0", true},
		{"awg42", true},
		{"awg", false},    // no digit suffix — not a valid inbound interface name
		{"awgo-1", false}, // outbound namespace — must NOT be swept
		{"awgo-42", false},
		{"awgo-", false},
		{"awgo", false},
		{"awgsomething", false}, // non-digit after awg
		{"eth0", false},
		{"wg0", false},
		{"", false},
		{"tun0", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := isInboundAwgInterface(c.name); got != c.want {
				t.Fatalf("isInboundAwgInterface(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

// TestParseMajorVersion locks in the awg-tools banner parsing that gates
// HeaderProtectionKey emission. The banner is "amneziawg-tools v3.0.20260730
// - https://amnezia.org"; src/version.h guarantees a v-prefixed floor on
// every build, so only a truly garbled output may yield -1.
func TestParseMajorVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"v3 banner", "amneziawg-tools v3.0.20260730 - https://amnezia.org\n", 3},
		{"v1 banner", "amneziawg-tools v1.0.20260618 - https://amnezia.org\n", 1},
		{"describe past tag", "amneziawg-tools v3.0.20260731-4-gabcdef0 - https://amnezia.org\n", 3},
		{"double digit major", "amneziawg-tools v12.1.0 - https://amnezia.org\n", 12},
		{"empty version", "amneziawg-tools v - https://amnezia.org\n", -1},
		{"no version token", "amneziawg-tools - https://amnezia.org\n", -1},
		{"empty output", "", -1},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := parseMajorVersion(c.in); got != c.want {
				t.Fatalf("parseMajorVersion(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestKallsymsProbe verifies the AWG3 kernel-module scan against the real
// /proc/kallsyms line format. The symbol exists only in builds with
// header_protection.c (upstream tag v3.0.20260730+). readable distinguishes a
// masked kallsyms (systemd ProtectKernelTunables hands the panel an empty
// file) from a genuinely symbol-less one.
func TestKallsymsProbe(t *testing.T) {
	awg3Kallsyms := "ffffffffc05a8000 t awg_allowedips_insert	[amneziawg]\n" +
		"ffffffffc05a8e10 T awg_header_protection_set_key	[amneziawg]\n" +
		"ffffffffc05a8e40 T awg_header_protection_get_key	[amneziawg]\n"
	preAwg3Kallsyms := "ffffffffc05a8000 t awg_allowedips_insert	[amneziawg]\n" +
		"ffffffffc05a8200 T awg_noise_handshake_init	[amneziawg]\n"
	cases := []struct {
		name         string
		input        string
		symbol       string
		wantFound    bool
		wantReadable bool
	}{
		{"awg3 module exports the symbol", awg3Kallsyms, "awg_header_protection_set_key", true, true},
		{"pre-awg3 module lacks it", preAwg3Kallsyms, "awg_header_protection_set_key", false, true},
		{"sandbox masked kallsyms is empty and unreadable", "", "awg_header_protection_set_key", false, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			found, readable := kallsymsProbe(strings.NewReader(c.input), c.symbol)
			if found != c.wantFound || readable != c.wantReadable {
				t.Fatalf("kallsymsProbe(...) = (%v, %v), want (%v, %v)", found, readable, c.wantFound, c.wantReadable)
			}
		})
	}
}

// TestModuleVersionAtLeast locks in Д12: AWG 3.1 gates on the LOADED module's
// own version, not the awg tools' — a 3.1-tools host can still carry a stale 3.0 module.
func TestModuleVersionAtLeast(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		missing   bool
		wantMajor int
		wantMinor int
		want      bool
	}{
		{name: "3.1 module satisfies want 3.1", content: "3.1.20260812\n", wantMajor: 3, wantMinor: 1, want: true},
		{name: "3.1 module satisfies want 3.0", content: "3.1.20260812\n", wantMajor: 3, wantMinor: 0, want: true},
		{name: "3.0 module fails want 3.1", content: "3.0.20260730\n", wantMajor: 3, wantMinor: 1, want: false},
		{name: "3.0 module satisfies want 3.0", content: "3.0.20260730\n", wantMajor: 3, wantMinor: 0, want: true},
		{name: "pre-AWG3 module fails want 3.1", content: "1.0.0\n", wantMajor: 3, wantMinor: 1, want: false},
		{name: "pre-AWG3 module fails want 3.0", content: "1.0.0\n", wantMajor: 3, wantMinor: 0, want: false},
		{name: "empty file fails", content: "", wantMajor: 3, wantMinor: 1, want: false},
		{name: "garbage content fails", content: "not-a-version\n", wantMajor: 3, wantMinor: 1, want: false},
		{name: "missing file fails", missing: true, wantMajor: 3, wantMinor: 1, want: false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			orig := moduleVersionPath
			t.Cleanup(func() { moduleVersionPath = orig })
			if c.missing {
				moduleVersionPath = filepath.Join(t.TempDir(), "does-not-exist")
			} else {
				path := filepath.Join(t.TempDir(), "version")
				if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				moduleVersionPath = path
			}
			if got := moduleVersionAtLeast(c.wantMajor, c.wantMinor); got != c.want {
				t.Fatalf("moduleVersionAtLeast(%d, %d) with content %q = %v, want %v", c.wantMajor, c.wantMinor, c.content, got, c.want)
			}
		})
	}
}

// TestModuleSupportsAwg31_ToolsAheadOfModule reproduces Д12: a tools-only
// check let 3.1 tools over a stale 3.0 module render fields awg-quick then rolled back.
func TestModuleSupportsAwg31_ToolsAheadOfModule(t *testing.T) {
	origPath := moduleVersionPath
	origChecked, origSupported := moduleAwg31Checked, moduleAwg31Supported
	t.Cleanup(func() {
		moduleVersionPath = origPath
		moduleAwg31Checked, moduleAwg31Supported = origChecked, origSupported
	})
	// Reset the cache: an earlier green test's cached "supported" would
	// otherwise mask this regression (only positives are cached).
	moduleAwg31Checked, moduleAwg31Supported = false, false

	dir := t.TempDir()
	fakeAwg := filepath.Join(dir, "awg")
	script := "#!/bin/sh\necho 'amneziawg-tools v3.1.20260812 - https://amnezia.org'\n"
	if err := os.WriteFile(fakeAwg, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)

	versionPath := filepath.Join(dir, "version")
	if err := os.WriteFile(versionPath, []byte("3.0.20260730\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	moduleVersionPath = versionPath

	if got := ModuleSupportsAwg31(); got {
		t.Fatalf("ModuleSupportsAwg31() = true with mocked 3.1 tools + a 3.0 module, want false — 3.1 tools alone must not pass the gate")
	}
}

// TestKernelExportsHeaderProtection_SandboxFallback: masked kallsyms must read the
// running kernel's DKMS .ko. Sysfs stays 1.0.0 (lucx.58) and must not decide.
func TestKernelExportsHeaderProtection_SandboxFallback(t *testing.T) {
	const kernel = "6.8.0-146-generic"
	awg3Kallsyms := "ffffffffc05a8e10 T awg_header_protection_set_key\t[amneziawg]\n"
	plainKallsyms := "0000000000000000 T awg_noise_handshake_init\t[amneziawg]\n"
	writeKo := func(root, rel string, withSymbol bool) {
		t.Helper()
		dir := filepath.Join(root, "3c38e168beb7", rel, "x86_64", "module")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		body := []byte("awg_noise_handshake_init\x00")
		if withSymbol {
			body = append(body, []byte("awg_header_protection_set_key\x00")...)
		}
		if err := os.WriteFile(filepath.Join(dir, "amneziawg.ko"), body, 0o644); err != nil {
			t.Fatalf("WriteFile ko: %v", err)
		}
	}
	cases := []struct {
		name            string
		kallsyms        string
		kallsymsMissing bool
		koKernel        string
		koSymbol        bool
		noKo            bool
		want            bool
	}{
		{name: "masked kallsyms + ko with symbol", kallsyms: "", koKernel: kernel, koSymbol: true, want: true},
		{name: "masked kallsyms + v1 ko", kallsyms: "", koKernel: kernel, want: false},
		{name: "masked kallsyms + ko for another kernel", kallsyms: "", koKernel: "5.15.0-100-generic", koSymbol: true, want: false},
		{name: "masked kallsyms + no ko", kallsyms: "", noKo: true, want: false},
		{name: "kallsyms file absent + ko with symbol", kallsymsMissing: true, koKernel: kernel, koSymbol: true, want: true},
		{name: "readable kallsyms with symbol, v1 ko ignored", kallsyms: awg3Kallsyms, koKernel: kernel, want: true},
		{name: "readable kallsyms without symbol, awg3 ko must not rescue", kallsyms: plainKallsyms, koKernel: kernel, koSymbol: true, want: false},
	}
	origKallsyms := kallsymsPath
	origRelease := kernelReleasePath
	origDKMS := dkmsModuleRoot
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Cleanup(func() {
				kallsymsPath = origKallsyms
				kernelReleasePath = origRelease
				dkmsModuleRoot = origDKMS
			})
			root := t.TempDir()
			relPath := filepath.Join(root, "osrelease")
			if err := os.WriteFile(relPath, []byte(kernel+"\n"), 0o644); err != nil {
				t.Fatalf("WriteFile osrelease: %v", err)
			}
			kernelReleasePath = relPath
			dkmsModuleRoot = filepath.Join(root, "dkms")
			if !c.noKo {
				writeKo(dkmsModuleRoot, c.koKernel, c.koSymbol)
			}
			if c.kallsymsMissing {
				kallsymsPath = filepath.Join(root, "kallsyms-absent")
			} else {
				kallsymsPath = filepath.Join(root, "kallsyms")
				if err := os.WriteFile(kallsymsPath, []byte(c.kallsyms), 0o644); err != nil {
					t.Fatalf("WriteFile kallsyms: %v", err)
				}
			}
			if got := kernelExportsHeaderProtection(); got != c.want {
				t.Fatalf("kernelExportsHeaderProtection() = %v, want %v", got, c.want)
			}
		})
	}
}
