// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
)

// NaiveOnlineGrace is how long a client stays "online" after the last access_log
// line for their basic_auth user. Longer than Xray's 20s grace: Caddy logs
// CONNECT at request end, so long-lived H2 sessions produce sparse lines.
const NaiveOnlineGrace = 120 * time.Second

// maxAccessLogBytes caps the per-instance access.json before truncate-after-scrape.
const maxAccessLogBytes = 32 << 20

// NaiveScrapeTarget is one Naive instance the job wants scraped: manager key,
// inbound tag for accounting, and basic_auth username → panel email.
type NaiveScrapeTarget struct {
	Key         string
	Tag         string
	UserToEmail map[string]string
}

// NaiveClientTraffic is a per-client byte delta from access_log lines since the
// previous scrape (best-effort: long CONNECT tunnels often log only at close).
type NaiveClientTraffic struct {
	Tag   string
	Email string
	Up    int64
	Down  int64
}

type naiveLogCursor struct {
	offset      int64
	initialized bool
	lastSeen    map[string]int64
}

type naiveAccessLine struct {
	Ts     json.Number `json:"ts"`
	UserID string      `json:"user_id"`
	Size   int64       `json:"size"`
	// Caddy 2.6+ may emit request body bytes separately from response size.
	BytesRead    int64 `json:"bytes_read"`
	BytesWritten int64 `json:"bytes_written"`
	Request      *struct {
		UserID string `json:"user_id"`
	} `json:"request"`
}

// CollectNaiveTraffic tails each target's access.json, maps authenticated users
// to emails, returns per-client deltas and currently-online emails (last-seen
// within grace). First scrape per key seeks to EOF so historical log is not
// double-counted after a panel restart.
func (m *Manager) CollectNaiveTraffic(targets []NaiveScrapeTarget, now time.Time, grace time.Duration) ([]NaiveClientTraffic, []string) {
	if grace <= 0 {
		grace = NaiveOnlineGrace
	}
	nowMs := now.UnixMilli()
	graceMs := grace.Milliseconds()

	var deltas []NaiveClientTraffic
	onlineSet := make(map[string]struct{})

	m.mu.Lock()
	if m.naiveTraffic == nil {
		m.naiveTraffic = make(map[string]*naiveLogCursor)
	}
	m.mu.Unlock()

	for _, t := range targets {
		key := strings.TrimSpace(t.Key)
		if key == "" {
			continue
		}
		path := AccessLogPath(key)

		m.mu.Lock()
		cur := m.naiveTraffic[key]
		if cur == nil {
			cur = &naiveLogCursor{lastSeen: make(map[string]int64)}
			m.naiveTraffic[key] = cur
		}
		// Copy cursor fields under lock; scrape unlocked.
		offset := cur.offset
		initialized := cur.initialized
		lastSeenCopy := make(map[string]int64, len(cur.lastSeen))
		for u, ts := range cur.lastSeen {
			lastSeenCopy[u] = ts
		}
		m.mu.Unlock()

		perUser, seenAt, newOffset, newInit := scrapeAccessLog(path, offset, initialized)
		for u, ts := range seenAt {
			if ts < 0 {
				// Log line had no ts — attribute activity to this scrape.
				ts = nowMs
			}
			if prev, ok := lastSeenCopy[u]; !ok || ts > prev {
				lastSeenCopy[u] = ts
			}
		}

		// Per-email accumulate (same email could only appear once per user map).
		byEmail := make(map[string]struct{ up, down int64 })
		for user, c := range perUser {
			email := t.UserToEmail[user]
			if email == "" {
				continue
			}
			acc := byEmail[email]
			acc.up += c.up
			acc.down += c.down
			byEmail[email] = acc
		}
		for email, acc := range byEmail {
			if acc.up > 0 || acc.down > 0 {
				deltas = append(deltas, NaiveClientTraffic{
					Tag:   t.Tag,
					Email: email,
					Up:    acc.up,
					Down:  acc.down,
				})
			}
		}
		for user, ts := range lastSeenCopy {
			email := t.UserToEmail[user]
			if email == "" {
				continue
			}
			if nowMs-ts <= graceMs && ts > 0 {
				onlineSet[email] = struct{}{}
			}
		}

		m.mu.Lock()
		if c := m.naiveTraffic[key]; c != nil {
			c.offset = newOffset
			c.initialized = newInit
			c.lastSeen = lastSeenCopy
		}
		m.mu.Unlock()

		maybeTruncateAccessLog(path, newOffset)
	}

	online := make([]string, 0, len(onlineSet))
	for e := range onlineSet {
		online = append(online, e)
	}
	return deltas, online
}

func scrapeAccessLog(path string, offset int64, initialized bool) (perUser map[string]struct{ up, down int64 }, seenAt map[string]int64, newOffset int64, newInit bool) {
	perUser = make(map[string]struct{ up, down int64 })
	seenAt = make(map[string]int64)
	newOffset = offset
	newInit = initialized

	f, err := os.Open(path)
	if err != nil {
		return perUser, seenAt, newOffset, newInit
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return perUser, seenAt, newOffset, newInit
	}
	size := st.Size()
	if size < offset {
		// Truncated or rotated under us.
		offset = 0
	}
	if !initialized {
		// First sight of this log after process start: skip backlog.
		newOffset = size
		newInit = true
		return perUser, seenAt, newOffset, newInit
	}
	if offset > size {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return perUser, seenAt, newOffset, newInit
	}

	r := bufio.NewReader(f)
	var read int64
	for {
		line, err := r.ReadBytes('\n')
		read += int64(len(line))
		if len(line) > 0 {
			parseAccessLogLine(line, perUser, seenAt)
		}
		if err != nil {
			break
		}
	}
	newOffset = offset + read
	newInit = true
	return perUser, seenAt, newOffset, newInit
}

func parseAccessLogLine(line []byte, perUser map[string]struct{ up, down int64 }, seenAt map[string]int64) {
	line = []byte(strings.TrimSpace(string(line)))
	if len(line) == 0 {
		return
	}
	var entry naiveAccessLine
	if err := json.Unmarshal(line, &entry); err != nil {
		return
	}
	user := strings.TrimSpace(entry.UserID)
	if user == "" && entry.Request != nil {
		user = strings.TrimSpace(entry.Request.UserID)
	}
	if user == "" {
		return
	}
	up := entry.BytesRead
	down := entry.BytesWritten
	if down == 0 {
		down = entry.Size
	}
	// Some builds only emit size; count it as down (client download).
	if up == 0 && down == 0 && entry.Size > 0 {
		down = entry.Size
	}
	if up > 0 || down > 0 {
		acc := perUser[user]
		acc.up += up
		acc.down += down
		perUser[user] = acc
	}
	if ts := accessLogTsMillis(entry.Ts); ts > 0 {
		if prev, ok := seenAt[user]; !ok || ts > prev {
			seenAt[user] = ts
		}
	} else {
		// No ts: treat as "now" relative to scrape — job overwrites with wall clock
		// by using seenAt only when >0; set a marker via 0 means skip.
		// Use a high watermark of 1 so online path can refresh from scrape time
		// when caller merges — actually CollectNaiveTraffic uses seenAt as absolute.
		// Fallback: set to 0 and let caller use time.Now for lines without ts.
		// Simpler: if ts missing, set seenAt to current wall when merging — done below
		// by setting UnixMilli(0) sentinel... Use -1 as "use now".
		seenAt[user] = -1
	}
}

func accessLogTsMillis(n json.Number) int64 {
	if n == "" {
		return 0
	}
	s := string(n)
	// Caddy emits floating seconds (e.g. 1609459200.123). Integer millis
	// must not go through float*1000 (precision + false "seconds" scale).
	if strings.ContainsAny(s, ".eE") {
		if f, err := n.Float64(); err == nil && f > 0 {
			return int64(f * 1000)
		}
		return 0
	}
	if i, err := n.Int64(); err == nil && i > 0 {
		// Heuristic: unix seconds (~1e9) vs unix millis (~1e12).
		if i < 1_000_000_000_000 {
			return i * 1000
		}
		return i
	}
	return 0
}

// maybeTruncateAccessLog shrinks a huge access.json after a successful tail so
// disk does not grow without bound. Resets are picked up on the next scrape
// (size < offset → offset 0).
func maybeTruncateAccessLog(path string, offset int64) {
	st, err := os.Stat(path)
	if err != nil || st.Size() < maxAccessLogBytes {
		return
	}
	// Only truncate once we have consumed up to EOF-ish (offset near size).
	if offset+1024 < st.Size() {
		return
	}
	_ = os.Truncate(path, 0)
}
