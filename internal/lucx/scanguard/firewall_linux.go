// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package scanguard

import (
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	tmpSuffix  = "-tmp"
	setMaxelem = 65536

	beIptables = "iptables"
	beNft      = "nft"
)

// detectBackend picks the firewall plumbing by binary presence: the
// battle-tested ipset+iptables path first (it also covers the iptables-nft
// compat layer), native nftables when the compat tooling is absent — the nft
// path needs neither ipset nor iptables. Empty = nothing usable on the host.
// A var so tests can force the backend.
var detectBackend = func() string {
	hasBin := func(b string) bool {
		_, err := exec.LookPath(b)
		return err == nil
	}
	switch {
	case hasBin("iptables") && hasBin("ip6tables"):
		return beIptables
	case hasBin("nft"):
		return beNft
	case hasBin("iptables"):
		return beIptables
	}
	return ""
}

// sgCmd and sgIpsetRestore are vars so tests can fake the exec layer.
// Failures carry the tool's first stderr line — a bare "exit status 2" is
// undiagnosable in the panel toast.
var sgCmd = func(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil && len(out) > 0 {
		err = fmt.Errorf("%w: %s", err, firstErrorLine(out))
	}
	return out, err
}

var sgIpsetRestore = func(input []byte) error {
	cmd := exec.Command("ipset", "restore", "-exist")
	cmd.Stdin = strings.NewReader(string(input))
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) > 0 {
		err = fmt.Errorf("%w: %s", err, firstErrorLine(out))
	}
	return err
}

var sgNftRestore = func(input []byte) error {
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(string(input))
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) > 0 {
		err = fmt.Errorf("%w: %s", err, firstErrorLine(out))
	}
	return err
}

// nftApply installs the whole guard (sets + hooked chain) in one atomic nft
// transaction.
func nftApply(v4, v6 []netip.Prefix) error {
	if err := sgNftRestore(nftBuildBatch(v4, v6)); err != nil {
		return fmt.Errorf("nftables apply failed: %w", err)
	}
	return nil
}

func nftProbe(st *ProbedStatus) {
	st.Backend = beNft
	st.IpsetAvailable = true // the nft path does not use ipset at all
	if out, err := sgCmd("nft", "-j", "list", "table", "inet", nftTable); err == nil {
		st.V4Entries, st.V6Entries = nftSetCounts(out)
	}
	if out, err := sgCmd("nft", "list", "chain", "inet", nftTable, nftChain); err == nil {
		text := string(out)
		st.RulesV4 = strings.Contains(text, "@"+setName4)
		st.RulesV6 = strings.Contains(text, "@"+setName6)
	}
}

// firstErrorLine keeps the message (drops the "Try iptables -h" hints).
func firstErrorLine(out []byte) string {
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// applyToFirewall swaps the sets atomically (build temp set, swap, destroy)
// so traffic between refreshes keeps hitting the previous set; the nft
// backend replaces the whole owned table atomically instead.
func applyToFirewall(v4, v6 []netip.Prefix) error {
	if len(v4) > setMaxelem || len(v6) > setMaxelem {
		return fmt.Errorf("list exceeds %d entries per family", setMaxelem)
	}
	if detectBackend() == beNft {
		return nftApply(v4, v6)
	}
	if _, err := sgCmd("ipset", "list", setName4); err != nil {
		if _, lookErr := exec.LookPath("ipset"); lookErr != nil {
			return errors.New("ipset not found — install the ipset package first")
		}
	}
	if err := sgIpsetRestore(restoreInput("inet", setName4, listBody(v4))); err != nil {
		return fmt.Errorf("ipset v4 restore failed: %w", err)
	}
	if err := sgIpsetRestore(restoreInput("inet6", setName6, listBody(v6))); err != nil {
		return fmt.Errorf("ipset v6 restore failed: %w", err)
	}
	return nil
}

// restoreInput renders a batch: create target+temp, fill temp, swap, destroy.
func restoreInput(family, name string, list []netip.Prefix) []byte {
	tmp := name + tmpSuffix
	var b strings.Builder
	fmt.Fprintf(&b, "create %s hash:net family %s maxelem %d -exist\n", tmp, family, setMaxelem)
	fmt.Fprintf(&b, "create %s hash:net family %s maxelem %d -exist\n", name, family, setMaxelem)
	fmt.Fprintf(&b, "flush %s\n", tmp)
	for _, p := range list {
		fmt.Fprintf(&b, "add %s %s\n", tmp, p.String())
	}
	fmt.Fprintf(&b, "swap %s %s\n", name, tmp)
	fmt.Fprintf(&b, "destroy %s\n", tmp)
	return []byte(b.String())
}

// listBody sorts prefixes so restores are byte-stable across refreshes.
func listBody(list []netip.Prefix) []netip.Prefix {
	out := append([]netip.Prefix(nil), list...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bits() != out[j].Bits() {
			return out[i].Bits() > out[j].Bits()
		}
		return out[i].Addr().String() < out[j].Addr().String()
	})
	return out
}

// ensureRules installs the owner-tagged NEW+set DROP rules for both families;
// the nft backend already got them with the apply transaction.
func ensureRules() error {
	if detectBackend() == beNft {
		return nil
	}
	if err := ensureRule("iptables", setName4); err != nil {
		return fmt.Errorf("iptables rule: %w", err)
	}
	if err := ensureRule("ip6tables", setName6); err != nil {
		logger.Warning("scanguard: ip6tables rule not installed:", err)
	}
	return nil
}

// ruleArgs builds the iptables argv WITHOUT the binary itself — sgCmd
// receives that separately. (Bug legacy note: prepending the binary here ran
// `iptables iptables -w 5 ...` → "Bad argument" exit 2 on every rule op.)
func ruleArgs(op, set string) []string {
	args := []string{"-w", "5", op, "INPUT"}
	if op == "-I" {
		args = append(args, "1")
	}
	return append(args,
		"-m", "conntrack", "--ctstate", "NEW",
		"-m", "set", "--match-set", set, "src",
		"-m", "comment", "--comment", firewallOwner,
		"-j", "DROP")
}

func ensureRule(bin, set string) error {
	if _, err := sgCmd(bin, ruleArgs("-C", set)...); err == nil {
		return nil
	}
	_, err := sgCmd(bin, ruleArgs("-I", set)...)
	return err
}

// removeFirewall sweeps every owner-tagged rule and destroys the sets
// (nft backend: deletes the owned table). Removing absent rules/sets is a
// success.
func removeFirewall() error {
	if detectBackend() == beNft {
		_, _ = sgCmd("nft", "delete", "table", "inet", nftTable)
		return nil
	}
	var firstErr error
	for _, bin := range []string{"iptables", "ip6tables"} {
		out, err := sgCmd(bin, "-w", "5", "-S", "INPUT")
		if err != nil {
			continue
		}
		for _, spec := range ownerRules(string(out)) {
			// The spec must arrive as one argv token per match/option — a
			// single combined token makes iptables read it as one match name
			// ("Invalid match name … 28 chars max").
			argv := append([]string{"-w", "5", "-D", "INPUT"}, strings.Fields(spec)...)
			if _, err := sgCmd(bin, argv...); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	for _, set := range []string{setName4, setName6} {
		_, _ = sgCmd("ipset", "destroy", set)
	}
	return firstErr
}

// ownerRules extracts the rule specs (without the "-A INPUT" prefix) of all
// owner-marked rules from `iptables -S INPUT` output, normalized the way
// `iptables -D INPUT <spec>` expects (no shell quotes around the comment).
func ownerRules(dump string) []string {
	specs := []string{}
	for _, line := range strings.Split(dump, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "-A INPUT ")
		if !ok {
			continue
		}
		parts := strings.Fields(rest)
		for i, p := range parts {
			parts[i] = strings.Trim(p, `"`)
		}
		spec := strings.Join(parts, " ")
		if strings.Contains(spec, "--comment "+firewallOwner) {
			specs = append(specs, spec)
		}
	}
	return specs
}

func probeStatus() ProbedStatus {
	st := ProbedStatus{}
	if detectBackend() == beNft {
		nftProbe(&st)
		return st
	}
	st.Backend = detectBackend()
	if _, err := sgCmd("ipset", "version"); err == nil {
		st.IpsetAvailable = true
	}
	if out, err := sgCmd("ipset", "list", setName4); err == nil {
		st.V4Entries = parseIpsetEntries(string(out))
	}
	if out, err := sgCmd("ipset", "list", setName6); err == nil {
		st.V6Entries = parseIpsetEntries(string(out))
	}
	st.RulesV4 = rulePresent("iptables", setName4)
	st.RulesV6 = rulePresent("ip6tables", setName6)
	return st
}

// parseIpsetEntries reads "Number of entries: N" from ipset list output.
func parseIpsetEntries(out string) int {
	for _, line := range strings.Split(out, "\n") {
		if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Number of entries:"))); err == nil {
			return n
		}
	}
	return 0
}

func rulePresent(bin, set string) bool {
	_, err := sgCmd(bin, ruleArgs("-C", set)...)
	return err == nil
}
