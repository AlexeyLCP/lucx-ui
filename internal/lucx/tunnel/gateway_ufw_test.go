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

func TestGatewayUFW_MovedRowsVanishFromAllow(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, OldPort: 46000},
		{InboundID: 2, Class: ClassPassthrough, OldPort: 9443},
		{InboundID: 3, Class: ClassPassthrough, OldPort: 51820},
	}
	// 1 = selected maskable, 2 = hidden inside the site, 3 = hidden naive.
	// All three have their port withdrawn, so none may appear in the allow
	// list or survive a re-apply (lucx.297).
	moved := map[int]bool{1: true, 2: true, 3: true}
	allows := GatewayUFWAllow(2053, 2096, 22, rows, moved, true)
	for _, s := range allows {
		if strings.HasPrefix(s, "46000/") || strings.HasPrefix(s, "9443/") || strings.HasPrefix(s, "51820/") {
			t.Fatalf("moved row still allowed: %s in %v", s, allows)
		}
	}
	closed := GatewayUFWClose(2053, 2096, 22, rows, moved, true)
	have := map[string]bool{}
	for _, s := range closed {
		have[s] = true
	}
	for _, need := range []string{"46000/tcp", "46000/udp", "9443/tcp", "9443/udp"} {
		if !have[need] {
			t.Fatalf("moved row port not closed: %s in %v", need, closed)
		}
	}
	if have["22/tcp"] || have["443/tcp"] {
		t.Fatalf("closed a port that stays public: %v", closed)
	}
	plain := GatewayUFWAllow(2053, 2096, 22, rows, nil, false)
	have = map[string]bool{}
	for _, s := range plain {
		have[s] = true
	}
	for _, need := range []string{"46000/tcp", "46000/udp", "9443/tcp", "9443/udp", "51820/tcp", "51820/udp"} {
		if !have[need] {
			t.Fatalf("unmoved row must stay allowed: %s in %v", need, plain)
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
	for _, need := range []string{"2053/tcp", "2096/tcp", "2053/udp", "2096/udp", "8443/tcp", "8443/udp", "18443/tcp"} {
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
	oldLinux, oldPath, oldRun, oldShow := ufwOSLinux, ufwLookPath, ufwRun, ufwShowAdded
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwRun, ufwShowAdded = oldLinux, oldPath, oldRun, oldShow })
	ufwOSLinux = true
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	ufwShowAdded = func() (string, error) { return "", exec.ErrNotFound }
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

func TestApplyUFW_ClosesManualRules(t *testing.T) {
	oldLinux, oldPath, oldRun, oldShow := ufwOSLinux, ufwLookPath, ufwRun, ufwShowAdded
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwRun, ufwShowAdded = oldLinux, oldPath, oldRun, oldShow })
	ufwOSLinux = true
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	// Exact spec deletes never match a rule the operator pasted by hand:
	// bare `ufw allow 35347` is one proto-less entry, and a full-syntax rule
	// only matches its own text (verified on ufw 0.36.2).
	ufwShowAdded = func() (string, error) {
		return strings.Join([]string{
			"Added user rules (see 'ufw status' for running firewall):",
			"ufw allow 22/tcp",
			"ufw allow 443/tcp",
			"ufw allow 35347",
			"ufw allow in 2053/tcp",
			"ufw allow from 1.2.3.4 to any port 35387 proto tcp",
		}, "\n") + "\n", nil
	}
	var got []string
	ufwRun = func(args ...string) error {
		got = append(got, strings.Join(args, " "))
		return nil
	}
	if err := ApplyUFW([]string{"22/tcp", "443/tcp"}, []string{"2053/tcp", "2053/udp", "35347/tcp", "35347/udp", "35387/tcp"}); err != nil {
		t.Fatal(err)
	}
	var deletes []string
	for _, cmd := range got {
		if strings.HasPrefix(cmd, "--force delete") {
			deletes = append(deletes, cmd)
		}
	}
	want := strings.Join([]string{
		"--force delete allow 2053/tcp",
		"--force delete allow 2053/udp",
		"--force delete allow 35347/tcp",
		"--force delete allow 35347/udp",
		"--force delete allow 35387/tcp",
		"--force delete allow 35347",
		"--force delete allow in 2053/tcp",
		"--force delete allow from 1.2.3.4 to any port 35387 proto tcp",
	}, ";")
	if strings.Join(deletes, ";") != want {
		t.Fatalf("%v", deletes)
	}
}

func TestApplyUFW_SweepLeavesForeignTargetsAlone(t *testing.T) {
	oldLinux, oldPath, oldRun, oldShow := ufwOSLinux, ufwLookPath, ufwRun, ufwShowAdded
	t.Cleanup(func() { ufwOSLinux, ufwLookPath, ufwRun, ufwShowAdded = oldLinux, oldPath, oldRun, oldShow })
	ufwOSLinux = true
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	ufwShowAdded = func() (string, error) {
		return strings.Join([]string{
			"ufw allow 22/tcp",
			"ufw allow 35340:35350/tcp",
			"ufw allow out 35347/tcp",
			"ufw allow 'Apache'",
			"ufw allow from 5.6.7.8 port 35347 to any port 4321 proto tcp",
			"ufw deny 35347/tcp",
			"ufw route allow 35347/tcp",
			"ufw allow 80/tcp",
		}, "\n") + "\n", nil
	}
	var got []string
	ufwRun = func(args ...string) error {
		got = append(got, strings.Join(args, " "))
		return nil
	}
	if err := ApplyUFW([]string{"22/tcp"}, []string{"35347/tcp", "35347/udp"}); err != nil {
		t.Fatal(err)
	}
	var deletes []string
	for _, cmd := range got {
		if strings.HasPrefix(cmd, "--force delete") {
			deletes = append(deletes, cmd)
		}
	}
	want := "--force delete allow 35347/tcp;--force delete allow 35347/udp"
	if strings.Join(deletes, ";") != want {
		t.Fatalf("%v", deletes)
	}
}

func TestUfwLineOpensPort(t *testing.T) {
	closed := map[int]bool{35347: true}
	yes := []string{
		"ufw allow 35347",
		"ufw allow 35347/tcp",
		" ufw allow 35347/tcp ",
		"ufw allow in 35347/tcp",
		"ufw allow log 35347/tcp",
		"ufw allow from 1.2.3.4 to any port 35347 proto tcp",
		"ufw allow proto tcp from any to any port 35347",
		"ufw allow from 1.2.3.4 port 35000 to any port 35347",
	}
	no := []string{
		"Added user rules (see 'ufw status' for running firewall):",
		"",
		"ufw allow 22/tcp",
		"ufw allow 12345",
		"ufw allow 35347:35350/tcp",
		"ufw allow out 35347/tcp",
		"ufw allow 'Apache'",
		"ufw deny 35347/tcp",
		"ufw route allow 35347/tcp",
		"ufw allow from 5.6.7.8 port 35347 to any port 4321 proto tcp",
	}
	for _, line := range yes {
		if !ufwLineOpensPort(line, closed) {
			t.Fatalf("not matched: %q", line)
		}
	}
	for _, line := range no {
		if ufwLineOpensPort(line, closed) {
			t.Fatalf("matched: %q", line)
		}
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
