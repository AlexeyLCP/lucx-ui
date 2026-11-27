// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

import (
	"os"
	"strings"
	"testing"
)

// Tester report (lucx.380-era, AnyTLS IPv6 EOF): originalTCPDest6 queried the
// SO_ORIGINAL_DST sockopt with the IPv4 level (IPPROTO_IP) — ENOENT on an
// AF_INET6 socket, every IPv6 connection closed silently with EOF. The pin
// keeps one IPv4-level call (the IPv4 branch) and one IPv6-level call in the
// source, so a revert in either direction fails here before it ships.
func TestOriginalDestSockoptLevels(t *testing.T) {
	src, err := os.ReadFile("tproxy_socks_linux.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	const v4level = "syscall.IPPROTO_IP,"
	const v6level = "syscall.IPPROTO_IPV6,"
	if strings.Count(got, v4level) != 1 {
		t.Fatalf("expected exactly one IPv4-level SO_ORIGINAL_DST call (the IPv4 branch), found %d",
			strings.Count(got, v4level))
	}
	if strings.Count(got, v6level) != 1 {
		t.Fatalf("expected exactly one IPv6-level SO_ORIGINAL_DST call (originalTCPDest6), found %d",
			strings.Count(got, v6level))
	}
	if strings.Contains(got, "nanoseconds()") || strings.Contains(got, "IPPROTO_IP, //") {
		t.Fatal("stale debug artifacts in tproxy_socks_linux.go")
	}
}

// A live end-to-end check (IPv6 TCP -> REDIRECT -> originalTCPDest6 -> SOCKS)
// needs CAP_NET_ADMIN and the nf_nat_ipv6 module on the loopback; it belongs
// to the tester's stand verification (see the bugreport checklist), not to
// this source pin.
