// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type PeerCounter struct {
	PublicKey string
	Received  int64
	Sent      int64
	Handshake int64
}
type Delta struct {
	ID        int
	PublicKey string
	Up        int64
	Down      int64
	Online    bool
}
type Counters struct {
	last map[int]map[string]PeerCounter
}

func NewCounters() *Counters { return &Counters{last: map[int]map[string]PeerCounter{}} }
func (c *Counters) Observe(id int, peers []PeerCounter, now time.Time) []Delta {
	previous := c.last[id]
	next := map[string]PeerCounter{}
	out := make([]Delta, 0, len(peers))
	for _, p := range peers {
		d := Delta{ID: id, PublicKey: p.PublicKey, Online: p.Handshake > 0 && now.Unix() >= p.Handshake && now.Unix()-p.Handshake < 180}
		if old, ok := previous[p.PublicKey]; ok && p.Received >= old.Received && p.Sent >= old.Sent {
			d.Up = p.Received - old.Received
			d.Down = p.Sent - old.Sent
		}
		next[p.PublicKey] = p
		out = append(out, d)
	}
	c.last[id] = next
	return out
}

func ParseDump(text string) ([]PeerCounter, error) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 || len(strings.Fields(lines[0])) < 4 {
		return nil, fmt.Errorf("invalid WireGuard device counters")
	}
	var out []PeerCounter
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) != 8 {
			return nil, fmt.Errorf("invalid WireGuard peer counters")
		}
		h, e1 := strconv.ParseInt(f[4], 10, 64)
		rx, e2 := strconv.ParseInt(f[5], 10, 64)
		tx, e3 := strconv.ParseInt(f[6], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || h < 0 || rx < 0 || tx < 0 {
			return nil, fmt.Errorf("invalid WireGuard counter values")
		}
		out = append(out, PeerCounter{PublicKey: f[0], Handshake: h, Received: rx, Sent: tx})
	}
	return out, nil
}
