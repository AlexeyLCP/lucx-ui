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

func TestGatewayUFWClose_DropsHiddenKeepsPublic(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, OldPort: 8443, NewPort: 18443},
		{InboundID: 2, Class: ClassSkip, OldPort: 51820},
		{InboundID: 3, Class: ClassPassthrough, OldPort: 9443},
	}
	got := GatewayUFWClose(2053, 2096, 22, rows, map[int]bool{1: true}, true)
	have := map[string]bool{}
	for _, s := range got {
		have[s] = true
	}
	for _, need := range []string{"2053/tcp", "2096/tcp", "8443/tcp", "8443/udp", "18443/tcp"} {
		if !have[need] {
			t.Fatalf("missing %s in %v", need, got)
		}
	}
	for _, banned := range []string{"22/tcp", "80/tcp", "443/tcp", "51820/udp", "9443/tcp"} {
		if have[banned] {
			t.Fatalf("closed a port that stays public: %s in %v", banned, got)
		}
	}
	shown := GatewayUFWClose(2053, 2096, 22, rows, map[int]bool{1: true}, false)
	for _, s := range shown {
		if s == "2053/tcp" || s == "2096/tcp" {
			t.Fatalf("panel stays open when not hidden: %v", shown)
		}
	}
}

func TestGatewayUFWRestore_PanelAndMovedPorts(t *testing.T) {
	got := GatewayUFWRestore(2053, 2096, []GatewaySnapshotRow{{Port: 8443}, {Port: 443}}, true)
	have := map[string]bool{}
	for _, s := range got {
		have[s] = true
	}
	for _, need := range []string{"2053/tcp", "2096/tcp", "8443/tcp", "8443/udp", "443/tcp"} {
		if !have[need] {
			t.Fatalf("missing %s in %v", need, got)
		}
	}
	plain := GatewayUFWRestore(2053, 2096, []GatewaySnapshotRow{{Port: 8443}}, false)
	for _, s := range plain {
		if s == "2053/tcp" {
			t.Fatalf("panel restored while it was never hidden: %v", plain)
		}
	}
}

func TestGatewayUFW_SSH22StaysTCPOnly(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, OldPort: 22, NewPort: 18022},
		{InboundID: 2, Class: ClassPassthrough, OldPort: 22},
	}
	allows := GatewayUFWAllow(2053, 2096, 22, rows, map[int]bool{1: true}, true)
	var tcp, udp int
	for _, s := range allows {
		if s == "22/tcp" {
			tcp++
		}
		if s == "22/udp" {
			udp++
		}
	}
	if tcp != 1 || udp != 0 {
		t.Fatalf("22 must be one tcp allow, got %v", allows)
	}
	closed := GatewayUFWClose(2053, 2096, 22, rows, map[int]bool{1: true}, true)
	for _, s := range closed {
		if s == "22/tcp" || s == "22/udp" {
			t.Fatalf("close touches ssh: %v", closed)
		}
	}
	restored := GatewayUFWRestore(2053, 2096, []GatewaySnapshotRow{{Port: 22}, {Port: 8443}}, true)
	for _, s := range restored {
		if s == "22/tcp" || s == "22/udp" {
			t.Fatalf("revert rewrites ssh: %v", restored)
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
	if err := ApplyUFW([]string{"22/tcp", "443/tcp"}, []string{"22/tcp", "22/udp", "8443/tcp"}); err != nil {
		t.Fatal(err)
	}
	want := "--force delete allow 8443/tcp;allow 22/tcp;allow 443/tcp;default deny incoming;--force enable"
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
	if err := RevertUFW(true, nil); err != nil || len(got) != 0 {
		t.Fatalf("leave active ufw alone: %v %q", err, got)
	}
	if err := RevertUFW(true, []string{"2053/tcp"}); err != nil || len(got) != 1 || got[0] != "allow 2053/tcp" {
		t.Fatalf("restore panel port on an already-active ufw: %v %q", err, got)
	}
	got = nil
	if err := RevertUFW(false, []string{"2053/tcp"}); err != nil || len(got) != 1 || got[0] != "disable" {
		t.Fatalf("disable our enable: %v %q", err, got)
	}
}

func TestAllowUFW_Port22StaysTCPOnly(t *testing.T) {
	oldLinux, oldPath, oldRun := ufwOSLinux, ufwLookPath, ufwRun
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwRun = oldLinux, oldPath, oldRun })
	ufwOSLinux = true
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	var got []string
	ufwRun = func(args ...string) error {
		got = append(got, strings.Join(args, " "))
		return nil
	}
	if err := AllowUFW(22); err != nil {
		t.Fatal(err)
	}
	if err := AllowUFW(8443); err != nil {
		t.Fatal(err)
	}
	if err := RevertUFW(true, []string{"22/udp", "22/tcp", "2053/tcp"}); err != nil {
		t.Fatal(err)
	}
	want := "allow 22/tcp;allow 8443/tcp;allow 8443/udp;allow 22/tcp;allow 2053/tcp"
	if strings.Join(got, ";") != want {
		t.Fatalf("%q", got)
	}
	got = nil
	if err := ApplyUFW([]string{"22/udp", "22/tcp", "443/tcp"}, []string{"22/tcp", "22/udp"}); err != nil {
		t.Fatal(err)
	}
	want = "allow 22/tcp;allow 443/tcp;default deny incoming;--force enable"
	if strings.Join(got, ";") != want {
		t.Fatalf("%q", got)
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
	if err := ApplyUFW([]string{"22/tcp"}, nil); err != nil {
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
	if err := ApplyUFW([]string{"22/tcp"}, nil); err == nil {
		t.Fatal("expected error without ufw and apt-get")
	}
}
