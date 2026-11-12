// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package scanguard

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

// Native nftables backend: used when the host has no usable ipset/iptables
// pair (nftables-first distros, stripped containers). State lives entirely
// in one owned table so apply is a single atomic nft transaction — no temp
// sets, no ipset, no per-rule iptables calls.

const (
	nftTable = "lucx_scanguard"
	nftChain = "input"
)

// nftBuildBatch renders the atomic replacement idiom (nftables wiki):
// `add table` guarantees the `delete table` never fails, then the table is
// re-declared fresh — all inside one transaction, so a failed apply leaves
// the previous table untouched. keepPorts become accept rules placed above
// the drops (tunnel sidecar ports survive a blocklist covering their own
// upstream relay subnets).
func nftBuildBatch(v4, v6 []netip.Prefix, keep []int) []byte {
	v4, v6 = minimizeList(v4), minimizeList(v6)
	var b strings.Builder
	fmt.Fprintf(&b, "add table inet %s\n", nftTable)
	fmt.Fprintf(&b, "delete table inet %s\n", nftTable)
	fmt.Fprintf(&b, "table inet %s {\n", nftTable)
	fmt.Fprintf(&b, "\tset %s {\n\t\ttype ipv4_addr\n\t\tflags interval\n", setName4)
	if len(v4) > 0 {
		fmt.Fprintf(&b, "\t\telements = { %s }\n", joinNftElements(v4))
	}
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\tset %s {\n\t\ttype ipv6_addr\n\t\tflags interval\n", setName6)
	if len(v6) > 0 {
		fmt.Fprintf(&b, "\t\telements = { %s }\n", joinNftElements(v6))
	}
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\tchain %s {\n", nftChain)
	fmt.Fprintf(&b, "\t\ttype filter hook input priority 0; policy accept;\n")
	for _, proto := range keepProtos {
		for _, port := range uniquePorts(keep) {
			fmt.Fprintf(&b, "\t\tct state new %s dport %d accept\n", proto, port)
		}
	}
	if len(v4) > 0 {
		fmt.Fprintf(&b, "\t\tct state new ip saddr @%s drop\n", setName4)
	}
	if len(v6) > 0 {
		fmt.Fprintf(&b, "\t\tct state new ip6 saddr @%s drop\n", setName6)
	}
	fmt.Fprintf(&b, "\t}\n}\n")
	return []byte(b.String())
}

func joinNftElements(list []netip.Prefix) string {
	items := make([]string, 0, len(list))
	for _, p := range list {
		items = append(items, p.String())
	}
	return strings.Join(items, ", ")
}

// nftChainKeepOk mirrors the iptables keep check for the native backend:
// the chain text must carry exactly the wanted tunnel-port accepts, above
// the drop rules. `nft list chain` renders rules in order, counters after
// the match expression.
func nftChainKeepOk(text string, want []int) bool {
	dropIdx := -1
	var refs []keepRef
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if dropIdx < 0 && strings.HasSuffix(line, "drop") &&
			(strings.Contains(line, "@"+setName4) || strings.Contains(line, "@"+setName6)) {
			dropIdx = i
		}
		if ref, ok := nftKeepRefOf(line); ok {
			ref.idx = i
			refs = append(refs, ref)
		}
	}
	return keepRefsOk(refs, dropIdx, want)
}

// nftKeepRefOf parses one chain rule line like `ct state new udp dport 46000
// counter packets 0 bytes 0 accept`.
func nftKeepRefOf(line string) (keepRef, bool) {
	if !strings.Contains(line, " accept") || strings.Contains(line, "drop") {
		return keepRef{}, false
	}
	fields := strings.Fields(line)
	for i := 1; i < len(fields); i++ {
		if fields[i] != "dport" {
			continue
		}
		proto := fields[i-1]
		if i+1 >= len(fields) {
			return keepRef{}, false
		}
		port, err := strconv.Atoi(fields[i+1])
		if err != nil {
			return keepRef{}, false
		}
		if proto == "udp" || proto == "tcp" {
			return keepRef{proto: proto, port: port}, true
		}
		return keepRef{}, false
	}
	return keepRef{}, false
}

// tunnel-port keep infrastructure — shared by both firewall backends
// (platform-neutral so nft.go and the guarded helpers compile everywhere).

// keepProtos are the transports exempted per keep port (a TURN/RTP sidecar
// answers on both).
var keepProtos = []string{"udp", "tcp"}

// keepRef is one tunnel-port ACCEPT rule identified by (proto, port); idx
// is its position in the dump the ref was parsed from (order checks).
type keepRef struct {
	proto string
	port  int
	idx   int
}

// uniquePorts dedups and range-checks the wanted keep ports.
func uniquePorts(keep []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(keep))
	for _, p := range keep {
		if p <= 0 || p > 65535 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// keepRefsOk is the shared (proto, port) set/order comparison for both
// backends: exact coverage of `want`, no extras/duplicates, every keep rule
// above the DROP.
func keepRefsOk(refs []keepRef, dropIdx int, want []int) bool {
	wanted := map[keepRef]struct{}{}
	for _, p := range uniquePorts(want) {
		for _, proto := range keepProtos {
			wanted[keepRef{proto: proto, port: p}] = struct{}{}
		}
	}
	seen := map[keepRef]int{}
	for _, ref := range refs {
		seen[keepRef{proto: ref.proto, port: ref.port}]++
	}
	if len(seen) != len(wanted) {
		return false
	}
	for ref, n := range seen {
		if _, ok := wanted[ref]; !ok || n != 1 {
			return false
		}
	}
	for _, ref := range refs {
		if dropIdx >= 0 && ref.idx > dropIdx {
			return false
		}
	}
	return true
}

// minimizeList drops every prefix contained in another listed prefix. nft
// interval sets reject overlapping elements (the ipset hash:net tolerated
// them), so the nft path needs exact nesting elimination. Sort by (addr asc,
// bits asc): any container sorts before its children, so one left-to-right
// comparison against the last kept prefix is enough.
func minimizeList(list []netip.Prefix) []netip.Prefix {
	if len(list) < 2 {
		return list
	}
	out := append([]netip.Prefix(nil), list...)
	sort.Slice(out, func(i, j int) bool {
		lo, hi := out[i].Addr(), out[j].Addr()
		if c := lo.Compare(hi); c != 0 {
			return c < 0
		}
		return out[i].Bits() < out[j].Bits()
	})
	kept := out[:0]
	for _, p := range out {
		if len(kept) > 0 && kept[len(kept)-1].Contains(p.Addr()) && kept[len(kept)-1].Bits() <= p.Bits() {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// nftSetCounts counts elements of the two owned sets inside `nft -j list
// table` output. Element entries vary in shape across nft versions (strings,
// objects with counters), so each elem is only counted, never decoded.
func nftSetCounts(out []byte) (v4, v6 int) {
	var doc struct {
		Objects []struct {
			Set struct {
				Name string            `json:"name"`
				Elem []json.RawMessage `json:"elem"`
			} `json:"set"`
		} `json:"nftables"`
	}
	if json.Unmarshal(out, &doc) != nil {
		return 0, 0
	}
	for _, o := range doc.Objects {
		switch o.Set.Name {
		case setName4:
			v4 = len(o.Set.Elem)
		case setName6:
			v6 = len(o.Set.Elem)
		}
	}
	return v4, v6
}
