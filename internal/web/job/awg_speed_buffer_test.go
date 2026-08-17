// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestTakeAwgSpeedSnapshotEmpty(t *testing.T) {
	awgSpeedMu.Lock()
	awgSpeed = awgSpeedSnapshot{}
	awgSpeedMu.Unlock()

	if _, _, ok := takeAwgSpeedSnapshot(); ok {
		t.Fatal("fresh process must have no snapshot")
	}
}

func TestStoreTakeAwgSpeedSnapshot(t *testing.T) {
	traffics := []*xray.Traffic{{IsInbound: true, Tag: "awg1", Up: 10, Down: 20}}
	clients := []*xray.ClientTraffic{{Email: "a@x", Up: 1, Down: 2}}
	storeAwgSpeedSnapshot(traffics, clients)

	gT, gC, ok := takeAwgSpeedSnapshot()
	if !ok {
		t.Fatal("fresh snapshot must be taken")
	}
	if len(gT) != 1 || gT[0].Tag != "awg1" || len(gC) != 1 || gC[0].Email != "a@x" {
		t.Fatalf("snapshot roundtrip mismatch: %+v %+v", gT, gC)
	}

	storeAwgSpeedSnapshot(nil, nil)
	gT, gC, ok = takeAwgSpeedSnapshot()
	if !ok || len(gT) != 0 || len(gC) != 0 {
		t.Fatalf("idle tick must store an empty snapshot, got ok=%v %d %d", ok, len(gT), len(gC))
	}

	awgSpeedMu.Lock()
	awgSpeed = awgSpeedSnapshot{}
	awgSpeedMu.Unlock()
}

func TestNormalizeAwgDeltas(t *testing.T) {
	traffics := []*xray.Traffic{
		{IsInbound: true, Tag: "awg1", Up: 1000, Down: 2000},
		{IsInbound: true, Tag: "idle", Up: 0, Down: 0},
	}
	clients := []*xray.ClientTraffic{{Email: "a@x", Up: 100, Down: 300}}

	outT, outC := normalizeAwgDeltas(traffics, clients, 10*time.Second)
	if len(outT) != 1 {
		t.Fatalf("zero rows must be dropped, got %d", len(outT))
	}
	if outT[0].Up != 500 || outT[0].Down != 1000 {
		t.Fatalf("10s delta must halve into the 5s window: %+v", outT[0])
	}
	if outC[0].Up != 50 || outC[0].Down != 150 {
		t.Fatalf("client 10s delta must halve: %+v", outC[0])
	}

	// Sub-second elapsed clamps to 1s: the rate multiplier is capped at x5
	// instead of blowing up on a pathological double tick.
	outT, _ = normalizeAwgDeltas(traffics, nil, 500*time.Millisecond)
	if outT[0].Up != 5000 {
		t.Fatalf("sub-second elapsed must clamp to 1s (rate capped at x5), got %d", outT[0].Up)
	}
}

func TestMergeAwgSpeedRows(t *testing.T) {
	traffics := []*xray.Traffic{{IsInbound: true, Tag: "vless1", Up: 1, Down: 1}}
	clients := []*xray.ClientTraffic{{Email: "shared@x", Up: 10, Down: 10}}

	awgT := []*xray.Traffic{
		{IsInbound: true, Tag: "vless1", Up: 2, Down: 3},
		{IsInbound: true, Tag: "awg1", Up: 5, Down: 5},
	}
	awgC := []*xray.ClientTraffic{
		{Email: "shared@x", Up: 1, Down: 2},
		{Email: "awg@x", Up: 7, Down: 7},
	}

	mT, mC := mergeAwgSpeedRows(traffics, clients, awgT, awgC)
	if len(mT) != 2 || len(mC) != 2 {
		t.Fatalf("new tags/emails must be appended: %d %d", len(mT), len(mC))
	}
	if mT[0].Up != 3 || mT[0].Down != 4 {
		t.Fatalf("duplicate tag must sum: %+v", mT[0])
	}
	if mC[0].Up != 11 || mC[0].Down != 12 {
		t.Fatalf("duplicate email must sum: %+v", mC[0])
	}
}
