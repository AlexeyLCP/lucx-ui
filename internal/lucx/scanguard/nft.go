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
// the previous table untouched.
func nftBuildBatch(v4, v6 []netip.Prefix) []byte {
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
