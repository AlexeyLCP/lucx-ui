// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import "testing"

func TestParseProcIO(t *testing.T) {
	rchar, wchar := parseProcIO("rchar: 100\nwchar: 250\nread_bytes: 3\n")
	if rchar != 100 || wchar != 250 {
		t.Fatalf("rchar=%d wchar=%d", rchar, wchar)
	}
}

func TestParseIpLinkStats(t *testing.T) {
	dump := `3: wdtt0: <POINTOPOINT,UP> mtu 1420
    link/none
    RX:  bytes packets errors dropped missed mcast
    1111 10 0 0 0 0
    TX:  bytes packets errors dropped carrier collsns
    2222 20 0 0 0 0
`
	rx, tx := parseIpLinkStats(dump)
	if rx != 1111 || tx != 2222 {
		t.Fatalf("rx=%d tx=%d", rx, tx)
	}
}

func TestFoldDelta(t *testing.T) {
	m := newManager()
	u, d := m.foldDelta("k", 50, 80, true)
	if u != 0 || d != 0 {
		t.Fatalf("first scrape must baseline, got %d/%d", u, d)
	}
	u, d = m.foldDelta("k", 60, 90, true)
	if u != 10 || d != 10 {
		t.Fatalf("delta = %d/%d, want 10/10", u, d)
	}
}
