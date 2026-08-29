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

type AnytlsScrapeTarget struct {
	Key  string
	Tag  string
	Port int
}

type AnytlsTraffic struct {
	Tag      string
	Up       int64
	Down     int64
	Sessions int
}

type anytlsCursor struct {
	up, down    int64
	initialized bool
}

func (m *Manager) CollectAnytlsTraffic(targets []AnytlsScrapeTarget) []AnytlsTraffic {
	var out []AnytlsTraffic
	m.mu.Lock()
	if m.anytlsTraffic == nil {
		m.anytlsTraffic = make(map[string]*anytlsCursor)
	}
	m.mu.Unlock()

	for _, t := range targets {
		if t.Port <= 0 || !m.IsRunningKey(t.Key) {
			continue
		}
		ensureAnytlsAcct(t.Key, t.Port)
		up, down, bytesOK := anytlsByteCounters(t.Key)
		sessions := anytlsSessionCount(t.Port)
		d := AnytlsTraffic{Tag: t.Tag, Sessions: sessions}
		if bytesOK {
			m.mu.Lock()
			cur, ok := m.anytlsTraffic[t.Key]
			if !ok {
				cur = &anytlsCursor{}
				m.anytlsTraffic[t.Key] = cur
			}
			if cur.initialized {
				if up > cur.up {
					d.Up = up - cur.up
				}
				if down > cur.down {
					d.Down = down - cur.down
				}
			}
			cur.up, cur.down, cur.initialized = up, down, true
			m.mu.Unlock()
		}
		out = append(out, d)
	}
	return out
}

func parseTCPEstablished(dump string, port int, skipLoopback bool) int {
	if port <= 0 || port > 65535 {
		return 0
	}
	want := strings.ToUpper(strconv.FormatInt(int64(port), 16))
	for len(want) < 4 {
		want = "0" + want
	}
	want = ":" + want
	n := 0
	for _, line := range strings.Split(dump, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[0] == "sl" || !strings.Contains(fields[0], ":") {
			continue
		}
		if fields[3] != "01" {
			continue
		}
		local := strings.ToUpper(fields[1])
		if !strings.HasSuffix(local, want) {
			continue
		}
		if skipLoopback && isLoopbackRemote(fields[2]) {
			continue
		}
		n++
	}
	return n
}

func isLoopbackRemote(rem string) bool {
	u := strings.ToUpper(rem)
	if strings.HasPrefix(u, "0100007F:") {
		return true
	}
	if strings.HasPrefix(u, "00000000:") {
		return true
	}
	if strings.Contains(u, ":00000000000000000000000000000001:") || strings.HasPrefix(u, "00000000000000000000000000000001:") {
		return true
	}
	return false
}

func parseIptablesSave(dump, comment string) (up, down int64, ok bool) {
	if comment == "" {
		return 0, 0, false
	}
	needle := "--comment " + comment
	quoted := `--comment "` + comment + `"`
	for _, line := range strings.Split(dump, "\n") {
		if !strings.Contains(line, needle) && !strings.Contains(line, quoted) {
			continue
		}
		bytes := iptablesSaveBytes(line)
		switch {
		case strings.Contains(line, "-A INPUT"):
			down += bytes
			ok = true
		case strings.Contains(line, "-A OUTPUT"):
			up += bytes
			ok = true
		}
	}
	return up, down, ok
}

func iptablesSaveBytes(line string) int64 {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") {
		return 0
	}
	end := strings.IndexByte(line, ']')
	if end < 2 {
		return 0
	}
	inner := line[1:end]
	_, rest, found := strings.Cut(inner, ":")
	if !found {
		return 0
	}
	n, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func anytlsAcctComment(key string) string {
	return "lucx-anytls-" + strings.TrimSpace(key)
}
