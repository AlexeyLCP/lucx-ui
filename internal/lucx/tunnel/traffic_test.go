// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseAccessLogLine(t *testing.T) {
	perUser := make(map[string]struct{ up, down int64 })
	seenAt := make(map[string]int64)

	line := []byte(`{"ts":1609459200.5,"user_id":"nxabc","size":100,"bytes_read":40,"bytes_written":60}`)
	parseAccessLogLine(line, perUser, seenAt)
	if perUser["nxabc"].up != 40 || perUser["nxabc"].down != 60 {
		t.Fatalf("bytes = %+v", perUser["nxabc"])
	}
	if seenAt["nxabc"] < 1609459200500-1 || seenAt["nxabc"] > 1609459200500+1 {
		t.Fatalf("ts ms = %d", seenAt["nxabc"])
	}

	perUser = make(map[string]struct{ up, down int64 })
	seenAt = make(map[string]int64)
	parseAccessLogLine([]byte(`{"ts":1609459201,"user_id":"u2","size":500}`), perUser, seenAt)
	if perUser["u2"].up != 0 || perUser["u2"].down != 500 {
		t.Fatalf("size-only = %+v", perUser["u2"])
	}

	perUser = make(map[string]struct{ up, down int64 })
	seenAt = make(map[string]int64)
	parseAccessLogLine([]byte(`{"ts":1,"request":{"user_id":"from-req"},"size":1}`), perUser, seenAt)
	if _, ok := perUser["from-req"]; !ok {
		t.Fatal("expected request.user_id")
	}

	perUser = make(map[string]struct{ up, down int64 })
	parseAccessLogLine([]byte(`{"ts":1,"size":99}`), perUser, seenAt)
	if len(perUser) != 0 {
		t.Fatalf("anonymous line must be skipped: %+v", perUser)
	}

	parseAccessLogLine([]byte(`not-json`), perUser, seenAt)
}

func TestAccessLogTsMillis(t *testing.T) {
	if got := accessLogTsMillis("1609459200.123"); got != 1609459200123 {
		t.Fatalf("float sec = %d", got)
	}
	if got := accessLogTsMillis("1609459200"); got != 1609459200000 {
		t.Fatalf("int sec = %d", got)
	}
	if got := accessLogTsMillis("1609459200123"); got != 1609459200123 {
		t.Fatalf("int ms = %d", got)
	}
	if got := accessLogTsMillis(""); got != 0 {
		t.Fatalf("empty = %d", got)
	}
}

func TestScrapeAccessLogFirstSkipThenDelta(t *testing.T) {
	dir := t.TempDir()
	old := tunnelDir
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = old })

	key := "naive-7"
	path := AccessLogPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	backlog := `{"ts":1609459200,"user_id":"nx1","size":999}` + "\n"
	if err := os.WriteFile(path, []byte(backlog), 0o600); err != nil {
		t.Fatal(err)
	}

	per, seen, off, init := scrapeAccessLog(path, 0, false)
	if !init || len(per) != 0 || len(seen) != 0 {
		t.Fatalf("first scrape must skip backlog: per=%v seen=%v init=%v", per, seen, init)
	}
	if off != int64(len(backlog)) {
		t.Fatalf("offset = %d want %d", off, len(backlog))
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	line := `{"ts":1609459300.0,"user_id":"nx1","bytes_read":10,"bytes_written":20}` + "\n"
	if _, err := f.WriteString(line); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	per, seen, off2, _ := scrapeAccessLog(path, off, true)
	if per["nx1"].up != 10 || per["nx1"].down != 20 {
		t.Fatalf("delta = %+v", per["nx1"])
	}
	if seen["nx1"] != 1609459300000 {
		t.Fatalf("seen = %d", seen["nx1"])
	}
	if off2 <= off {
		t.Fatalf("offset did not advance: %d → %d", off, off2)
	}
}

func TestCollectNaiveTrafficOnlineAndMap(t *testing.T) {
	dir := t.TempDir()
	old := tunnelDir
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = old })

	m := newManager()
	key := "naive-1"
	path := AccessLogPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	targets := []NaiveScrapeTarget{{
		Key: key,
		Tag: "naive-in-1",
		UserToEmail: map[string]string{
			"nxalice": "alice@x",
			"nxbob":   "bob@x",
		},
	}}
	now := time.Unix(1_700_000_000, 0)
	_, _ = m.CollectNaiveTraffic(targets, now, time.Minute)

	ts := float64(now.Unix()) + 0.5
	line := fmt.Sprintf(`{"ts":%v,"user_id":"nxalice","bytes_read":100,"bytes_written":200}`+"\n", ts)
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	// After first collect on empty file, cursor offset=0. Truncating write leaves
	// size < previous EOF semantics: size was 0, offset 0. New content size>0,
	// offset 0, initialized → reads from start. But wait — first collect set
	// offset=0 (empty). WriteFile replaces content; size=len(line), offset still 0
	// → full read. However size went 0→N without truncate detection (offset not > size).
	// Good.

	// Problem: if first collect set offset=0 for empty, and we WriteFile, we read.
	// If first collect had backlog and skipped to EOF, WriteFile truncate would
	// make size < offset → reset. Here empty first is fine.

	// Actually after empty first: offset=0. WriteFile overwrites — we need cursor
	// to still be at 0. Yes. But CollectNaiveTraffic on empty set offset=0.
	// When we WriteFile the whole file, reading from 0 gets the line.
	// HOWEVER: if offset was 0 and initialized, and file was empty, then we wrote
	// — OK.

	// One issue: WriteFile after first collect with offset=0 on empty file means
	// the second scrape reads from 0. Perfect.

	deltas, online := m.CollectNaiveTraffic(targets, now.Add(time.Second), time.Minute)
	if len(deltas) != 1 || deltas[0].Email != "alice@x" || deltas[0].Up != 100 || deltas[0].Down != 200 {
		t.Fatalf("deltas = %+v", deltas)
	}
	if deltas[0].Tag != "naive-in-1" {
		t.Fatalf("tag = %q", deltas[0].Tag)
	}
	if len(online) != 1 || online[0] != "alice@x" {
		t.Fatalf("online = %v", online)
	}

	_, online2 := m.CollectNaiveTraffic(targets, now.Add(2*time.Minute), 30*time.Second)
	if len(online2) != 0 {
		t.Fatalf("stale must be offline: %v", online2)
	}
}

func TestRenderCaddyfileAccessLog(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.AuthUser = "u"
	cfg.AuthPass = "p"
	cfg.CertFile = "/c.pem"
	cfg.KeyFile = "/k.pem"
	got := cfg.RenderCaddyfile(nil, `/var/lib/x-ui/tunnel/naive-1-data/access.json`)
	for _, want := range []string{
		`output file "/var/lib/x-ui/tunnel/naive-1-data/access.json"`,
		"format json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	got = cfg.RenderCaddyfile(nil, "")
	if strings.Contains(got, "format json") {
		t.Fatalf("preview must omit access log:\n%s", got)
	}
}
