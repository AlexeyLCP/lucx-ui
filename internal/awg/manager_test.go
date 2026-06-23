// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"
)

// TestGetManager_Singleton verifies the Manager is a process-wide singleton:
// repeated calls return the same pointer.
func TestGetManager_Singleton(t *testing.T) {
	m1 := GetManager()
	m2 := GetManager()
	if m1 != m2 {
		t.Fatal("GetManager must return the same pointer")
	}
}

// TestManager_StopAllOnEmpty verifies StopAll is safe on an empty manager
// (no goroutines, no panic). Covers the shutdown path when no AWG inbounds
// exist yet.
func TestManager_StopAll_OnEmpty(t *testing.T) {
	m := GetManager()
	m.StopAll()
	if len(m.procs) != 0 {
		t.Fatalf("expected 0 procs after StopAll, got %d", len(m.procs))
	}
}

// TestManager_RemoveMissing verifies Remove is a no-op for an unknown id
// (must not panic or add state).
func TestManager_Remove_Missing(t *testing.T) {
	m := GetManager()
	before := len(m.procs)
	m.Remove(999999)
	if len(m.procs) != before {
		t.Fatalf("Remove of missing id must not change state: before=%d after=%d", before, len(m.procs))
	}
}

// TestManager_CollectTraffic_Empty verifies CollectTraffic returns nil with
// no running interfaces (the cron job calls this every 10s).
func TestManager_CollectTraffic_Empty(t *testing.T) {
	m := GetManager()
	out := m.CollectTraffic()
	if out != nil && len(out) != 0 {
		t.Fatalf("expected nil/empty from CollectTraffic on empty manager, got %v", out)
	}
}

// TestManager_Reconcile_EmptyDesired verifies Reconcile with an empty desired
// set is safe and leaves the manager empty. Does not exercise awg-quick
// (which requires Linux + the amneziawg kernel module); this guards the
// state-machine bookkeeping only.
func TestManager_Reconcile_EmptyDesired(t *testing.T) {
	m := GetManager()
	m.Reconcile(nil)
	if len(m.procs) != 0 {
		t.Fatalf("expected 0 procs after Reconcile(nil), got %d", len(m.procs))
	}
}