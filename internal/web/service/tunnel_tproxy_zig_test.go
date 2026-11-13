// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

// TestReconcileTproxyInboundsRestoresMtprotoZig guards issue #201: the
// MTProto-Zig WEB backend is reconciled like the classic tproxy stack, so the
// panel revives it after a restart. Before the fix the zig instance fell
// through the core switch of reconcileTproxyInbounds and was never started
// (only a manual inbound save did it, through the runtime apply path).
func TestReconcileTproxyInboundsRestoresMtprotoZig(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	binDir := filepath.Join(dbDir, "bin")
	t.Setenv("XUI_BIN_FOLDER", binDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	settings, err := json.Marshal(map[string]any{
		"backend":  "mtproto-zig",
		"hostname": "example.com",
		"secret":   "0123456789abcdef0123456789abcdef",
		"port":     52888,
	})
	if err != nil {
		t.Fatal(err)
	}
	ib := &model.Inbound{
		Remark:   "tg-web-zig",
		Tag:      "tgwebzigin",
		Protocol: model.Tproxy,
		Port:     52888,
		Enable:   true,
		Settings: string(settings),
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	(&TunnelService{}).reconcileTproxyInbounds()

	cfgPath := filepath.Join(binDir, "tunnel", tunnel.MtprotoZigKey(ib.Id)+".conf")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("reconcile must render the MTProto-Zig config of inbound %d: %v", ib.Id, err)
	}
}
