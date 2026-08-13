// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MieruOnlineGrace is how long a client stays "online" after its last mita
// activity timestamp (LastActive in `mita get users`).
const MieruOnlineGrace = 120 * time.Second

// MieruScrapeTarget is one mieru instance the job wants scraped: manager key,
// inbound tag for accounting, and mita username → panel email.
type MieruScrapeTarget struct {
	Key         string
	Tag         string
	UserToEmail map[string]string
}

// MieruClientTraffic is a per-client byte delta since the previous scrape.
type MieruClientTraffic struct {
	Tag   string
	Email string
	Up    int64
	Down  int64
}

type mieruUserCursor struct {
	up, down    int64
	initialized bool
}

type mieruUserStats struct {
	up, down   int64
	lastActive time.Time
}

// CollectMieruTraffic scrapes each running mita daemon (`get users` over the
// per-instance RPC socket), folds per-user 1-day counters into deltas and
// reports online emails. Best-effort by construction: mita exposes rolling
// 24h windows, so deltas use the high-water mark — a window shift or daemon
// restart without a persisted metrics dump undercounts, never overcounts.
func (m *Manager) CollectMieruTraffic(targets []MieruScrapeTarget, now time.Time, grace time.Duration) ([]MieruClientTraffic, []string) {
	if grace <= 0 {
		grace = MieruOnlineGrace
	}
	var deltas []MieruClientTraffic
	onlineSet := make(map[string]struct{})

	m.mu.Lock()
	if m.mieruTraffic == nil {
		m.mieruTraffic = make(map[string]map[string]*mieruUserCursor)
	}
	m.mu.Unlock()

	for _, t := range targets {
		if !m.IsRunningKey(t.Key) {
			continue
		}
		stats, ok := m.scrapeMieruUsers(t.Key)
		if !ok {
			continue
		}
		m.mu.Lock()
		cursors, ok := m.mieruTraffic[t.Key]
		if !ok {
			cursors = make(map[string]*mieruUserCursor)
			m.mieruTraffic[t.Key] = cursors
		}
		for user, st := range stats {
			email := strings.TrimSpace(t.UserToEmail[user])
			if email == "" {
				continue
			}
			cur, ok := cursors[user]
			if !ok {
				cur = &mieruUserCursor{}
				cursors[user] = cur
			}
			var d MieruClientTraffic
			if cur.initialized {
				if st.up > cur.up {
					d.Up = st.up - cur.up
				}
				if st.down > cur.down {
					d.Down = st.down - cur.down
				}
			}
			cur.up, cur.down, cur.initialized = st.up, st.down, true
			if d.Up > 0 || d.Down > 0 {
				d.Tag = t.Tag
				d.Email = email
				deltas = append(deltas, d)
			}
			if !st.lastActive.IsZero() && now.Sub(st.lastActive) <= grace {
				onlineSet[email] = struct{}{}
			}
		}
		m.mu.Unlock()
	}

	online := make([]string, 0, len(onlineSet))
	for e := range onlineSet {
		online = append(online, e)
	}
	return deltas, online
}

// scrapeMieruUsers runs `mita get users` against the instance's RPC socket
// and parses the printed table.
func (m *Manager) scrapeMieruUsers(key string) (map[string]mieruUserStats, bool) {
	bin := Mieru.BinaryPath()
	if _, err := os.Stat(bin); err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "get", "users")
	// Absolute UDS path required — see absPath (relative "bin/…" breaks gRPC).
	cmd.Env = append(os.Environ(),
		"MITA_UDS_PATH="+absPath(filepath.Join(dataDirFor(key, Mieru), "mita.sock")),
		"MITA_INSECURE_UDS=1",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	return parseMieruUsersTable(string(out)), true
}

// parseMieruUsersTable parses the `mita get users` table:
//
//	User  LastActive  1DayDown  1DayUp  7DaysDown  7DaysUp  30DaysDown  30DaysUp
//	miabc  2026-08-13T10:00:00Z  1.5 MiB  200.0 KiB  ...
//
// Byte values are two tokens each ("<number> <unit>"); the 1-day window feeds
// the delta cursors. Malformed lines are skipped.
func parseMieruUsersTable(out string) map[string]mieruUserStats {
	res := make(map[string]mieruUserStats)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] == "User" {
			continue
		}
		user := fields[0]
		st := mieruUserStats{}
		if ts, err := time.Parse(time.RFC3339, fields[1]); err == nil {
			st.lastActive = ts
		}
		if mieruByteTokenCompact(fields[2]) {
			st.down = parseMieruByteToken(fields[2])
			st.up = parseMieruByteToken(fields[3])
		} else if len(fields) >= 6 {
			st.down = parseMieruByteCount(fields[2], fields[3])
			st.up = parseMieruByteCount(fields[4], fields[5])
		} else {
			continue
		}
		res[user] = st
	}
	return res
}

func mieruByteTokenCompact(s string) bool {
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

func parseMieruByteToken(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '-' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i == 0 || i == len(s) {
		return 0
	}
	return parseMieruByteCount(s[:i], s[i:])
}

// parseMieruByteCount converts mita's ByteCountIEC pair ("1.5" "MiB") to bytes.
func parseMieruByteCount(num, unit string) int64 {
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	var mult float64
	switch strings.TrimSpace(unit) {
	case "B":
		mult = 1
	case "KiB":
		mult = 1 << 10
	case "MiB":
		mult = 1 << 20
	case "GiB":
		mult = 1 << 30
	case "TiB":
		mult = 1 << 40
	case "PiB":
		mult = 1 << 50
	default:
		return 0
	}
	return int64(v * mult)
}
