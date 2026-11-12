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
	dump     string
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
			return []byte(f.dump), nil
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

	res, err := Apply(Config{Sources: []string{DefaultSource}, Whitelist: []string{"203.0.113.0/24"}}, nil)
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
	if strings.Contains(strings.Join(ruleIns, " "), " ACCEPT") {
		t.Fatalf("empty keep list must not install accepts: %v", ruleIns)
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

// With wanted keep ports, Apply inserts tunnel-port ACCEPTs ABOVE the DROPs
// (owner-tagged, conntrack-NEW), both transports per port; duplicate ports
// collapse.
func TestApplyInsertsKeepPorts(t *testing.T) {
	forceBackend(t, beIptables)
	f := installFakeSG(t, "", false)

	origFetch := httpFetch
	t.Cleanup(func() { httpFetch = origFetch })
	httpFetch = func(url string) ([]byte, error) {
		return []byte("1.2.3.0/24\n"), nil
	}

	if _, err := Apply(Config{Sources: []string{DefaultSource}}, []int{56000, 46000, 46000}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	var dropIns, keepIns []string
	for _, c := range f.calls {
		switch {
		case strings.Contains(c, "-I INPUT") && strings.Contains(c, "--match-set"):
			dropIns = append(dropIns, c)
		case strings.Contains(c, "-I INPUT") && strings.Contains(c, keepOwner):
			keepIns = append(keepIns, c)
		}
	}
	if len(dropIns) != 2 {
		t.Fatalf("expected one drop per family, got %v", f.calls)
	}
	if len(keepIns) != 8 {
		t.Fatalf("expected 2 ports x 2 transports x 2 families accepts, got %d:\n%v", len(keepIns), keepIns)
	}
	for _, c := range keepIns {
		if !strings.Contains(c, "--ctstate NEW") || !strings.Contains(c, "-j ACCEPT") {
			t.Fatalf("keep accept must be NEW-scoped: %q", c)
		}
		if !strings.Contains(c, "-p tcp") && !strings.Contains(c, "-p udp") {
			t.Fatalf("keep accept must be transport-scoped: %q", c)
		}
	}
	for _, want := range []string{"--dport 46000", "--dport 56000"} {
		if got := strings.Count(strings.Join(keepIns, "|"), want); got != 4 {
			t.Fatalf("expected 4 accepts for %q (tcp+udp x 2 families), got %d:\n%v", want, got, keepIns)
		}
	}
	// Family shape: the first insert of the run is the v4 DROP; every keep
	// insert lands after it (position 1 keeps them ABOVE).
	firstIns := -1
	for i, c := range f.calls {
		if strings.Contains(c, "-I INPUT") {
			firstIns = i
			break
		}
	}
	if firstIns < 0 || !strings.Contains(f.calls[firstIns], "--match-set") {
		t.Fatalf("the first insert must be the family drop, calls: %v", f.calls)
	}
	for _, c := range keepIns {
		idx := -1
		for i, call := range f.calls {
			if call == c {
				idx = i
				break
			}
		}
		if idx < firstIns {
			t.Fatalf("keep accept %q must come after the drop insert", c)
		}
	}
}

// Convergence heals every stale state: keeps below the DROP get re-inserted,
// ports the tunnels no longer use get swept, missing ones get inserted,
// properly placed rules stay untouched.
func TestKeepRulesConverge(t *testing.T) {
	forceBackend(t, beIptables)
	installFakeSG(t, "", true)
	dump := "-P INPUT ACCEPT\n" +
		"-A INPUT -m conntrack --ctstate NEW -p tcp --dport 46000 -m comment --comment \"lucx-scanguard-keep\" -j ACCEPT\n" +
		"-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg4 src -m comment --comment \"lucx-scanguard\" -j DROP\n" +
		"-A INPUT -m conntrack --ctstate NEW -p udp --dport 46000 -m comment --comment \"lucx-scanguard-keep\" -j ACCEPT\n" +
		"-A INPUT -p tcp --dport 9999 -m conntrack --ctstate NEW -m comment --comment \"lucx-scanguard-keep\" -j ACCEPT\n"
	var calls []string
	origCmd := sgCmd
	t.Cleanup(func() { sgCmd = origCmd })
	sgCmd = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "iptables" && len(args) > 0 && args[0] == "-w" && args[2] == "-S" {
			return []byte(dump), nil
		}
		if strings.Contains(calls[len(calls)-1], " -C INPUT") {
			return []byte("ok"), nil // drop rule already present
		}
		return []byte("ok"), nil
	}

	if err := ensureFamilyRules("iptables", setName4, []int{46000, 46000}); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	var dels, ins []string
	for _, c := range calls {
		switch {
		case strings.Contains(c, "-D INPUT"):
			dels = append(dels, c)
		case strings.Contains(c, "-I INPUT"):
			ins = append(ins, c)
		}
	}
	if len(ins) != 1 || !strings.Contains(ins[0], "-I INPUT 1 -p udp --dport 46000") {
		t.Fatalf("expected exactly the misplaced udp/46000 re-insert, got %v", ins)
	}
	if len(dels) != 2 {
		t.Fatalf("expected 2 sweeps (misplaced udp/46000 + stale tcp/9999), got %v", dels)
	}
	joined := strings.Join(dels, "|")
	if !strings.Contains(joined, "--dport 9999") ||
		!strings.Contains(joined, "udp") {
		t.Fatalf("sweeps must target the stale and misplaced rules: %v", dels)
	}
	for _, d := range dels {
		if strings.Contains(d, "tcp") && strings.Contains(d, "--dport 46000") {
			t.Fatalf("the properly placed tcp/46000 must stay untouched: %v", dels)
		}
	}
}

func TestDisableSweepsOwnerRules(t *testing.T) {
	forceBackend(t, beIptables)
	f := installFakeSG(t, "", false)
	const v4Dump = "-P INPUT DROP\n" +
		`-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg4 src -m comment --comment "lucx-scanguard" -j DROP` + "\n" +
		"-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT\n" +
		`-A INPUT -p udp --dport 46000 -m conntrack --ctstate NEW -m comment --comment "lucx-scanguard-keep" -j ACCEPT` + "\n"
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
	if swept != 3 {
		t.Fatalf("expected 3 owner rules swept (2 drops + 1 keep), got %d in %v", swept, f.calls)
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
		owner := strings.Contains(strings.Join(a[4:], " "), "--comment "+firewallOwner) ||
			strings.Contains(strings.Join(a[4:], " "), "--comment "+keepOwner)
		if !owner {
			t.Fatalf("delete argv must keep an owner comment: %v", a)
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

	res, err := Apply(Config{Sources: []string{DefaultSource}}, []int{46000})
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
	drop := strings.Index(batch, "ct state new ip saddr @lucx-sg4 drop")
	udp := strings.Index(batch, "ct state new udp dport 46000 accept")
	tcp := strings.Index(batch, "ct state new tcp dport 46000 accept")
	if udp < 0 || tcp < 0 || udp > drop || tcp > drop {
		t.Fatalf("keep accepts must sit above the drop:\n%s", batch)
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
	chain := "table inet lucx_scanguard {\n\tchain input {\n" +
		"\t\ttype filter hook input priority 0; policy accept;\n" +
		"\t\tct state new udp dport 46000 accept\n" +
		"\t\tct state new tcp dport 46000 accept\n" +
		"\t\tct state new ip saddr @lucx-sg4 counter packets 1 bytes 52 drop\n" +
		"\t}\n}\n"
	sgCmd = func(name string, args ...string) ([]byte, error) {
		switch {
		case name != "nft":
			return nil, errors.New("not available")
		case args[0] == "-j":
			return []byte(`{"nftables":[` +
				`{"set":{"family":"inet","name":"lucx-sg4","elem":["203.0.113.0/24"]}},` +
				`{"set":{"family":"inet","name":"lucx-sg6"}}]}`), nil
		case args[0] == "list":
			return []byte(chain), nil
		}
		t.Fatalf("unexpected nft call %v", args)
		return nil, nil
	}
	st := probeStatus([]int{46000})
	if st.Backend != beNft || !st.IpsetAvailable || st.V4Entries != 1 || st.V6Entries != 0 ||
		!st.RulesV4 || st.RulesV6 || !st.KeepOk {
		t.Fatalf("nft probe = %+v", st)
	}
	if probeStatus([]int{46000, 56000}).KeepOk {
		t.Fatalf("a wanted missing keep port must report unsatisfied: %+v", st)
	}
	if probeStatus(nil).KeepOk {
		t.Fatalf("installed keeps must fail an empty want")
	}
}

func TestProbeStatus(t *testing.T) {
	forceBackend(t, beIptables)
	base := installFakeSG(t, "Name: lucx-sg4\nNumber of entries: 7\n", true)
	const keep4 = `-A INPUT -p udp --dport 46000 -m conntrack --ctstate NEW -m comment --comment "lucx-scanguard-keep" -j ACCEPT` + "\n" +
		`-A INPUT -p tcp --dport 46000 -m conntrack --ctstate NEW -m comment --comment "lucx-scanguard-keep" -j ACCEPT` + "\n"
	const v4Dump = "-P INPUT ACCEPT\n" + keep4 +
		`-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg4 src -m comment --comment "lucx-scanguard" -j DROP` + "\n"
	const v6Dump = keep4 +
		`-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg6 src -m comment --comment lucx-scanguard -j DROP` + "\n"
	sgCmd = func(name string, args ...string) ([]byte, error) {
		if (name == "iptables" || name == "ip6tables") && len(args) > 2 && args[2] == "-S" {
			if name == "iptables" {
				return []byte(v4Dump), nil
			}
			return []byte(v6Dump), nil
		}
		return baseFakeCall(base, name, args)
	}
	st := probeStatus([]int{46000})
	if !st.IpsetAvailable || st.V4Entries != 7 || !st.RulesV4 || !st.RulesV6 || !st.KeepOk {
		t.Fatalf("probed status = %+v", st)
	}
	if probeStatus([]int{46000, 56000}).KeepOk {
		t.Fatalf("a wanted port missing from the dumps must be unsatisfied")
	}
	if probeStatus(nil).KeepOk {
		t.Fatalf("installed keeps must fail an empty want")
	}
}

// baseFakeCall re-runs the installed fake for non-override calls (ipset
// probes etc.).
func baseFakeCall(f *fakeSG, name string, args []string) ([]byte, error) {
	if name == "ipset" && len(args) > 0 && args[0] == "list" {
		return []byte(f.listBody), nil
	}
	return []byte("ok"), nil
}

// IpsetAvailable must mean "the `ipset version` subcommand runs", NOT "the
// `lucx-sg*` set exists" — otherwise a fresh host with ipset installed gets
// told to install it (lesson lucx.352, tester VPS with ipset 7.19).
func TestProbeStatusWithoutSets(t *testing.T) {
	forceBackend(t, beIptables)
	installFakeSG(t, "", false)
	origCmd := sgCmd
	t.Cleanup(func() { sgCmd = origCmd })
	sgCmd = func(name string, args ...string) ([]byte, error) {
		switch {
		case name == "ipset" && len(args) > 0 && args[0] == "list":
			return nil, errors.New("set does not exist")
		case (name == "iptables" || name == "ip6tables") && len(args) > 2 && args[2] == "-S":
			return []byte(""), nil
		}
		return []byte("ok"), nil
	}
	st := probeStatus(nil)
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
	st := probeStatus(nil)
	if st.IpsetAvailable {
		t.Fatalf("missing ipset binary must probe unavailable: %+v", st)
	}
}

// Platform-neutral dump parser tests (keepRef/keepOk live in nft.go).
func TestKeepDumpParsing(t *testing.T) {
	t.Parallel()
	dump := "-P INPUT ACCEPT\n" +
		`-A INPUT -p udp --dport 46000 -m conntrack --ctstate NEW -m comment --comment "lucx-scanguard-keep" -j ACCEPT` + "\n" +
		`-A INPUT -p tcp --dport 46000 -m conntrack --ctstate NEW -m comment --comment "lucx-scanguard-keep" -j ACCEPT` + "\n" +
		`-A INPUT -m conntrack --ctstate NEW -m set --match-set lucx-sg4 src -m comment --comment "lucx-scanguard" -j DROP` + "\n"
	refs, specs, dropIdx := parseKeepDump(dump, setName4)
	if len(refs) != 2 || len(specs) != 2 || dropIdx != 3 {
		t.Fatalf("parse = %v %v drop@%d", refs, specs, dropIdx)
	}
	if refs[0].proto != "udp" || refs[0].port != 46000 {
		t.Fatalf("keep ref = %+v", refs[0])
	}
	if !strings.Contains(specs[0], "--comment lucx-scanguard-keep") ||
		strings.Contains(specs[0], `"`) {
		t.Fatalf("spec must be quote-free for -D: %q", specs[0])
	}
	if !keepOk(dump, setName4, []int{46000}) {
		t.Fatal("matching dump must be keepOk")
	}
	if keepOk(dump, setName4, []int{46000, 56000}) {
		t.Fatal("missing port must fail keepOk")
	}
	if !dropPresent(dump, setName4) {
		t.Fatal("drop must be detected")
	}
	if dropPresent(dump, setName6) {
		t.Fatal("wrong-family drop must not match")
	}
}
