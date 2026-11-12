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

// nftApply installs the whole guard (sets + hooked chain + keep accepts) in
// one atomic nft transaction.
func nftApply(v4, v6 []netip.Prefix, keep []int) error {
	if err := sgNftRestore(nftBuildBatch(v4, v6, keep)); err != nil {
		return fmt.Errorf("nftables apply failed: %w", err)
	}
	return nil
}

func nftProbe(st *ProbedStatus, want []int) {
	st.Backend = beNft
	st.IpsetAvailable = true // the nft path does not use ipset at all
	if out, err := sgCmd("nft", "-j", "list", "table", "inet", nftTable); err == nil {
		st.V4Entries, st.V6Entries = nftSetCounts(out)
	}
	if out, err := sgCmd("nft", "list", "chain", "inet", nftTable, nftChain); err == nil {
		// `nft list chain` puts counters between the match and the verdict
		// (`… @lucx-sg4 counter packets 1 bytes 52 drop`), so match per line.
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.HasSuffix(line, "drop") {
				continue
			}
			if strings.Contains(line, "@"+setName4) {
				st.RulesV4 = true
			}
			if strings.Contains(line, "@"+setName6) {
				st.RulesV6 = true
			}
		}
		st.KeepOk = nftChainKeepOk(string(out), want)
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
func applyToFirewall(v4, v6 []netip.Prefix, keep []int) error {
	if len(v4) > setMaxelem || len(v6) > setMaxelem {
		return fmt.Errorf("list exceeds %d entries per family", setMaxelem)
	}
	if detectBackend() == beNft {
		return nftApply(v4, v6, keep)
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

// ensureRules installs the owner-tagged NEW+set DROP rules for both families
// and converges the tunnel-port keep ACCEPTs above them; the nft backend got
// everything with the apply transaction already.
func ensureRules(keep []int) error {
	if detectBackend() == beNft {
		return nil
	}
	if err := ensureFamilyRules("iptables", setName4, keep); err != nil {
		return fmt.Errorf("iptables rule: %w", err)
	}
	if err := ensureFamilyRules("ip6tables", setName6, keep); err != nil {
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

// keepRuleArgs builds the tunnel-port ACCEPT argv (no binary). The accept
// mirrors the drop's conntrack-NEW scope so foreign rules still apply to
// established flows.
func keepRuleArgs(op, proto string, port int) []string {
	args := []string{"-w", "5", op, "INPUT"}
	if op == "-I" {
		args = append(args, "1")
	}
	return append(args,
		"-p", proto, "--dport", strconv.Itoa(port),
		"-m", "conntrack", "--ctstate", "NEW",
		"-m", "comment", "--comment", keepOwner,
		"-j", "ACCEPT")
}

// ensureFamilyRules converges one family: ensure the DROP, then make the
// keep ACCEPTs exactly cover the wanted ports, every one of them ABOVE the
// DROP. Stale ports (tunnel moved/disabled) get swept, missing ones get
// inserted, keeps found below the DROP get re-inserted.
func ensureFamilyRules(bin, set string, keep []int) error {
	if err := ensureRule(bin, set); err != nil {
		return err
	}
	dump, err := sgCmd(bin, "-w", "5", "-S", "INPUT")
	if err != nil {
		return nil // cannot inspect the chain; the DROP is already ensured
	}
	delSpecs, insArgs := planKeepRules(string(dump), set, uniquePorts(keep))
	for _, spec := range delSpecs {
		argv := append([]string{"-w", "5", "-D", "INPUT"}, strings.Fields(spec)...)
		if _, err := sgCmd(bin, argv...); err != nil {
			logger.Warning("scanguard: keep rule sweep failed:", err)
		}
	}
	for _, args := range insArgs {
		if _, err := sgCmd(bin, args...); err != nil {
			logger.Warning("scanguard: keep rule insert failed:", err)
		}
	}
	return nil
}

// parseKeepDump walks an `iptables -S INPUT` dump in chain order and returns
// every owner KEEP rule (as ref + normalized delete spec) plus the line
// index of the family's DROP rule (-1 when absent). iptables -S quotes
// comment values, so matching happens on the quote-stripped spec.
// keepRef and uniquePorts live in nft.go — shared with the nft backend.
func parseKeepDump(dump, set string) (refs []keepRef, specs []string, dropIdx int) {
	dropIdx = -1
	for i, line := range strings.Split(dump, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-A INPUT ") {
			continue
		}
		spec := normalizeSpec(line)
		if strings.Contains(spec, "--comment "+keepOwner) &&
			strings.Contains(spec, "-j ACCEPT") {
			if proto, port, ok := keepRefOf(line); ok {
				refs = append(refs, keepRef{proto: proto, port: port, idx: i})
				specs = append(specs, spec)
			}
			continue
		}
		if dropIdx < 0 && strings.Contains(spec, "--match-set "+set+" src") &&
			strings.Contains(spec, "-j DROP") {
			dropIdx = i
		}
	}
	return refs, specs, dropIdx
}

// keepRefOf extracts (proto, port) from an iptables -S rule line (tokens
// arrive shell-quoted, e.g. --comment "lucx-scanguard-keep").
func keepRefOf(line string) (string, int, bool) {
	fields := strings.Fields(line)
	proto := ""
	port := 0
	hasAccept := false
	for i, part := range fields {
		part = strings.Trim(part, `"`)
		switch part {
		case "-p":
			if i+1 < len(fields) {
				proto = strings.Trim(fields[i+1], `"`)
			}
		case "--dport":
			if i+1 < len(fields) {
				port, _ = strconv.Atoi(strings.Trim(fields[i+1], `"`))
			}
		case "ACCEPT":
			hasAccept = true
		}
	}
	if !hasAccept || port <= 0 || (proto != "udp" && proto != "tcp") {
		return "", 0, false
	}
	return proto, port, true
}

// normalizeSpec turns one "-A INPUT …" dump line into the argv tail
// `iptables -D INPUT <spec>` / `-I INPUT 1 <spec>` expects.
func normalizeSpec(line string) string {
	rest, _ := strings.CutPrefix(strings.TrimSpace(line), "-A INPUT ")
	parts := strings.Fields(rest)
	for i := range parts {
		parts[i] = strings.Trim(parts[i], `"`)
	}
	return strings.Join(parts, " ")
}

// planKeepRules computes the firewall surgery that converges the keep rules
// to exactly `want`: sweep stale/duplicate/misplaced rules, insert missing
// ones at position 1 (above the DROP).
func planKeepRules(dump, set string, want []int) (delSpecs []string, insArgs [][]string) {
	refs, specs, dropIdx := parseKeepDump(dump, set)
	wanted := map[keepRef]struct{}{}
	for _, p := range want {
		for _, proto := range keepProtos {
			wanted[keepRef{proto: proto, port: p}] = struct{}{}
		}
	}
	placed := map[keepRef]struct{}{}
	for i := range refs {
		ref := refs[i]
		key := keepRef{proto: ref.proto, port: ref.port}
		_, isWanted := wanted[key]
		_, alreadyPlaced := placed[key]
		misplaced := dropIdx >= 0 && ref.idx > dropIdx
		if isWanted && !alreadyPlaced && !misplaced {
			placed[key] = struct{}{}
			continue
		}
		delSpecs = append(delSpecs, specs[i])
		if isWanted && !alreadyPlaced {
			placed[key] = struct{}{}
			insArgs = append(insArgs, keepRuleArgs("-I", ref.proto, ref.port))
		}
	}
	for _, proto := range keepProtos {
		for _, p := range want {
			ref := keepRef{proto: proto, port: p}
			if _, ok := placed[ref]; !ok {
				insArgs = append(insArgs, keepRuleArgs("-I", proto, p))
			}
		}
	}
	return delSpecs, insArgs
}

// keepOk checks one -S dump against the wanted ports: every wanted (proto,
// port) rule present exactly once and above the DROP, nothing extra.
func keepOk(dump, set string, want []int) bool {
	refs, _, dropIdx := parseKeepDump(dump, set)
	return keepRefsOk(refs, dropIdx, want)
}

// dropPresent replicates the old rulePresent probe from an -S dump.
func dropPresent(dump, set string) bool {
	_, _, dropIdx := parseKeepDump(dump, set)
	return dropIdx >= 0
}

// removeFirewall sweeps every owner-tagged rule (drops AND keep accepts) and
// destroys the sets (nft backend: deletes the owned table). Removing absent
// rules/sets is a success.
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
		if strings.Contains(spec, "--comment "+firewallOwner) ||
			strings.Contains(spec, "--comment "+keepOwner) {
			specs = append(specs, spec)
		}
	}
	return specs
}

func probeStatus(want []int) ProbedStatus {
	st := ProbedStatus{}
	if detectBackend() == beNft {
		nftProbe(&st, want)
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
	dump4, err4 := sgCmd("iptables", "-w", "5", "-S", "INPUT")
	if err4 == nil {
		st.RulesV4 = dropPresent(string(dump4), setName4)
	}
	dump6, err6 := sgCmd("ip6tables", "-w", "5", "-S", "INPUT")
	if err6 == nil {
		st.RulesV6 = dropPresent(string(dump6), setName6)
	}
	// KeepOk requires BOTH family dumps; an unreadable dump reports
	// unsatisfied so the reconcile tick re-applies and re-converges.
	st.KeepOk = err4 == nil && err6 == nil &&
		keepOk(string(dump4), setName4, want) &&
		keepOk(string(dump6), setName6, want)
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
