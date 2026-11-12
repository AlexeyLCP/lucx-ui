// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package scanguard blocks incoming connections from known RKN/government
// scanner subnets. Lists are fetched from operator-configured HTTPS sources
// and installed as ipset hash:net sets, with owner-marked iptables/ip6tables
// rules in INPUT (conntrack NEW only, so established sessions survive list
// updates). No per-IP rules — one rule + one set per family.
package scanguard

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// DefaultSource is the blocklist shipped as the single default: known
// government / state-affiliated networks (the same list rkn-guard consumes).
const DefaultSource = "https://raw.githubusercontent.com/shadow-netlab/traffic-guard-lists/refs/heads/main/public/government_networks.list"

// fetchLimit and fetchTimeout cap every source download so a broken or
// hostile mirror cannot OOM the panel.
const (
	fetchLimit   = 20 << 20
	fetchTimeout = 30 * time.Second
)

// Shared across both firewall backends: the owner comment tag and the
// ipset/nft set names (the nft table is namespaced on its own, so the same
// names are safe there).
const (
	firewallOwner = "lucx-scanguard"
	// keepOwner marks the tunnel-port ACCEPT rules that sit above the DROPs
	// (see Apply) and are swept by Disable together with the drop rules.
	keepOwner = "lucx-scanguard-keep"
	setName4  = "lucx-sg4"
	setName6  = "lucx-sg6"
)

// Config is the operator-owned state of the feature.
type Config struct {
	Enabled   bool     `json:"enabled"`
	Sources   []string `json:"sources"`
	Whitelist []string `json:"whitelist"`
}

// DefaultConfig returns a config with the stock source and no whitelist.
func DefaultConfig() Config {
	return Config{Sources: []string{DefaultSource}, Whitelist: []string{}}
}

// ApplyResult reports the last successful apply.
type ApplyResult struct {
	V4      int `json:"v4"`
	V6      int `json:"v6"`
	Skipped int `json:"skipped"`
}

// ProbedStatus is the live OS-level state (no config involved). Backend
// names the firewall plumbing the probes/apply use: "iptables" (binary
// compat layer), "nft" (native nftables — needs neither ipset nor iptables)
// or "" when neither exists. KeepOk reports whether every wanted tunnel-port
// exemption is installed and ordered above the DROP rules.
type ProbedStatus struct {
	Backend        string `json:"backend,omitempty"`
	IpsetAvailable bool   `json:"ipsetAvailable"`
	RulesV4        bool   `json:"rulesV4"`
	RulesV6        bool   `json:"rulesV6"`
	V4Entries      int    `json:"v4Entries"`
	V6Entries      int    `json:"v6Entries"`
	KeepOk         bool   `json:"keepOk"`
}

// ErrUnsupported is returned off Linux; apply is a no-op there.
var ErrUnsupported = errors.New("scanner guard is linux-only")

// ErrDisabled is returned when a refresh is requested while the guard is off.
var ErrDisabled = errors.New("scanner guard is disabled")

// Validate checks the config before it reaches the firewall layer.
func Validate(cfg Config) error {
	if !cfg.Enabled {
		return nil
	}
	if len(cfg.Sources) == 0 {
		return errors.New("at least one list source is required")
	}
	for _, src := range cfg.Sources {
		if !strings.HasPrefix(src, "https://") && !strings.HasPrefix(src, "http://") {
			return fmt.Errorf("source %q is not an http(s) URL", src)
		}
	}
	for _, wl := range cfg.Whitelist {
		if _, err := parseOnePrefix(strings.TrimSpace(wl)); err != nil {
			return fmt.Errorf("whitelist entry %q: %w", wl, err)
		}
	}
	return nil
}

// httpFetch is a var so tests can feed canned list bodies.
var httpFetch = func(url string) ([]byte, error) {
	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("source %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, fetchLimit))
}

// Collect fetches every source, parses CIDRs and subtracts the whitelist.
// Any source failure aborts the whole refresh. This is the platform-neutral
// half of Apply — unit-testable everywhere.
func Collect(cfg Config) (v4, v6 []netip.Prefix, skipped int, err error) {
	if len(cfg.Sources) == 0 {
		return nil, nil, 0, errors.New("no list sources configured")
	}
	whitelist := map[netip.Prefix]struct{}{}
	for _, wl := range cfg.Whitelist {
		p, parseErr := parseOnePrefix(strings.TrimSpace(wl))
		if parseErr != nil {
			return nil, nil, 0, fmt.Errorf("whitelist entry %q: %w", wl, parseErr)
		}
		whitelist[p] = struct{}{}
	}

	blocked := map[netip.Prefix]struct{}{}
	for _, src := range cfg.Sources {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		body, fetchErr := httpFetch(src)
		if fetchErr != nil {
			return nil, nil, 0, fmt.Errorf("source fetch failed: %w", fetchErr)
		}
		pfx, listSkipped, parseErr := parsePrefixList(string(body))
		if parseErr != nil {
			return nil, nil, 0, fmt.Errorf("source %s is not a CIDR list: %w", src, parseErr)
		}
		skipped += listSkipped
		for p := range pfx {
			blocked[p] = struct{}{}
		}
	}

	for p := range blocked {
		if coveredBy(p, whitelist) {
			skipped++
			continue
		}
		if p.Addr().Is4() {
			v4 = append(v4, p)
		} else {
			v6 = append(v6, p)
		}
	}
	return v4, v6, skipped, nil
}

// Apply runs Collect, atomically swaps the ipset sets and ensures the INPUT
// rules match cfg.Enabled. keepPorts are the local listening ports (panel-
// owned tunnel sidecars) exempted from the drop — they land ABOVE the drop
// rules as owner-tagged ACCEPTs so a blocklist that covers a tunnel's own
// upstream relay subnets cannot cut the panel's sidecars (lucx.358).
func Apply(cfg Config, keepPorts []int) (ApplyResult, error) {
	var res ApplyResult
	v4, v6, skipped, err := Collect(cfg)
	if err != nil {
		return res, err
	}
	res.Skipped = skipped
	res.V4 = len(v4)
	res.V6 = len(v6)

	if err := applyToFirewall(v4, v6, keepPorts); err != nil {
		return res, err
	}
	if err := ensureRules(keepPorts); err != nil {
		return res, err
	}
	return res, nil
}

// Disable removes the owner-marked rules (drops AND tunnel-port accepts)
// and destroys the ipset sets.
func Disable() error {
	return removeFirewall()
}

// Status probes the live OS state without touching the firewall. want is
// the list of ports the keep rules must currently cover.
func Status(want []int) ProbedStatus {
	return probeStatus(want)
}

// parsePrefixList parses one list body. A single bogus line fails the whole
// list (strict, same as remnanode-antiscan) so a broken mirror never
// silently drops half the block data.
func parsePrefixList(text string) (map[netip.Prefix]struct{}, int, error) {
	pfx := map[netip.Prefix]struct{}{}
	skipped := 0
	for _, line := range strings.Split(text, "\n") {
		token := strings.Fields(line)
		if len(token) == 0 || strings.HasPrefix(token[0], "#") {
			continue
		}
		p, err := parseOnePrefix(token[0])
		if err != nil {
			return nil, skipped, fmt.Errorf("line %q: %w", line, err)
		}
		if unsafePrefix(p) {
			skipped++
			continue
		}
		pfx[p] = struct{}{}
	}
	if len(pfx) == 0 && skipped == 0 {
		return nil, skipped, errors.New("list parsed to zero entries")
	}
	return pfx, skipped, nil
}

// parseOnePrefix accepts CIDR notation (netip) and bare addresses.
func parseOnePrefix(token string) (netip.Prefix, error) {
	if strings.Contains(token, "/") {
		p, err := netip.ParsePrefix(token)
		if err != nil {
			return netip.Prefix{}, err
		}
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(token)
	if err != nil {
		return netip.Prefix{}, err
	}
	bits := 32
	if a.Is6() {
		bits = 128
	}
	return netip.PrefixFrom(a, bits), nil
}

// unsafePrefix filters entries that would cut the host off from its own
// infrastructure: everything broader than /8 (v4) / /32 (v6) and
// loopback/private/link-local/multicast ranges.
func unsafePrefix(p netip.Prefix) bool {
	a := p.Addr()
	if a.Is4() && p.Bits() < 8 {
		return true
	}
	if a.Is6() && !a.Is4In6() && p.Bits() < 32 {
		return true
	}
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified()
}

// coveredBy reports whether p equals or sits fully inside any prefix of the
// set. Partial overlaps are kept — CIDR subtraction is not worth the code.
func coveredBy(p netip.Prefix, set map[netip.Prefix]struct{}) bool {
	for wl := range set {
		if wl.Addr().BitLen() != p.Addr().BitLen() {
			continue
		}
		if wl.Bits() <= p.Bits() && wl.Contains(p.Addr()) {
			return true
		}
	}
	return false
}
