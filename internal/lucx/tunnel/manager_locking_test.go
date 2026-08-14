// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"sync"
	"testing"
)

// acquire must hand back the same record for a key, or two callers would
// serialise on different mutexes and both drive the same process.
func TestAcquireReturnsStableRecord(t *testing.T) {
	m := newManager()
	first := m.acquire("naive-1")
	second := m.acquire("naive-1")
	if first != second {
		t.Fatal("acquire returned two different records for one key")
	}
	if other := m.acquire("naive-2"); other == first {
		t.Fatal("acquire returned the same record for two different keys")
	}
}

// proc is created with the slot and never reassigned, which is what lets the
// read-only accessors reach it under mu while a lifecycle operation runs under
// opMu.
func TestAcquireCreatesProcEagerly(t *testing.T) {
	m := newManager()
	mc := m.acquire("trusttunnel-4")
	if mc.proc == nil {
		t.Fatal("acquire left proc nil, want an idle supervised process")
	}
	if mc.proc.IsRunning() {
		t.Fatal("a freshly created proc reports itself running")
	}
	before := mc.proc
	if again := m.acquire("trusttunnel-4"); again.proc != before {
		t.Fatal("proc was reassigned on a second acquire")
	}
}

// Removing a key must not disturb its neighbours: a panel with several tunnel
// inbounds deletes one of them all the time.
func TestRemoveOnlyDropsItsOwnKey(t *testing.T) {
	m := newManager()
	kept := m.acquire("mieru-1")
	m.acquire("mieru-2")

	m.Remove("mieru-2")

	m.mu.Lock()
	_, goneStillThere := m.cores["mieru-2"]
	survivor, keptStillThere := m.cores["mieru-1"]
	m.mu.Unlock()

	if goneStillThere {
		t.Error("Remove left the removed key in the map")
	}
	if !keptStillThere || survivor != kept {
		t.Error("Remove disturbed an unrelated key")
	}
}

func TestRemoveIgnoresBlankKey(t *testing.T) {
	m := newManager()
	m.acquire("naive-1")
	m.Remove("")
	m.Remove("   ")
	m.mu.Lock()
	n := len(m.cores)
	m.mu.Unlock()
	if n != 1 {
		t.Fatalf("cores holds %d keys after removing a blank one, want 1", n)
	}
}

// StopKey and the read accessors take different locks by design. Running them
// against one another catches a regression where a lifecycle path starts
// touching the maps without mu again (`go test -race` is where this bites).
func TestConcurrentLifecycleAndReads(t *testing.T) {
	m := newManager()
	keys := []string{"naive-1", "olcrtc-2", "qwdtt", "mieru-3", "trusttunnel-4"}
	for _, key := range keys {
		m.acquire(key)
	}

	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			for range 50 {
				if err := m.StopKey(key); err != nil {
					t.Errorf("StopKey(%s) on an idle slot = %v, want nil", key, err)
					return
				}
			}
		}(key)

		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			for range 50 {
				m.IsRunningKey(key)
				m.LastLogKey(key)
				m.LogsKey(key, 10)
				m.AnyRunning("naive")
				m.LogsPrefixed("mieru", 10)
			}
		}(key)
	}
	wg.Wait()
}

// StopAll stops the cores in parallel; on an idle manager it must still return
// promptly and leave every fingerprint cleared.
func TestStopAllClearsFingerprints(t *testing.T) {
	m := newManager()
	for _, key := range []string{"naive-1", "olcrtc-2"} {
		mc := m.acquire(key)
		mc.fp = "stale"
	}

	m.StopAll()

	m.mu.Lock()
	defer m.mu.Unlock()
	for key, mc := range m.cores {
		if mc.fp != "" {
			t.Errorf("StopAll left fingerprint %q on %s", mc.fp, key)
		}
	}
}

// The sweep SIGKILLs anything named like a core binary, so it must stay off
// for manager instances built by tests.
func TestSweepDisabledOutsideSharedManager(t *testing.T) {
	m := newManager()
	if m.sweepEnabled {
		t.Fatal("newManager enabled the orphan sweep; only GetManager may")
	}
	m.sweepOrphans()
	m.mu.Lock()
	swept := m.swept
	m.mu.Unlock()
	if swept {
		t.Fatal("sweepOrphans marked a disabled sweep as done")
	}
}
