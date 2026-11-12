// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package scanguard

import (
	"net/netip"
	"strings"
	"testing"
)

func TestMinimizeList(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "drop nested into broad", in: []string{"1.2.0.0/16", "1.2.3.0/24"}, want: []string{"1.2.0.0/16"}},
		{name: "broad arrives after child", in: []string{"1.0.0.0/24", "1.0.0.0/16", "1.0.1.0/24"}, want: []string{"1.0.0.0/16"}},
		{name: "siblings both kept", in: []string{"1.2.0.0/16", "1.3.0.0/16"}, want: []string{"1.2.0.0/16", "1.3.0.0/16"}},
		{name: "siblings of one parent", in: []string{"1.2.0.0/16", "1.2.64.0/18", "1.2.128.0/18"}, want: []string{"1.2.0.0/16"}},
		{name: "unrelated chains unchanged", in: []string{"5.6.7.0/24", "1.2.3.4/32"}, want: []string{"1.2.3.4/32", "5.6.7.0/24"}},
		{name: "duplicate collapses", in: []string{"5.6.7.0/24", "5.6.7.0/24"}, want: []string{"5.6.7.0/24"}},
		{name: "mixed families kept separate", in: []string{"0.0.0.0/0", "::/0"}, want: []string{"0.0.0.0/0", "::/0"}},
		{name: "single entry untouched", in: []string{"8.8.8.8/32"}, want: []string{"8.8.8.8/32"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []netip.Prefix{}
			for _, s := range tc.in {
				in = append(in, netip.MustParsePrefix(s))
			}
			var want []netip.Prefix
			for _, s := range tc.want {
				want = append(want, netip.MustParsePrefix(s))
			}
			got := minimizeList(in)
			if len(got) != len(want) {
				t.Fatalf("minimizeList = %v, want %v", got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("minimizeList[%d] = %v, want %v (all: %v)", i, got[i], want[i], got)
				}
			}
		})
	}
}

func TestNftBuildBatch(t *testing.T) {
	t.Parallel()
	v4 := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("203.0.114.0/24")}
	v6 := []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}
	batch := string(nftBuildBatch(v4, v6, nil))

	// The atomic-replacement idiom must come in exact order inside one
	// transaction: declare, delete, then the fresh table.
	if !strings.Contains(batch, "add table inet lucx_scanguard\ndelete table inet lucx_scanguard\ntable inet lucx_scanguard {") {
		t.Fatalf("replacement idiom broken:\n%s", batch)
	}
	if !strings.Contains(batch, "\t\ttype ipv4_addr\n\t\tflags interval\n") ||
		!strings.Contains(batch, "elements = { 203.0.113.0/24, 203.0.114.0/24 }") {
		t.Fatalf("v4 set malformed:\n%s", batch)
	}
	if !strings.Contains(batch, "type ipv6_addr") || !strings.Contains(batch, "elements = { 2001:db8::/32 }") {
		t.Fatalf("v6 set malformed:\n%s", batch)
	}
	if !strings.Contains(batch, "type filter hook input priority 0; policy accept;") ||
		!strings.Contains(batch, "ct state new ip saddr @lucx-sg4 drop") ||
		!strings.Contains(batch, "ct state new ip6 saddr @lucx-sg6 drop") {
		t.Fatalf("chain malformed:\n%s", batch)
	}
	if strings.Contains(batch, "ct state new") && strings.Contains(batch, "dport") {
		t.Fatalf("empty keep list must not add accept rules:\n%s", batch)
	}
}

// Tunnel-port keeps become accept rules placed before the drop rules, both
// transports per port, deduped and sorted.
func TestNftBuildBatchKeepPorts(t *testing.T) {
	t.Parallel()
	v4 := []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}
	batch := string(nftBuildBatch(v4, nil, []int{56000, 46000, 46000}))
	udp := strings.Index(batch, "ct state new udp dport 46000 accept")
	tcp := strings.Index(batch, "ct state new tcp dport 46000 accept")
	drop := strings.Index(batch, "ct state new ip saddr @lucx-sg4 drop")
	if udp < 0 || tcp < 0 || drop < 0 || udp > drop || tcp > drop {
		t.Fatalf("keep accepts must exist and sit above the drop:\n%s", batch)
	}
	if !strings.Contains(batch, "ct state new udp dport 56000 accept") ||
		!strings.Contains(batch, "ct state new tcp dport 56000 accept") {
		t.Fatalf("both transports per port required:\n%s", batch)
	}
}

// An empty family keeps the set declared but adds neither elements nor a
// drop rule — an empty set can never block traffic.
func TestNftBuildBatchEmptyV6(t *testing.T) {
	t.Parallel()
	batch := string(nftBuildBatch([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}, nil, nil))
	if strings.Contains(batch, "elements = { }") {
		t.Fatalf("empty v6 elements line must be absent:\n%s", batch)
	}
	if strings.Contains(batch, "ip6 saddr") {
		t.Fatalf("empty v6 must not add a drop rule:\n%s", batch)
	}
}

func TestNftSetCounts(t *testing.T) {
	t.Parallel()
	dump := []byte(`{"nftables":[
		{"metainfo":{"version":"1.0.9"}},
		{"set":{"family":"inet","name":"lucx-sg4","elem":["203.0.113.0/24",{"prefix":"5.6.7.0/24"}]}},
		{"set":{"family":"inet","name":"lucx-sg6","elem":[{"family":"ipv6","prefix":"2001:db8::/32"}]}},
		{"chain":{"name":"input"}}
	]}`)
	v4, v6 := nftSetCounts(dump)
	if v4 != 2 || v6 != 1 {
		t.Fatalf("counts = v4:%d v6:%d, want 2/1", v4, v6)
	}
	if v4, v6 := nftSetCounts([]byte(`{"nftables":[{"set":{"name":"lucx-sg4"}}]}`)); v4 != 0 || v6 != 0 {
		t.Fatalf("missing elem must count zero, got v4:%d v6:%d", v4, v6)
	}
	if v4, v6 := nftSetCounts([]byte("garbage")); v4 != 0 || v6 != 0 {
		t.Fatalf("garbage must count zero without panicking, got v4:%d v6:%d", v4, v6)
	}
}
