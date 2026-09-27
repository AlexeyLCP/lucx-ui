//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTproxyNetns(t *testing.T) {
	if os.Getenv("LUCX_TPROXY_NETNS_TEST") != "1" {
		t.Skip("opt-in: root + unshare + iproute2 + iptables + ip6tables")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("unshare", "--net", "--", exe, "-test.run=^TestTproxyNetnsChild$", "-test.v")
	cmd.Env = append(os.Environ(), "LUCX_TPROXY_NETNS_CHILD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated network test: %v\n%s", err, out)
	}
}

func TestTproxyNetnsChild(t *testing.T) {
	if os.Getenv("LUCX_TPROXY_NETNS_CHILD") != "1" {
		t.Skip("runs only in a disposable network namespace")
	}
	run := func(name string, args ...string) string {
		t.Helper()
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	run("ip", "link", "set", "lo", "up")
	run("ip", "link", "add", "awg7", "type", "dummy")
	run("ip", "link", "set", "awg7", "up")
	run("ip", "addr", "add", "10.8.0.1/24", "dev", "awg7")
	i := tproxyFixture()
	up, down := tproxyPostUpPostDown(i)
	apply := func() {
		t.Helper()
		if err := runTproxyScript(up); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	apply()
	selector := []string{"-4", "rule", "show", "iif", "awg7", "fwmark", "0x40000007/0xffffffff", "lookup", "40007"}
	if strings.Count(strings.TrimSpace(run("ip", selector...)), "lookup") != 1 {
		t.Fatal("policy-rule duplication")
	}
	if strings.Count(run("iptables", "-t", "mangle", "-S", "PREROUTING"), "-j LXAT7") != 1 {
		t.Fatal("jump duplication")
	}
	run("ip", "-4", "route", "del", "local", "default", "dev", "lo", "table", "40007", "proto", "242")
	apply()
	if !strings.Contains(run("ip", "-4", "route", "show", "table", "40007"), "local default") {
		t.Fatal("route not repaired independently")
	}
	run("ip", "-4", "rule", "del", "pref", "10000", "iif", "awg7", "fwmark", "0x40000007/0xffffffff", "lookup", "40007")
	run("iptables", "-t", "mangle", "-F", "LXAT7")
	apply()
	if strings.TrimSpace(run("ip", selector...)) == "" {
		t.Fatal("policy rule not repaired")
	}
	if strings.Count(run("iptables", "-t", "mangle", "-S", "LXAT7"), "-j TPROXY") != 2 {
		t.Fatal("TCP/UDP interception not repaired")
	}
	run("iptables", "-A", "FORWARD", "-i", "awg7", "-j", "DROP", "-m", "comment", "--comment", "foreign-test")
	run("ip", "link", "del", "awg7")
	if err := runTproxyScript(down); err != nil {
		t.Fatal(err)
	}
	if err := runTproxyScript(down); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(run("ip", selector...)) != "" {
		t.Fatal("policy rule survived cleanup")
	}
	if !strings.Contains(run("iptables", "-S", "FORWARD"), "foreign-test") {
		t.Fatal("cleanup removed foreign firewall rule")
	}
	if strings.Contains(run("iptables", "-t", "mangle", "-S"), "LXAT7") {
		t.Fatal("chain survived cleanup")
	}
	run("ip", "link", "add", "awg7", "type", "dummy")
	run("ip", "link", "set", "awg7", "up")
	run("ip", "route", "add", "blackhole", "default", "table", "40007")
	if err := runTproxyScript(up); err == nil {
		t.Fatal("foreign routing table was accepted")
	}
	if err := runTproxyScript(down); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(run("ip", "route", "show", "table", "40007"), "blackhole") {
		t.Fatal("foreign route destroyed")
	}
}
