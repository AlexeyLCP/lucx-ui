// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strconv"
	"strings"
)

type SidecarTraffic struct {
	Tag      string
	Up       int64
	Down     int64
	Sessions int
}

type deltaCursor struct {
	up, down    int64
	initialized bool
}

func (m *Manager) foldDelta(key string, up, down int64, ok bool) (dUp, dDown int64) {
	if !ok {
		return 0, 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sidecarDelta == nil {
		m.sidecarDelta = make(map[string]*deltaCursor)
	}
	cur, hit := m.sidecarDelta[key]
	if !hit {
		cur = &deltaCursor{}
		m.sidecarDelta[key] = cur
	}
	if cur.initialized {
		if up > cur.up {
			dUp = up - cur.up
		}
		if down > cur.down {
			dDown = down - cur.down
		}
	}
	cur.up, cur.down, cur.initialized = up, down, true
	return dUp, dDown
}

func parseProcIO(dump string) (rchar, wchar int64) {
	for _, line := range strings.Split(dump, "\n") {
		k, v, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil || n < 0 {
			continue
		}
		switch strings.TrimSpace(k) {
		case "rchar":
			rchar = n
		case "wchar":
			wchar = n
		}
	}
	return rchar, wchar
}

func parseIpLinkStats(dump string) (rx, tx int64) {
	lines := strings.Split(dump, "\n")
	for i, line := range lines {
		u := strings.ToUpper(strings.TrimSpace(line))
		next := int64(0)
		if i+1 < len(lines) {
			next = firstFieldInt(lines[i+1])
		}
		switch {
		case strings.HasPrefix(u, "RX:"):
			if n := firstFieldInt(line); n > 0 && !strings.Contains(u, "PACKETS") {
				rx = n
			} else {
				rx = next
			}
		case strings.HasPrefix(u, "TX:"):
			if n := firstFieldInt(line); n > 0 && !strings.Contains(u, "PACKETS") {
				tx = n
			} else {
				tx = next
			}
		}
	}
	return rx, tx
}

func firstFieldInt(line string) int64 {
	for _, f := range strings.Fields(line) {
		n, err := strconv.ParseInt(f, 10, 64)
		if err == nil && n >= 0 {
			return n
		}
	}
	return 0
}

func (m *Manager) CollectOlcrtcTraffic(key, tag string) SidecarTraffic {
	d := SidecarTraffic{Tag: tag}
	if !m.IsRunningKey(key) {
		return d
	}
	rchar, wchar, ok := olcrtcProcIO(m.PidOf(key))
	if !ok {
		return d
	}
	d.Up, d.Down = m.foldDelta(key, wchar, rchar, true)
	if d.Up > 0 || d.Down > 0 {
		d.Sessions = 1
	}
	return d
}

func (m *Manager) CollectQwdttTraffic(tag string) SidecarTraffic {
	d := SidecarTraffic{Tag: tag}
	if !m.IsRunningKey(QwdttKey) {
		return d
	}
	rx, tx, ok := qwdttIfaceStats()
	if !ok {
		return d
	}
	d.Up, d.Down = m.foldDelta(QwdttKey, rx, tx, true)
	if d.Up > 0 || d.Down > 0 {
		d.Sessions = 1
	}
	return d
}
