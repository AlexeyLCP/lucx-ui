// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The live speed columns on the Clients and Inbounds pages are fed by the
// per-poll deltas XrayTrafficJob broadcasts every 5s. AWG traffic never passes
// Xray's stats API (kernel TUN, no per-email accounting), so AWG rows would
// show a permanent dash. AwgJob stores its own scraped deltas here, normalized
// to the 5s window the frontend divides by, and XrayTrafficJob folds them into
// the same broadcast frame. The snapshot is sticky: re-sent by every 5s frame
// between two 10s AWG ticks (no flicker), and an idle tick stores an empty
// snapshot so the speed clears on the next frame.

const (
	// awgSpeedWindowMs is the poll window the frontend divides deltas by
	// (frontend TRAFFIC_POLL_INTERVAL_S = 5).
	awgSpeedWindowMs = int64(5000)
	// awgSpeedTTL drops the snapshot when the AWG job stops ticking
	// (cadenceAwg is 10s, so 20s covers a missed tick).
	awgSpeedTTL = 20 * time.Second
)

type awgSpeedSnapshot struct {
	traffics []*xray.Traffic
	clients  []*xray.ClientTraffic
	at       time.Time
}

var (
	awgSpeedMu sync.Mutex
	awgSpeed   awgSpeedSnapshot
)

func storeAwgSpeedSnapshot(traffics []*xray.Traffic, clients []*xray.ClientTraffic) {
	awgSpeedMu.Lock()
	defer awgSpeedMu.Unlock()
	awgSpeed = awgSpeedSnapshot{traffics: traffics, clients: clients, at: time.Now()}
}

func takeAwgSpeedSnapshot() ([]*xray.Traffic, []*xray.ClientTraffic, bool) {
	awgSpeedMu.Lock()
	defer awgSpeedMu.Unlock()
	if time.Since(awgSpeed.at) > awgSpeedTTL {
		return nil, nil, false
	}
	return awgSpeed.traffics, awgSpeed.clients, true
}

// normalizeAwgDeltas scales byte deltas measured over elapsed (the real AWG
// tick interval, ~10s) to the equivalent 5s-window deltas, mirroring
// nodeInboundSpeed in node_traffic_sync_job.go. Movers only: an absent row and
// a zero row render identically in the UI.
func normalizeAwgDeltas(traffics []*xray.Traffic, clients []*xray.ClientTraffic, elapsed time.Duration) ([]*xray.Traffic, []*xray.ClientTraffic) {
	ms := elapsed.Milliseconds()
	if ms < 1000 {
		ms = 1000
	}
	scale := func(v int64) int64 { return v * awgSpeedWindowMs / ms }

	outT := make([]*xray.Traffic, 0, len(traffics))
	for _, t := range traffics {
		if t == nil || t.Up+t.Down <= 0 {
			continue
		}
		outT = append(outT, &xray.Traffic{IsInbound: t.IsInbound, Tag: t.Tag, Up: scale(t.Up), Down: scale(t.Down)})
	}
	outC := make([]*xray.ClientTraffic, 0, len(clients))
	for _, ct := range clients {
		if ct == nil || ct.Up+ct.Down <= 0 {
			continue
		}
		outC = append(outC, &xray.ClientTraffic{Email: ct.Email, Up: scale(ct.Up), Down: scale(ct.Down)})
	}
	return outT, outC
}

// mergeAwgSpeedRows appends the AWG rows to the Xray broadcast slices, summing
// duplicates (a client attached to both an AWG and an Xray inbound, an AWG tag
// also metered by Xray) instead of double-listing them.
func mergeAwgSpeedRows(traffics []*xray.Traffic, clients []*xray.ClientTraffic, awgTraffics []*xray.Traffic, awgClients []*xray.ClientTraffic) ([]*xray.Traffic, []*xray.ClientTraffic) {
	for _, at := range awgTraffics {
		merged := false
		for _, t := range traffics {
			if t != nil && t.IsInbound && t.Tag == at.Tag {
				t.Up += at.Up
				t.Down += at.Down
				merged = true
				break
			}
		}
		if !merged {
			traffics = append(traffics, at)
		}
	}
	for _, ac := range awgClients {
		merged := false
		for _, ct := range clients {
			if ct != nil && ct.Email == ac.Email {
				ct.Up += ac.Up
				ct.Down += ac.Down
				merged = true
				break
			}
		}
		if !merged {
			clients = append(clients, ac)
		}
	}
	return traffics, clients
}
