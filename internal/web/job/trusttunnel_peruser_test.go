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

func TestTrustTunnelPerUser(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	d := tunnel.TrustTunnelTraffic{Tag: "tt", PerUser: true, Users: []tunnel.TrustTunnelUserTraffic{
		{User: "tu1", Up: 10, Down: 20, Sessions: 1, IP: "203.0.113.7"},
		{User: "tu2", Sessions: 1, IP: "127.0.0.1"},
		{User: "tu3"},
		{User: "gone", Up: 5, Sessions: 1},
	}}
	ct, online := trustTunnelPerUser(d, map[string]string{"tu1": "a@x", "tu2": "b@x", "tu3": "c@x"})
	if len(ct) != 1 || ct[0].Email != "a@x" || ct[0].Up != 10 || ct[0].Down != 20 {
		t.Fatalf("traffic = %+v", ct)
	}
	if len(online) != 2 || online[0] != "a@x" || online[1] != "b@x" {
		t.Fatalf("online = %v (idle and unknown users must not count)", online)
	}
}
