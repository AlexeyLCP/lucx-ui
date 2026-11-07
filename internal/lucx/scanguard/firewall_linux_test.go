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

// fakeSG installs a recording exec fake under sgCmd/sgIpsetRestore. argv
// keeps the raw per-token arguments so tests can assert tokenization (a
// whitespace inside one token is how the sweep passed a whole rule spec as
// one iptables match name).
type fakeSG struct {
	calls    []string
	argv     [][]string
	restore  []string
	listBody string
}

// forceBackend pins detectBackend for the duration of the test — the real
// one depends on host binaries.
func forceBackend(t *testing.T, be string) {
	t.Helper()
	orig := detectBackend
	t.Cleanup(func() { detectBackend = orig })
	detectBackend = func() string { return be }
}

func installFakeSG(t *testing.T, listBody string, rulePresent bool) *fakeSG {
	t.Helper()
	f := &fakeSG{listBody: listBody}
	origCmd, origRestore := sgCmd, sgIpsetRestore
	t.Cleanup(func() { sgCmd, sgIpsetRestore = origCmd, origRestore })
	sgCmd = func(name string, args ...string) ([]byte, error) {
		joined := name + " " + strings.Join(args, " ")
		f.calls = append(f.calls, joined)
		f.argv = append(f.argv, append([]string{name}, args...))
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
	forceBackend(t, beIptables)
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

	for _, c := range f.calls {
		if strings.HasPrefix(c, "iptables iptables") || strings.HasPrefix(c, "ip6tables ip6tables") {
			t.Fatalf("exec argv must carry the binary once (was: %q)", c)
		}
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
	forceBackend(t, beIptables)
	f := installFakeSG(t, "", false)
	const v4Dump = "-P INPUT DROP\n" +
		`-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg4 src -m comment --comment "lucx-scanguard" -j DROP` + "\n" +
		"-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT\n"
	const v6Dump = `-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg6 src -m comment --comment lucx-scanguard -j DROP` + "\n"
	sgCmd = func(name string, args ...string) ([]byte, error) {
		joined := name + " " + strings.Join(args, " ")
		f.calls = append(f.calls, joined)
		f.argv = append(f.argv, append([]string{name}, args...))
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
	// Tokenization: every delete must reach iptables as one token per
	// option — a combined spec token reads as a single match name.
	for _, a := range f.argv {
		if len(a) < 4 || a[2] != "-D" {
			continue
		}
		for _, tok := range a[4:] {
			if strings.Contains(tok, " ") {
				t.Fatalf("delete argv must be split into tokens, got: %q", tok)
			}
		}
		if !strings.Contains(strings.Join(a[4:], " "), "--comment lucx-scanguard") {
			t.Fatalf("delete argv must keep the owner comment: %v", a)
		}
	}
}

// The nft backend installs sets+rules in ONE transaction with neither
// ipset nor iptables.
func TestNftBackendApplyAndRemove(t *testing.T) {
	forceBackend(t, beNft)
	f := installFakeSG(t, "", false)
	origNft := sgNftRestore
	t.Cleanup(func() { sgNftRestore = origNft })
	var batch string
	sgNftRestore = func(input []byte) error {
		batch = string(input)
		return nil
	}

	origFetch := httpFetch
	t.Cleanup(func() { httpFetch = origFetch })
	httpFetch = func(url string) ([]byte, error) {
		return []byte("1.2.3.0/24\n5.6.7.0/24\n"), nil
	}

	res, err := Apply(Config{Sources: []string{DefaultSource}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.V4 != 2 || res.V6 != 0 {
		t.Fatalf("apply result = %+v", res)
	}
	if !strings.Contains(batch, "table inet lucx_scanguard {") ||
		!strings.Contains(batch, "elements = { 1.2.3.0/24, 5.6.7.0/24 }") ||
		!strings.Contains(batch, "ct state new ip saddr @lucx-sg4 drop") {
		t.Fatalf("nft batch malformed:\n%s", batch)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "iptables") || strings.Contains(c, "ipset") {
			t.Fatalf("nft backend must not touch ipset/iptables: %q", c)
		}
	}

	// Disable deletes the owned table, nothing else.
	if err := Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	dels := 0
	for _, c := range f.calls {
		if strings.Contains(c, "nft delete table inet lucx_scanguard") {
			dels++
		}
	}
	if dels != 1 {
		t.Fatalf("disable must delete only the owned table, calls: %v", f.calls)
	}
}

func TestNftProbe(t *testing.T) {
	forceBackend(t, beNft)
	orig := sgCmd
	t.Cleanup(func() { sgCmd = orig })
	sgCmd = func(name string, args ...string) ([]byte, error) {
		switch {
		case name != "nft":
			return nil, errors.New("not available")
		case args[0] == "-j":
			return []byte(`{"nftables":[` +
				`{"set":{"family":"inet","name":"lucx-sg4","elem":["203.0.113.0/24"]}},` +
				`{"set":{"family":"inet","name":"lucx-sg6"}}]}`), nil
		case args[0] == "list":
			return []byte("chain input {\n\tct state new ip saddr @lucx-sg4 drop\n}\n"), nil
		}
		t.Fatalf("unexpected nft call %v", args)
		return nil, nil
	}
	st := probeStatus()
	if st.Backend != beNft || !st.IpsetAvailable || st.V4Entries != 1 || st.V6Entries != 0 || !st.RulesV4 || st.RulesV6 {
		t.Fatalf("nft probe = %+v", st)
	}
}

func TestProbeStatus(t *testing.T) {
	forceBackend(t, beIptables)
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
	if !st.IpsetAvailable || st.V4Entries != 7 || !st.RulesV4 || !st.RulesV6 {
		t.Fatalf("probed status = %+v (calls=%d)", st, calls)
	}
}

// IpsetAvailable must mean "the ipset binary runs", NOT "the lucx-sg set
// exists" — otherwise a fresh host with ipset installed gets told to install
// it (lesson lucx.352, tester VPS with ipset 7.19).
func TestProbeStatusWithoutSets(t *testing.T) {
	forceBackend(t, beIptables)
	installFakeSG(t, "", false)
	origCmd := sgCmd
	t.Cleanup(func() { sgCmd = origCmd })
	sgCmd = func(name string, args ...string) ([]byte, error) {
		switch {
		case name == "ipset" && len(args) > 0 && args[0] == "list":
			return nil, errors.New("set does not exist")
		case (name == "iptables" || name == "ip6tables") && len(args) > 2 && args[2] == "-C":
			return nil, errors.New("Bad rule (does a matching rule exist in that chain?)")
		}
		return []byte("ok"), nil
	}
	st := probeStatus()
	if !st.IpsetAvailable {
		t.Fatalf("installed ipset must probe available: %+v", st)
	}
	if st.V4Entries != 0 || st.V6Entries != 0 {
		t.Fatalf("missing sets must report zero entries: %+v", st)
	}
	if st.RulesV4 || st.RulesV6 {
		t.Fatalf("absent rules must report false: %+v", st)
	}
}

// A host without ipset keeps getting the "install the package" hint.
func TestProbeStatusWithoutIpset(t *testing.T) {
	forceBackend(t, beIptables)
	installFakeSG(t, "", false)
	origCmd := sgCmd
	t.Cleanup(func() { sgCmd = origCmd })
	sgCmd = func(name string, args ...string) ([]byte, error) {
		if name == "ipset" && len(args) > 0 && args[0] == "version" {
			return nil, errors.New("exec: not found")
		}
		if name == "ipset" && len(args) > 0 && args[0] == "list" {
			return nil, errors.New("exec: not found")
		}
		return []byte("ok"), nil
	}
	st := probeStatus()
	if st.IpsetAvailable {
		t.Fatalf("missing ipset binary must probe unavailable: %+v", st)
	}
}
