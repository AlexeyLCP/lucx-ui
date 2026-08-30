//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// awgShowStub stands in for the amneziawg-tools binary: it wedges on the named
// interface while the flag file exists, and dumps one peer for anything else.
const awgShowStub = `#!/bin/sh
if [ "$2" = "@WEDGED@" ] && [ -f "@FLAG@" ]; then exec sleep 60; fi
printf 'iface-privkey\tiface-pubkey\t51820\toff\n'
printf 'peerkey\t(none)\t1.2.3.4:1234\t10.0.0.2/32\t%s\t4000\t9000\t0\n' "$(date +%s)"
`

// fakeAwgShow installs the stub as the only `awg` on PATH and returns the flag
// file whose presence makes the wedged interface hang.
func fakeAwgShow(t *testing.T, wedged string) string {
	t.Helper()
	dir := t.TempDir()
	flag := filepath.Join(dir, "wedged")
	if err := os.WriteFile(flag, nil, 0o600); err != nil {
		t.Fatalf("write wedge flag: %v", err)
	}
	script := strings.NewReplacer("@WEDGED@", wedged, "@FLAG@", flag).Replace(awgShowStub)
	if err := os.WriteFile(filepath.Join(dir, "awg"), []byte(script), 0o755); err != nil {
		t.Fatalf("write awg stub: %v", err)
	}
	// Prepended, not replacing: the stub itself shells out to sleep and date.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return flag
}

// managedOnLoopback builds a managed entry whose liveness check passes (the
// proc watches lo) while CollectTraffic scrapes the synthetic ifname.
func managedOnLoopback(t *testing.T, ifname, tag string) *managed {
	t.Helper()
	if _, err := os.Stat("/sys/class/net/lo"); err != nil {
		t.Skipf("no /sys/class/net/lo, cannot fake a running interface: %v", err)
	}
	return &managed{
		proc:     newProcess("lo", "", tag),
		tag:      tag,
		ifname:   ifname,
		lastRx:   map[string]int64{"peerkey": 1000},
		lastTx:   map[string]int64{"peerkey": 2000},
		haveLast: true,
	}
}

func countStuckWarnings(t *testing.T, ifname string) int {
	t.Helper()
	n := 0
	for _, line := range logger.GetLogs(1000, "warning") {
		if strings.Contains(line, "awg show") && strings.Contains(line, ifname) {
			n++
		}
	}
	return n
}

// TestCollectTraffic_WedgedInterfaceDoesNotBlockOthers reproduces the adopted
// foreign interface whose oversized I-fields make `awg show` spin for ~30 min.
func TestCollectTraffic_WedgedInterfaceDoesNotBlockOthers(t *testing.T) {
	const wedged, healthy = "awgtest-wedged", "awgtest-healthy"
	fakeAwgShow(t, wedged)

	m := &Manager{procs: map[int]*managed{
		1: managedOnLoopback(t, wedged, "tag-wedged"),
		2: managedOnLoopback(t, healthy, "tag-healthy"),
	}}

	type result struct {
		traffic []Traffic
		peers   []PeerTraffic
		online  map[string][]string
	}
	done := make(chan result, 1)
	go func() {
		tr, pt, on := m.CollectTraffic()
		done <- result{tr, pt, on}
	}()

	var got result
	select {
	case got = <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("CollectTraffic did not return within 30s: `awg show %s dump` runs without a deadline, so one wedged interface withholds every other interface's statistics", wedged)
	}

	var healthyTraffic *Traffic
	for i := range got.traffic {
		switch got.traffic[i].Tag {
		case "tag-healthy":
			healthyTraffic = &got.traffic[i]
		case "tag-wedged":
			t.Fatalf("unreadable interface reported traffic %+v, want no entry at all", got.traffic[i])
		}
	}
	if healthyTraffic == nil {
		t.Fatalf("healthy interface produced no traffic entry, got %+v — a wedged neighbour must not cost it its statistics", got.traffic)
	}
	if healthyTraffic.Up != 3000 || healthyTraffic.Down != 7000 {
		t.Fatalf("healthy delta = up %d down %d, want up 3000 down 7000 (4000-1000 / 9000-2000)", healthyTraffic.Up, healthyTraffic.Down)
	}
	if online := got.online["tag-healthy"]; len(online) != 1 || online[0] != "peerkey" {
		t.Fatalf("healthy online peers = %v, want [peerkey]", online)
	}

	m.mu.Lock()
	rx, tx, have := m.procs[1].lastRx["peerkey"], m.procs[1].lastTx["peerkey"], m.procs[1].haveLast
	m.mu.Unlock()
	if rx != 1000 || tx != 2000 || !have {
		t.Fatalf("wedged baseline became rx=%d tx=%d haveLast=%v, want rx=1000 tx=2000 haveLast=true: an unreadable interface is not an interface with no peers", rx, tx, have)
	}
}

// TestScrapePeers_TimeoutWarnsOncePerInterface pins the log volume: the traffic
// job rescrapes every 10s for the whole half-hour a wedged device is stuck.
func TestScrapePeers_TimeoutWarnsOncePerInterface(t *testing.T) {
	const wedged = "awgtest-stuck"
	flag := fakeAwgShow(t, wedged)

	before := countStuckWarnings(t, wedged)
	for i := range 3 {
		if _, ok := scrapePeers(wedged); ok {
			t.Fatalf("scrape %d of a wedged interface reported ok, want ok=false", i+1)
		}
	}
	if n := countStuckWarnings(t, wedged) - before; n != 1 {
		t.Fatalf("3 wedged scrapes logged %d warnings, want exactly 1", n)
	}

	if err := os.Remove(flag); err != nil {
		t.Fatalf("clear wedge flag: %v", err)
	}
	if _, ok := scrapePeers(wedged); !ok {
		t.Fatal("a recovered interface must scrape normally")
	}
	if err := os.WriteFile(flag, nil, 0o600); err != nil {
		t.Fatalf("re-arm wedge flag: %v", err)
	}
	if _, ok := scrapePeers(wedged); ok {
		t.Fatal("re-wedged interface reported ok, want ok=false")
	}
	if n := countStuckWarnings(t, wedged) - before; n != 2 {
		t.Fatalf("a recurrence after recovery logged %d warnings in total, want 2", n)
	}
}

// TestScrapePeers_ExitCodeIsNotReportedAsTimeout separates the two failures: a
// down interface exits non-zero in milliseconds and is not a wedged device.
func TestScrapePeers_ExitCodeIsNotReportedAsTimeout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "awg"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write awg stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	const down = "awgtest-down"
	before := countStuckWarnings(t, down)
	if _, ok := scrapePeers(down); ok {
		t.Fatal("scrapePeers reported ok while awg exited non-zero, want ok=false")
	}
	if n := countStuckWarnings(t, down) - before; n != 0 {
		t.Fatalf("a non-zero exit logged %d timeout warnings, want 0", n)
	}
}
