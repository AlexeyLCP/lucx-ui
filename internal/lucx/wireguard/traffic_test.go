// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"testing"
	"time"
)

func TestTrafficBaselineResetAndRemoval(t *testing.T) {
	c := NewCounters()
	now := time.Unix(1000, 0)
	first := []PeerCounter{{PublicKey: "a", Received: 100, Sent: 200, Handshake: 990}}
	if got := c.Observe(7, first, now); len(got) != 1 || got[0].Up != 0 || got[0].Down != 0 || !got[0].Online {
		t.Fatalf("first observation must establish a zero-charge baseline: %+v", got)
	}
	got := c.Observe(7, []PeerCounter{{PublicKey: "a", Received: 130, Sent: 280, Handshake: 990}}, now)
	if len(got) != 1 || got[0].Up != 30 || got[0].Down != 80 {
		t.Fatalf("bad delta: %+v", got)
	}
	got = c.Observe(7, []PeerCounter{{PublicKey: "a", Received: 2, Sent: 5, Handshake: 0}}, now)
	if len(got) != 1 || got[0].Up != 0 || got[0].Down != 0 || got[0].Online {
		t.Fatalf("reset must not charge old data: %+v", got)
	}
	c.Observe(7, nil, now)
	got = c.Observe(7, first, now)
	if got[0].Up != 0 || got[0].Down != 0 {
		t.Fatal("removed peer baseline was retained")
	}
}

func TestParseWireGuardDumpReadsOnlyPeerCounterFields(t *testing.T) {
	rows, err := ParseDump("PRIVATE PUBLIC 51820 off\npeer PSK endpoint 10.90.0.2/32 990 123 456 25\n")
	if err != nil || len(rows) != 1 || rows[0].PublicKey != "peer" || rows[0].Received != 123 || rows[0].Sent != 456 || rows[0].Handshake != 990 {
		t.Fatalf("bad parsed counters: %+v err=%v", rows, err)
	}
	if _, err := ParseDump("PRIVATE PUBLIC 51820 off\nmalformed\n"); err == nil {
		t.Fatal("malformed peer dump accepted")
	}
}

func TestNewPeerCountsFromZeroWithoutResettingExistingPeer(t *testing.T) {
	c := NewCounters()
	now := time.Now()
	c.Observe(7, []PeerCounter{{PublicKey: "old", Received: 100}}, now)
	c.SeedNew(7, []Peer{{PublicKey: "old"}, {PublicKey: "new"}})
	got := c.Observe(7, []PeerCounter{{PublicKey: "old", Received: 150}, {PublicKey: "new", Received: 80}}, now)
	if got[0].Up != 50 || got[1].Up != 80 {
		t.Fatalf("lost or duplicated traffic: %+v", got)
	}
}
