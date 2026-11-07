// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package scanguard

import (
	"errors"
	"strings"
	"testing"
)

// fakeSG installs a recording exec fake under sgCmd/sgIpsetRestore.
type fakeSG struct {
	calls    []string
	restore  []string
	listBody string
}

func installFakeSG(t *testing.T, listBody string, rulePresent bool) *fakeSG {
	t.Helper()
	f := &fakeSG{listBody: listBody}
	origCmd, origRestore := sgCmd, sgIpsetRestore
	t.Cleanup(func() { sgCmd, sgIpsetRestore = origCmd, origRestore })
	sgCmd = func(name string, args ...string) ([]byte, error) {
		joined := name + " " + strings.Join(args, " ")
		f.calls = append(f.calls, joined)
		switch {
		case name == "ipset" && args[0] == "list":
			if args[1] != setName4 && args[1] != setName6 {
				return nil, errors.New("set does not exist")
			}
			return []byte(f.listBody), nil
		case strings.Contains(joined, " -S "):
			return []byte(f.listBody), nil
		case strings.Contains(joined, " -C INPUT"):
			if rulePresent {
				return []byte("ok"), nil
			}
			return nil, errors.New("rule absent")
		}
		return []byte("ok"), nil
	}
	sgIpsetRestore = func(input []byte) error {
		f.restore = append(f.restore, string(input))
		return nil
	}
	return f
}

func TestApplySwapsSetsAndEnsuresRules(t *testing.T) {
	f := installFakeSG(t, "Name: lucx-sg4\nNumber of entries: 42\n", false)

	origFetch := httpFetch
	t.Cleanup(func() { httpFetch = origFetch })
	httpFetch = func(url string) ([]byte, error) {
		return []byte("1.2.3.0/24\n2001:db8::/32\n"), nil
	}

	res, err := Apply(Config{Sources: []string{DefaultSource}, Whitelist: []string{"203.0.113.0/24"}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.V4 != 1 || res.V6 != 1 || res.Skipped != 0 {
		t.Fatalf("apply result = %+v", res)
	}

	var ruleIns []string
	for _, c := range f.calls {
		if strings.Contains(c, "-I INPUT") {
			ruleIns = append(ruleIns, c)
		}
	}
	if len(ruleIns) != 2 {
		t.Fatalf("expected insertions for both families, got %v", f.calls)
	}
	v4, v6 := strings.Join(ruleIns, "|"), strings.Join(ruleIns, "|")
	if !strings.Contains(v4, "--match-set lucx-sg4 src") || !strings.Contains(v6, "--match-set lucx-sg6 src") {
		t.Fatalf("rules must target the sets: %v", ruleIns)
	}
	if !strings.Contains(v4, "--ctstate NEW") || !strings.Contains(v4, "--comment "+firewallOwner) {
		t.Fatalf("rules must be NEW-only owner-tagged: %v", ruleIns)
	}
	if len(f.restore) != 2 {
		t.Fatalf("expected one restore per family, got %d", len(f.restore))
	}
	if got := f.restore[0]; !strings.Contains(got, "create lucx-sg4 hash:net family inet") ||
		!strings.Contains(got, "add lucx-sg4-tmp 1.2.3.0/24") ||
		!strings.Contains(got, "swap lucx-sg4 lucx-sg4-tmp") ||
		!strings.HasSuffix(got, "destroy lucx-sg4-tmp\n") {
		t.Fatalf("v4 restore input malformed:\n%s", got)
	}
	if got := f.restore[1]; !strings.Contains(got, "family inet6") || !strings.Contains(got, "swap lucx-sg6 lucx-sg6-tmp") {
		t.Fatalf("v6 restore input malformed:\n%s", got)
	}
}

func TestDisableSweepsOwnerRules(t *testing.T) {
	f := installFakeSG(t, "", false)
	const v4Dump = "-P INPUT DROP\n" +
		`-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg4 src -m comment --comment "lucx-scanguard" -j DROP` + "\n" +
		"-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT\n"
	const v6Dump = `-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg6 src -m comment --comment lucx-scanguard -j DROP` + "\n"
	sgCmd = func(name string, args ...string) ([]byte, error) {
		joined := name + " " + strings.Join(args, " ")
		f.calls = append(f.calls, joined)
		switch {
		case strings.Contains(joined, " -S "):
			if name == "iptables" {
				return []byte(v4Dump), nil
			}
			return []byte(v6Dump), nil
		case name == "ipset" && args[0] == "list":
			return nil, errors.New("set does not exist")
		}
		return []byte("ok"), nil
	}

	if err := Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	swept, destroyed := 0, 0
	for _, c := range f.calls {
		switch {
		case strings.Contains(c, "-D INPUT"):
			swept++
			if strings.Contains(c, "RELATED,ESTABLISHED") {
				t.Fatalf("foreign rule must not be swept: %q", c)
			}
		case strings.Contains(c, "destroy"):
			destroyed++
		}
	}
	if swept != 2 {
		t.Fatalf("expected 2 owner rules swept, got %d in %v", swept, f.calls)
	}
	if destroyed < 2 {
		t.Fatalf("expected both sets destroyed, got %v", f.calls)
	}
}

func TestProbeStatus(t *testing.T) {
	installFakeSG(t, "Name: lucx-sg4\nNumber of entries: 7\n", true)
	origCmd := sgCmd
	calls := 0
	sgCmd = func(name string, args ...string) ([]byte, error) {
		calls++
		if len(args) > 0 && args[0] == "-S" {
			return []byte(""), nil
		}
		return origCmd(name, args...)
	}
	t.Cleanup(func() { sgCmd = origCmd })
	st := probeStatus()
	if !st.IpsetAvailable || st.V4Entries != 7 || !st.RulesV4 {
		t.Fatalf("probed status = %+v (calls=%d)", st, calls)
	}
}
