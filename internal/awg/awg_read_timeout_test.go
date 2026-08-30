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

// hangingBin drops a never-returning binary of that name into dir, so a probe
// that carries no interface name can be made to hit the same deadline.
func hangingBin(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexec sleep 60\n"), 0o755); err != nil {
		t.Fatalf("write %s stub: %v", name, err)
	}
}

func countTimeoutWarnings(t *testing.T) int {
	t.Helper()
	n := 0
	for _, line := range logger.GetLogs(1000, "warning") {
		if strings.Contains(line, "reading interface ") && strings.Contains(line, "timed out") {
			n++
		}
	}
	return n
}

// TestRunningClientIfnames_WedgedInterfaceDoesNotHideOthers covers the second
// unbounded reader: awgShowIfname, called in a loop over every awgo-N outbound.
func TestRunningClientIfnames_WedgedInterfaceDoesNotHideOthers(t *testing.T) {
	const wedged, healthy = "awgo-test-wedged", "awgo-test-healthy"
	fakeAwgShow(t, wedged)

	clientMu.Lock()
	saved := clients
	clients = map[string]clientState{wedged: {}, healthy: {}}
	clientMu.Unlock()
	t.Cleanup(func() {
		// TryLock: in the red run the wedged reader still owns the mutex, and
		// a blocking cleanup would replace the verdict with a test-binary hang.
		if clientMu.TryLock() {
			clients = saved
			clientMu.Unlock()
		}
	})

	done := make(chan []string, 1)
	go func() { done <- (&Manager{}).RunningClientIfnames() }()

	select {
	case names := <-done:
		if len(names) != 1 || names[0] != healthy {
			t.Fatalf("RunningClientIfnames() = %v, want [%s]: a wedged neighbour must not cost the others their entry", names, healthy)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("RunningClientIfnames did not return within 30s: awgShowIfname reads `awg show %s dump` without a deadline, so one wedged outbound hides every other one from the dashboard", wedged)
	}
}

// TestExecProber_WedgedProbeIsBoundedAndShared covers the third reader: the
// diagnostics prober, which the panel runs on operator demand.
func TestExecProber_WedgedProbeIsBoundedAndShared(t *testing.T) {
	const wedged = "awgtest-diag"
	dir, _ := fakeAwgShow(t, wedged)

	before := countStuckWarnings(t, wedged)
	type probe struct {
		out string
		err error
	}
	done := make(chan probe, 1)
	go func() {
		out, err := execProber{}.Run("awg", "show", wedged, "peers")
		done <- probe{out, err}
	}()

	select {
	case got := <-done:
		if got.err == nil {
			t.Fatalf("execProber.Run on a wedged interface returned no error, out=%q", got.out)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("execProber.Run did not return within 30s: the diagnostics prober runs `awg show %s peers` without a deadline, so opening the AWG diagnostics view on a wedged interface never answers", wedged)
	}
	if n := countStuckWarnings(t, wedged) - before; n != 1 {
		t.Fatalf("the diagnostics probe logged %d warnings for %s, want 1: it must share the per-interface latch, not stay silent and not report twice", n, wedged)
	}

	// A host-wide probe owns no interface, so its deadline must not produce a
	// warning naming one.
	hangingBin(t, dir, "ip")
	beforeAny := countTimeoutWarnings(t)
	if _, err := (execProber{}).Run("ip", "link", "show", wedged); err == nil {
		t.Fatal("a hanging `ip link show` returned no error")
	}
	if n := countTimeoutWarnings(t) - beforeAny; n != 0 {
		t.Fatalf("a host-wide probe timing out logged %d interface warnings, want 0", n)
	}
}

// TestStuckInterfaceWarnsOnceAcrossAllReaders pins the shared latch: all three
// readers hit the same device, and the operator gets one line, not three.
func TestStuckInterfaceWarnsOnceAcrossAllReaders(t *testing.T) {
	const wedged = "awgtest-shared"
	fakeAwgShow(t, wedged)

	before := countStuckWarnings(t, wedged)
	if _, ok := scrapePeers(wedged); ok {
		t.Fatal("scrapePeers reported ok on a wedged interface")
	}
	if out, err := awgShowIfname(wedged); err == nil {
		t.Fatalf("awgShowIfname reported the wedged interface as readable, out=%q", out)
	}
	if _, err := (execProber{}).Run("awg", "show", wedged, "peers"); err == nil {
		t.Fatal("execProber.Run reported the wedged interface as readable")
	}
	if n := countStuckWarnings(t, wedged) - before; n != 1 {
		t.Fatalf("three readers of one wedged interface logged %d warnings, want exactly 1", n)
	}
}
