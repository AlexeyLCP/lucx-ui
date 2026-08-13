// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var tunnelReconcileLoggerOnce sync.Once

func setupTunnelReconcileDB(t *testing.T) {
	t.Helper()
	tunnelReconcileLoggerOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })

	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Logf("CloseDB warning: %v", err)
		}
	})
}

func seedTunnelBlob(t *testing.T, key, value string) {
	t.Helper()
	if err := database.GetDB().Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
		t.Fatalf("seed setting %s: %v", key, err)
	}
}

func TestTunnelBlobMigrated(t *testing.T) {
	setupTunnelReconcileDB(t)
	s := &TunnelService{}

	if s.tunnelBlobMigrated(tunnelOlcrtcSettingKey) {
		t.Error("missing blob must not count as migrated")
	}

	seedTunnelBlob(t, tunnelOlcrtcSettingKey, `{"enabled":true,"roomId":"r"}`)
	if s.tunnelBlobMigrated(tunnelOlcrtcSettingKey) {
		t.Error("blob without the marker must not count as migrated")
	}

	seedTunnelBlob(t, tunnelNaiveSettingKey, `{"enabled":true,"migratedToInbound":true,"migratedInboundId":4}`)
	if !s.tunnelBlobMigrated(tunnelNaiveSettingKey) {
		t.Error("marked blob must count as migrated")
	}

	seedTunnelBlob(t, tunnelQwdttSettingKey, `{corrupt`)
	if s.tunnelBlobMigrated(tunnelQwdttSettingKey) {
		t.Error("corrupt blob must not count as migrated")
	}
}

func TestLegacyLifecycleBlockedAfterMigration(t *testing.T) {
	setupTunnelReconcileDB(t)
	s := &TunnelService{}

	seedTunnelBlob(t, tunnelOlcrtcSettingKey, `{"enabled":true,"roomId":"r","migratedToInbound":true}`)
	if err := s.StartOlcrtc(); err == nil || !strings.Contains(err.Error(), "Inbounds page") {
		t.Errorf("StartOlcrtc on a migrated blob must be refused, got: %v", err)
	}
	if err := s.RestartOlcrtc(); err == nil || !strings.Contains(err.Error(), "Inbounds page") {
		t.Errorf("RestartOlcrtc on a migrated blob must be refused, got: %v", err)
	}
	if err := s.StopOlcrtc(); err != nil {
		t.Errorf("StopOlcrtc must stay allowed (zombie kill), got: %v", err)
	}

	seedTunnelBlob(t, tunnelNaiveSettingKey, `{"enabled":true,"migratedToInbound":true}`)
	if _, err := s.StartNaive(); err == nil || !strings.Contains(err.Error(), "Inbounds page") {
		t.Errorf("StartNaive on a migrated blob must be refused, got: %v", err)
	}

	seedTunnelBlob(t, tunnelQwdttSettingKey, `{"enabled":true,"migratedToInbound":true}`)
	if err := s.StartQwdtt(); err == nil || !strings.Contains(err.Error(), "Inbounds page") {
		t.Errorf("StartQwdtt on a migrated blob must be refused, got: %v", err)
	}
}

func TestLegacyLifecycleBlockedWhileInboundExists(t *testing.T) {
	setupTunnelReconcileDB(t)
	s := &TunnelService{}

	ib := &model.Inbound{
		UserId:   1,
		Remark:   "olcRTC test",
		Enable:   true,
		Protocol: model.Olcrtc,
		Tag:      "inbound-olcrtc-1",
		Settings: `{"roomId":"r"}`,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	if err := s.StartOlcrtc(); err == nil || !strings.Contains(err.Error(), "managed by inbound") {
		t.Errorf("StartOlcrtc while an olcrtc inbound exists must be refused, got: %v", err)
	}
}

func TestLegacyLifecycleAllowedForLegacyOnlyHost(t *testing.T) {
	setupTunnelReconcileDB(t)
	s := &TunnelService{}

	err := s.StartOlcrtc()
	if err == nil {
		t.Fatal("StartOlcrtc with an empty blob must fail on validation, not on the gate")
	}
	if strings.Contains(err.Error(), "Inbounds page") || strings.Contains(err.Error(), "managed by inbound") {
		t.Errorf("legacy-only host must pass the lifecycle gate, got: %v", err)
	}
}
