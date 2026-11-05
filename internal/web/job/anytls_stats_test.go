// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

func anytlsTestStats(started int64, users map[string][2]int64, activeIP string) tunnel.AnytlsStats {
	st := tunnel.AnytlsStats{Started: started, Users: map[string]tunnel.AnytlsUserStats{}}
	for name, c := range users {
		u := tunnel.AnytlsUserStats{Up: c[0], Down: c[1]}
		if activeIP != "" {
			u.IPs = map[string]struct {
				Active int   `json:"active"`
				Last   int64 `json:"last"`
			}{activeIP: {Active: 1, Last: started}}
		}
		st.Users[name] = u
	}
	return st
}

func TestAnytlsUserDeltas(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	started := anytlsPanelStart + 10 // the server started after this panel
	emails := []string{"a@x", "b@x"}
	// First read of a server started after the panel: everything counts.
	ct, online := anytlsUserDeltas(901, "t", anytlsTestStats(started, map[string][2]int64{"a@x": {100, 1000}, "": {5, 5}}, "203.0.113.7"), emails)
	if len(ct) != 1 || ct[0].Email != "a@x" || ct[0].Up != 100 || ct[0].Down != 1000 || len(online) != 1 {
		t.Fatalf("first read: %+v %v (the shared user has no sole client here)", ct, online)
	}
	// Second read: only the difference.
	ct, _ = anytlsUserDeltas(901, "t", anytlsTestStats(started, map[string][2]int64{"a@x": {150, 1600}, "b@x": {1, 2}}, ""), emails)
	got := map[string][2]int64{}
	for _, c := range ct {
		got[c.Email] = [2]int64{c.Up, c.Down}
	}
	if got["a@x"] != [2]int64{50, 600} || got["b@x"] != [2]int64{1, 2} {
		t.Fatalf("second read: %v", got)
	}
	// Server restart: counters start again from zero.
	ct, _ = anytlsUserDeltas(901, "t", anytlsTestStats(started+60, map[string][2]int64{"a@x": {7, 8}}, ""), emails)
	if len(ct) != 1 || ct[0].Up != 7 || ct[0].Down != 8 {
		t.Fatalf("after restart: %+v", ct)
	}
	// A server already running when the panel started: the first read is the baseline.
	ct, _ = anytlsUserDeltas(902, "t", anytlsTestStats(anytlsPanelStart-100, map[string][2]int64{"a@x": {500, 500}}, ""), emails)
	if len(ct) != 0 {
		t.Fatalf("baseline read credited %+v", ct)
	}
	// The shared password is credited to the inbound's only client.
	ct, _ = anytlsUserDeltas(903, "t", anytlsTestStats(started, map[string][2]int64{"": {3, 4}}, ""), []string{"solo@x"})
	if len(ct) != 1 || ct[0].Email != "solo@x" {
		t.Fatalf("shared user with sole client: %+v", ct)
	}
	// Users no longer attached are dropped.
	ct, _ = anytlsUserDeltas(904, "t", anytlsTestStats(started, map[string][2]int64{"gone@x": {3, 4}}, ""), emails)
	if len(ct) != 0 {
		t.Fatalf("detached user credited: %+v", ct)
	}
}
