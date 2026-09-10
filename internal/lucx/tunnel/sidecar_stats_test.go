// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"testing"
	"time"
)

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
	t0 := time.Unix(1000, 0)
	u, d, last := m.foldDelta("k", 50, 80, true, t0)
	if u != 0 || d != 0 || !last.IsZero() {
		t.Fatalf("first scrape must baseline, got %d/%d last=%v", u, d, last)
	}
	t1 := t0.Add(10 * time.Second)
	u, d, last = m.foldDelta("k", 60, 90, true, t1)
	if u != 10 || d != 10 || !last.Equal(t1) {
		t.Fatalf("delta = %d/%d last=%v, want 10/10 at t1", u, d, last)
	}
	t2 := t1.Add(30 * time.Second)
	u, d, last = m.foldDelta("k", 60, 90, true, t2)
	if u != 0 || d != 0 || !last.Equal(t1) {
		t.Fatalf("idle keeps lastIO, got %d/%d last=%v", u, d, last)
	}
}

func TestSidecarOnlineGrace(t *testing.T) {
	now := time.Unix(10_000, 0)
	if sidecarOnline(time.Time{}, now) {
		t.Fatal("zero lastIO")
	}
	if !sidecarOnline(now, now) {
		t.Fatal("now")
	}
	if !sidecarOnline(now.Add(-SidecarOnlineGrace), now) {
		t.Fatal("edge")
	}
	if sidecarOnline(now.Add(-SidecarOnlineGrace-time.Second), now) {
		t.Fatal("expired")
	}
}

func TestLiveTags(t *testing.T) {
	got := LiveTags(map[string][]string{
		"a": {"alice@x"},
		"b": {"bob@x", "carol@x"},
		"c": {"alice@x"},
		"":  {"ghost@x"},
	}, []string{"alice@x", "ghost@x"}, []string{"extra", "a", ""})
	want := map[string]bool{"a": true, "c": true, "extra": true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, tag := range got {
		if !want[tag] {
			t.Fatalf("unexpected %q in %v", tag, got)
		}
	}
}
