// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os/exec"
	"strings"
	"testing"
)

func TestParseSSHDPort(t *testing.T) {
	if got := parseSSHDPort("# Port 2222\nPort 2200\n"); got != 2200 {
		t.Fatalf("%d", got)
	}
	if got := parseSSHDPort(""); got != 22 {
		t.Fatalf("%d", got)
	}
}

func TestGatewayUFWAllow(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, OldPort: 443},
		{InboundID: 2, Class: ClassSkip, OldPort: 51820},
		{InboundID: 3, Class: ClassPassthrough, OldPort: 8443},
	}
	got := GatewayUFWAllow(2053, 2096, 22, rows, map[int]bool{1: true}, false)
	have := map[string]bool{}
	for _, s := range got {
		have[s] = true
	}
	for _, need := range []string{"22/tcp", "80/tcp", "443/tcp", "2053/tcp", "2096/tcp", "51820/udp", "8443/tcp"} {
		if !have[need] {
			t.Fatalf("missing %s in %v", need, got)
		}
	}
	if have["443/udp"] {
		t.Fatalf("muxed 443 should not add udp: %v", got)
	}
	hidden := GatewayUFWAllow(2053, 2096, 22, rows, map[int]bool{1: true}, true)
	for _, s := range hidden {
		if s == "2053/tcp" || s == "2096/tcp" {
			t.Fatalf("hide panel still allows %s in %v", s, hidden)
		}
	}
}

func TestApplyUFW_RecordsCommands(t *testing.T) {
	oldLinux, oldPath, oldRun := ufwOSLinux, ufwLookPath, ufwRun
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwRun = oldLinux, oldPath, oldRun })
	ufwOSLinux = true
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	var got []string
	ufwRun = func(args ...string) error {
		got = append(got, strings.Join(args, " "))
		return nil
	}
	if err := ApplyUFW([]string{"22/tcp", "443/tcp"}); err != nil {
		t.Fatal(err)
	}
	want := "allow 22/tcp;allow 443/tcp;default deny incoming;--force enable"
	if strings.Join(got, ";") != want {
		t.Fatalf("%q", got)
	}
}

func TestRevertUFW_DisablesOnlyIfWeEnabled(t *testing.T) {
	oldLinux, oldPath, oldRun := ufwOSLinux, ufwLookPath, ufwRun
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwRun = oldLinux, oldPath, oldRun })
	ufwOSLinux = true
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	var got []string
	ufwRun = func(args ...string) error {
		got = append(got, strings.Join(args, " "))
		return nil
	}
	if err := RevertUFW(true); err != nil || len(got) != 0 {
		t.Fatalf("leave active ufw alone: %v %q", err, got)
	}
	if err := RevertUFW(false); err != nil || len(got) != 1 || got[0] != "disable" {
		t.Fatalf("disable our enable: %v %q", err, got)
	}
}

func TestApplyUFW_InstallsWhenMissing(t *testing.T) {
	oldLinux, oldPath, oldRun, oldInstall := ufwOSLinux, ufwLookPath, ufwRun, ufwInstall
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwRun, ufwInstall = oldLinux, oldPath, oldRun, oldInstall })
	ufwOSLinux = true
	ufwLookPath = func(name string) (string, error) {
		if name == "apt-get" {
			return "/usr/bin/apt-get", nil
		}
		return "", exec.ErrNotFound
	}
	installed := false
	ufwInstall = func() error { installed = true; return nil }
	var got []string
	ufwRun = func(args ...string) error {
		got = append(got, strings.Join(args, " "))
		return nil
	}
	if err := ApplyUFW([]string{"22/tcp"}); err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Fatal("ufw was not installed on apply")
	}
	want := "allow 22/tcp;default deny incoming;--force enable"
	if strings.Join(got, ";") != want {
		t.Fatalf("%q", got)
	}
}

func TestApplyUFW_MissingUFWAndApt(t *testing.T) {
	oldLinux, oldPath, oldInstall := ufwOSLinux, ufwLookPath, ufwInstall
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwInstall = oldLinux, oldPath, oldInstall })
	ufwOSLinux = true
	ufwLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	ufwInstall = func() error { t.Fatal("install must not run without apt-get"); return nil }
	if err := ApplyUFW([]string{"22/tcp"}); err == nil {
		t.Fatal("expected error without ufw and apt-get")
	}
}
