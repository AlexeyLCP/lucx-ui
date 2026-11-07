// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func newTestLogWriter() (*procLogWriter, *ring) {
	lines := &ring{}
	return &procLogWriter{label: "test", ring: lines}, lines
}

func TestProcLogWriterSplitsLines(t *testing.T) {
	w, lines := newTestLogWriter()
	if n, err := w.Write([]byte("first\nsecond\n")); err != nil || n != 13 {
		t.Fatalf("Write = (%d, %v), want (13, nil)", n, err)
	}
	got := lines.all(0)
	want := []string{"first", "second"}
	if len(got) != len(want) {
		t.Fatalf("captured %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A line arriving in pieces must be emitted once, whole, when the newline
// finally shows up — not once per Write.
func TestProcLogWriterJoinsSplitWrites(t *testing.T) {
	w, lines := newTestLogWriter()
	for _, chunk := range []string{"caddy: ", "listening on ", ":443"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write(%q) = %v", chunk, err)
		}
	}
	if got := lines.all(0); len(got) != 0 {
		t.Fatalf("captured %v before the newline, want nothing", got)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		t.Fatalf("Write = %v", err)
	}
	got := lines.all(0)
	if len(got) != 1 || got[0] != "caddy: listening on :443" {
		t.Fatalf("captured %v, want one joined line", got)
	}
}

func TestProcLogWriterTrimsCarriageReturn(t *testing.T) {
	w, lines := newTestLogWriter()
	if _, err := w.Write([]byte("windows style\r\n")); err != nil {
		t.Fatalf("Write = %v", err)
	}
	if got := lines.last(); got != "windows style" {
		t.Fatalf("last = %q, want %q", got, "windows style")
	}
}

func TestProcLogWriterDropsBlankLines(t *testing.T) {
	w, lines := newTestLogWriter()
	if _, err := w.Write([]byte("\n   \n\t\nreal\n")); err != nil {
		t.Fatalf("Write = %v", err)
	}
	got := lines.all(0)
	if len(got) != 1 || got[0] != "real" {
		t.Fatalf("captured %v, want only the non-blank line", got)
	}
}

// A core that never emits a newline (a \r progress bar, a single-line dump,
// binary garbage from a truncated download) must not grow the pending buffer
// without bound: at the cap the tail is flushed as its own line.
func TestProcLogWriterCapsUnterminatedLine(t *testing.T) {
	w, lines := newTestLogWriter()
	chunk := strings.Repeat("x", 4096)
	written := 0
	for written < maxPartialLine*2 {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write = %v", err)
		}
		written += len(chunk)
	}
	w.mu.Lock()
	pending := w.buf.Len()
	w.mu.Unlock()
	if pending > maxPartialLine {
		t.Fatalf("pending buffer = %d bytes, want at most %d", pending, maxPartialLine)
	}
	if len(lines.all(0)) == 0 {
		t.Fatal("nothing was emitted, want the capped tail flushed as a line")
	}
}

// Write reports bytes consumed from the caller's slice; the loop below
// reslices it internally, so a wrong return here would look like a short write
// to os/exec and abort the pipe.
func TestProcLogWriterReportsFullLength(t *testing.T) {
	w, _ := newTestLogWriter()
	payload := []byte("alpha\nbeta\ngamma")
	n, err := w.Write(payload)
	if err != nil {
		t.Fatalf("Write = %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write = %d, want %d", n, len(payload))
	}
}

func TestProcLogWriterFlushEmitsTail(t *testing.T) {
	w, lines := newTestLogWriter()
	if _, err := w.Write([]byte("fatal: cannot bind")); err != nil {
		t.Fatalf("Write = %v", err)
	}
	w.flush()
	if got := lines.last(); got != "fatal: cannot bind" {
		t.Fatalf("last after flush = %q, want the unterminated tail", got)
	}
	w.flush()
	if got := lines.all(0); len(got) != 1 {
		t.Fatalf("captured %v after a second flush, want the tail only once", got)
	}
}

func TestRingKeepsLastLines(t *testing.T) {
	r := &ring{}
	total := maxLogLines + 50
	for i := range total {
		r.push(strconv.Itoa(i))
	}
	got := r.all(0)
	if len(got) != maxLogLines {
		t.Fatalf("ring holds %d lines, want %d", len(got), maxLogLines)
	}
	if want := strconv.Itoa(total - 1); got[len(got)-1] != want {
		t.Fatalf("newest line = %q, want %q", got[len(got)-1], want)
	}
}

func TestProcLogWriterConcurrentWrites(t *testing.T) {
	w, lines := newTestLogWriter()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 32 {
				_, _ = w.Write([]byte("line from writer\n"))
			}
		}()
	}
	wg.Wait()
	if len(lines.all(0)) == 0 {
		t.Fatal("no lines captured from concurrent writers")
	}
}

func TestProcEchoFilterQwdttForwardsOnlyErrors(t *testing.T) {
	f := procEchoFilter(QwdttKey)
	if f == nil {
		t.Fatal("qwdtt writer must filter its console echo")
	}
	cases := map[string]bool{
		"[Stats] Active: 3 | Total: 12 | NAT: prs | 0.00 Mb/s | 0.00 Mb/s": false,
		"[RAW] device abc unregistered (ip=10.66.0.2, getConf=true)":       false,
		"[API] Starting HTTP API at 0.0.0.0:56000 (TCP)...":                false,
		"[CFG] password: hunter2":                                          false,
		"[ERR] WireGuard is not running, skipping SIGHUP":                  true,
		"[API] [ERR] HTTP API startup error: listen 443: address in use":   true,
		"panic: runtime error: index out of range":                         true,
	}
	for line, want := range cases {
		if got := f(line); got != want {
			t.Errorf("filter(%q) = %v, want %v", line, got, want)
		}
	}
	if procEchoFilter("caddy-1") != nil {
		t.Error("non-qwdtt labels must echo every line (nil filter)")
	}
}

// The qwdtt echo policy end-to-end: the ring keeps every line for the Logs
// viewer, but only error-tagged lines reach the panel log (journald).
func TestProcLogWriterQwdttRingKeepsAllEchoKeepsErrors(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	lines := &ring{}
	w := &procLogWriter{label: QwdttKey, ring: lines, echoFilter: procEchoFilter(QwdttKey)}
	for _, line := range []string{
		"[Stats] Active: 1 | Total: 1 | NAT: prs | 0.00 Mb/s | 0.00 Mb/s",
		"[ERR] Handshake failed from 10.66.0.2:44321: timeout",
	} {
		if _, err := w.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("Write = %v", err)
		}
	}
	if got := lines.all(0); len(got) != 2 {
		t.Fatalf("ring holds %v, want both lines for the Logs viewer", got)
	}
	logs := logger.GetLogs(50, "debug")
	var echoedStats, echoedErr bool
	for _, line := range logs {
		if strings.Contains(line, "tunnel: "+QwdttKey+" | [Stats] Active: 1") {
			echoedStats = true
		}
		if strings.Contains(line, "tunnel: "+QwdttKey+" | [ERR] Handshake failed") {
			echoedErr = true
		}
	}
	if echoedStats {
		t.Errorf("INFO line was echoed to the panel log:\n%s", strings.Join(logs, "\n"))
	}
	if !echoedErr {
		t.Errorf("error line was not echoed to the panel log:\n%s", strings.Join(logs, "\n"))
	}
}
