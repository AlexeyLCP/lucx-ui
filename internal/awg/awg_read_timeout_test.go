//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func withStuckCooldown(t *testing.T, d time.Duration) {
	t.Helper()
	orig := awgStuckCooldown
	awgStuckCooldown = d
	t.Cleanup(func() { awgStuckCooldown = orig })
}

// countAwgRuns counts how many times the stub was actually executed for ifname.
func countAwgRuns(t *testing.T, dir, ifname string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "calls"))
	if err != nil {
		t.Fatalf("read stub call log: %v", err)
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if line == ifname {
			n++
		}
	}
	return n
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

// TestStuckInterfaceIsReadOncePerCooldown pins the damage bound: the deadline
// caps one read at ~820 MB of leak, and only the cooldown caps how often.
func TestStuckInterfaceIsReadOncePerCooldown(t *testing.T) {
	const wedged = "awgtest-cooldown"
	dir, flag := fakeAwgShow(t, wedged)

	for i := range 10 {
		if _, ok := scrapePeers(wedged); ok {
			t.Fatalf("traffic tick %d reported ok on a wedged interface", i+1)
		}
	}
	if n := countAwgRuns(t, dir, wedged); n != 1 {
		t.Fatalf("ten traffic ticks spawned %d `awg show` runs on one wedged interface, want 1: each run is a 5s read leaking ~164 MB/s, so ten of them are ~8 GB churned per hundred seconds", n)
	}

	for i := range 10 {
		if _, err := awgShowIfname(wedged); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("outbound read %d returned %v, want a deadline error without running awg", i+1, err)
		}
	}
	if _, err := (execProber{}).Run("awg", "show", wedged, "peers"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("diagnostics probe returned %v, want a deadline error without running awg", err)
	}
	if n := countAwgRuns(t, dir, wedged); n != 1 {
		t.Fatalf("the other two readers spawned %d runs in total, want the same 1: the cooldown is shared, not per-reader", n)
	}

	withStuckCooldown(t, 0)
	if err := os.Remove(flag); err != nil {
		t.Fatalf("clear wedge flag: %v", err)
	}
	if _, ok := scrapePeers(wedged); !ok {
		t.Fatal("an interface must be retried once its cooldown expires, otherwise a fixed device stays dead forever")
	}
	if _, err := awgShowIfname(wedged); err != nil {
		t.Fatalf("after a successful read the interface must be back in normal rotation: %v", err)
	}
	if n := countAwgRuns(t, dir, wedged); n != 3 {
		t.Fatalf("%d runs in total, want 3 (one while stuck, two after recovery)", n)
	}
}

// TestSweepOrphanClients_TimedOutInterfaceKeepsItsConf guards the consequence
// the deadline introduced: a read that ran out of time is not a missing device.
func TestSweepOrphanClients_TimedOutInterfaceKeepsItsConf(t *testing.T) {
	const wedged, gone = "awgo-91", "awgo-92"
	dir := withTempConfigDir(t)

	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$2\" = \"" + wedged + "\" ]; then exec sleep 60; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "awg"), []byte(script), 0o755); err != nil {
		t.Fatalf("write awg stub: %v", err)
	}
	// Nothing here may answer ok: an up interface would send the sweep into
	// awg-quick down, which edits the real host's routing.
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, name := range []string{wedged + ".conf", gone + ".conf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(xuiManagedMarker+"\n[Interface]\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	clientMu.Lock()
	saved := clients
	// Re-arm the once so this test drives a real sweep; it leaves it consumed,
	// which is the state any earlier sweep would also have left it in.
	clients, clientSwept = map[string]clientState{}, sync.Once{}
	clientMu.Unlock()
	t.Cleanup(func() {
		clientMu.Lock()
		clients = saved
		clientMu.Unlock()
	})

	(&Manager{}).SweepOrphanClients(nil)

	if _, err := os.Stat(filepath.Join(dir, gone+".conf")); err == nil {
		t.Fatal("the sweep kept a genuinely absent interface's .conf — it never ran its normal path, so this test proves nothing about the wedged one")
	}
	if _, err := os.Stat(filepath.Join(dir, wedged+".conf")); err != nil {
		t.Fatalf("the sweep deleted the .conf of an interface it merely failed to read in time: %v", err)
	}
}

// TestStuckInterfaceWarnsOnceAcrossAllReaders pins the shared latch: all three
// readers hit the same device, and the operator gets one line, not three.
func TestStuckInterfaceWarnsOnceAcrossAllReaders(t *testing.T) {
	const wedged = "awgtest-shared"
	fakeAwgShow(t, wedged)
	// All three must actually run the read here, or the latch is not what is
	// being tested; the read gate is pinned separately.
	withStuckCooldown(t, 0)

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
