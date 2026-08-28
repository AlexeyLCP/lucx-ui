// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"
)

// TestGetManager_Singleton verifies the Manager is a process-wide singleton:
// repeated calls return the same pointer.
func TestGetManager_Singleton(t *testing.T) {
	m1 := GetManager()
	m2 := GetManager()
	if m1 != m2 {
		t.Fatal("GetManager must return the same pointer")
	}
}

// TestManager_StopAllOnEmpty verifies StopAll is safe on an empty manager
// (no goroutines, no panic). Covers the shutdown path when no AWG inbounds
// exist yet.
func TestManager_StopAll_OnEmpty(t *testing.T) {
	m := GetManager()
	m.StopAll()
	if len(m.procs) != 0 {
		t.Fatalf("expected 0 procs after StopAll, got %d", len(m.procs))
	}
}

// TestManager_RemoveMissing verifies Remove is a no-op for an unknown id
// (must not panic or add state).
func TestManager_Remove_Missing(t *testing.T) {
	m := GetManager()
	before := len(m.procs)
	m.Remove(999999)
	if len(m.procs) != before {
		t.Fatalf("Remove of missing id must not change state: before=%d after=%d", before, len(m.procs))
	}
}

// TestManager_CollectTraffic_Empty verifies CollectTraffic returns nil with
// no running interfaces (the cron job calls this every 10s).
func TestManager_CollectTraffic_Empty(t *testing.T) {
	m := GetManager()
	out, peerOut, online := m.CollectTraffic()
	if len(out) != 0 || len(peerOut) != 0 || len(online) != 0 {
		t.Fatalf("expected empty results from CollectTraffic on empty manager, got %v %v %v", out, peerOut, online)
	}
}

// TestManager_Reconcile_EmptyDesired verifies Reconcile with an empty desired
// set is safe and leaves the manager empty. Does not exercise awg-quick
// (which requires Linux + the amneziawg kernel module); this guards the
// state-machine bookkeeping only.
func TestManager_Reconcile_EmptyDesired(t *testing.T) {
	m := GetManager()
	m.Reconcile(nil)
	if len(m.procs) != 0 {
		t.Fatalf("expected 0 procs after Reconcile(nil), got %d", len(m.procs))
	}
}

func TestParseAwgDump(t *testing.T) {
	dump := "awg1\tPRIV\tPUB\t51820\toff\n" +
		"peerA\tpskA\t1.2.3.4:51820\t10.8.0.2/32\t1800000000\t1024\t2048\t25\n" +
		"peerB\tpskB\t(off)\t10.8.0.3/32\t0\t0\t0\toff\n"
	peers, ok := parseAwgDump(dump)
	if !ok {
		t.Fatal("parseAwgDump must succeed on a dump with peers")
	}
	if len(peers) != 2 {
		t.Fatalf("parsed %d peers, want 2", len(peers))
	}
	if peers[0].PublicKey != "peerA" || peers[0].Rx != 1024 || peers[0].Tx != 2048 || peers[0].LastHandshake != 1800000000 {
		t.Errorf("peerA parsed wrong: %+v", peers[0])
	}
	if peers[0].Endpoint != "1.2.3.4:51820" || peers[0].AllowedIPs != "10.8.0.2/32" {
		t.Errorf("peerA endpoint/ips: %+v", peers[0])
	}
	if peers[1].LastHandshake != 0 {
		t.Errorf("peerB must keep zero handshake, got %d", peers[1].LastHandshake)
	}
}

func TestParseAwgDump_NoPeers(t *testing.T) {
	peers, ok := parseAwgDump("awg1\tPRIV\tPUB\t51820\toff\n")
	if !ok || len(peers) != 0 {
		t.Errorf("interface-only dump must yield ok=true with 0 peers, got ok=%v peers=%v", ok, peers)
	}
	if _, ok := parseAwgDump(""); ok {
		t.Error("empty output must yield ok=false (interface down/absent)")
	}
}

func TestParseAwgDump_SkipsMalformed(t *testing.T) {
	dump := "awg1\tPRIV\tPUB\t51820\toff\n" +
		"peerA\tpskA\n" +
		"peerB\tpskB\t1.2.3.4:51820\t10.8.0.3/32\tnotanumber\t1\t2\t0\n" +
		"peerC\tpskC\t1.2.3.5:51820\t10.8.0.4/32\t1800000000\t100\t200\t0\n"
	peers, ok := parseAwgDump(dump)
	if !ok || len(peers) != 1 || peers[0].PublicKey != "peerC" {
		t.Errorf("malformed rows must be skipped, got ok=%v peers=%+v", ok, peers)
	}
}

func TestParseInboundConfName(t *testing.T) {
	for _, tc := range []struct {
		name   string
		wantID int
		wantOK bool
	}{
		{"awg1.conf", 1, true},
		{"awg6.conf", 6, true},
		{"awg14.conf", 14, true},
		{"awgo-1.conf", 0, false}, // outbound tunnel, never an inbound conf
		{"awgo-3.conf", 0, false},
		{"awg.conf", 0, false},  // no digits
		{"awgX.conf", 0, false}, // non-digit segment
		{"awg1", 0, false},      // missing .conf
		{"wg1.conf", 0, false},  // wrong prefix
		{"", 0, false},
	} {
		id, ok := parseInboundConfName(tc.name)
		if id != tc.wantID || ok != tc.wantOK {
			t.Errorf("parseInboundConfName(%q) = (%d,%v), want (%d,%v)", tc.name, id, ok, tc.wantID, tc.wantOK)
		}
	}
}

func TestSetRebuildPause_BlocksEnsure(t *testing.T) {
	m := GetManager()
	SetRebuildPause(true)
	t.Cleanup(func() { SetRebuildPause(false) })
	if err := m.Ensure(Instance{Id: 1, Ifname: "awg1", Port: 51820}); err == nil {
		t.Fatal("Ensure must fail while rebuild is paused")
	}
}
