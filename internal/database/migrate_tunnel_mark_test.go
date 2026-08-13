// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func setupTunnelMigrateDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: glogger.Default.LogMode(glogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&model.User{}, &model.Inbound{}, &model.Setting{}); err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&model.User{Username: "admin", Password: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	prev := db
	db = gdb
	t.Cleanup(func() { db = prev })
	return gdb
}

func blobMarker(t *testing.T, gdb *gorm.DB, key string) bool {
	t.Helper()
	var setting model.Setting
	if err := gdb.Where("key = ?", key).First(&setting).Error; err != nil {
		t.Fatalf("read blob %s: %v", key, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		t.Fatalf("parse blob %s: %v", key, err)
	}
	v, _ := cfg["migratedToInbound"].(bool)
	return v
}

// An inbound already owns the protocol (created manually before the migration
// existed): the blob must be marked so the reconcile fallback never
// resurrects it after the inbound is deleted, and no second inbound appears.
func TestMigrateTunnelMarksBlobWhenInboundExists(t *testing.T) {
	gdb := setupTunnelMigrateDB(t, "tunnel_mark_existing")
	if err := gdb.Create(&model.Inbound{
		UserId: 1, Remark: "manual", Protocol: model.Olcrtc, Tag: "inbound-olcrtc-1",
		Enable: true, Settings: `{"roomId":"r"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&model.Setting{
		Key: "lucxTunnel_olcrtc", Value: `{"enabled":true,"roomId":"r"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	migrateOlcrtcTunnelToInbound()

	var count int64
	if err := gdb.Model(&model.Inbound{}).Where("protocol = ?", model.Olcrtc).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("olcrtc inbound count = %d, want 1 (no duplicate)", count)
	}
	if !blobMarker(t, gdb, "lucxTunnel_olcrtc") {
		t.Fatal("legacy blob must be marked when an inbound already exists")
	}
}

// No inbound yet: the blob is promoted (existing behavior preserved).
func TestMigrateTunnelPromotesBlobWhenNoInbound(t *testing.T) {
	gdb := setupTunnelMigrateDB(t, "tunnel_mark_promote")
	if err := gdb.Create(&model.Setting{
		Key: "lucxTunnel_olcrtc", Value: `{"enabled":true,"roomId":"r","provider":"jitsi"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	migrateOlcrtcTunnelToInbound()

	var ib model.Inbound
	if err := gdb.Where("protocol = ?", model.Olcrtc).First(&ib).Error; err != nil {
		t.Fatalf("promoted inbound not found: %v", err)
	}
	if !ib.Enable {
		t.Error("promoted inbound must keep the blob enabled state")
	}
	if strings.Contains(ib.Settings, "migratedToInbound") {
		t.Error("inbound settings must not carry the migration marker")
	}
	if !blobMarker(t, gdb, "lucxTunnel_olcrtc") {
		t.Fatal("promoted blob must be marked")
	}
}

// Already-marked blob: idempotent, nothing changes.
func TestMigrateTunnelSkipsMarkedBlob(t *testing.T) {
	gdb := setupTunnelMigrateDB(t, "tunnel_mark_marked")
	if err := gdb.Create(&model.Setting{
		Key: "lucxTunnel_qwdtt", Value: `{"enabled":true,"migratedToInbound":true,"migratedInboundId":7}`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	migrateQwdttTunnelToInbound()

	var count int64
	if err := gdb.Model(&model.Inbound{}).Where("protocol = ?", model.Qwdtt).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("qwdtt inbound count = %d, want 0 (marked blob must not re-promote)", count)
	}
}

func TestMigrateNaiveMarksBlobWhenInboundExists(t *testing.T) {
	gdb := setupTunnelMigrateDB(t, "tunnel_mark_naive")
	if err := gdb.Create(&model.Inbound{
		UserId: 1, Remark: "manual", Protocol: model.Naive, Tag: "inbound-naive-1",
		Enable: true, Port: 443, Settings: `{}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&model.Setting{
		Key: "lucxTunnel_naive", Value: `{"enabled":true,"domain":"example.org"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	migrateNaiveTunnelToInbound()

	var count int64
	if err := gdb.Model(&model.Inbound{}).Where("protocol = ?", model.Naive).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("naive inbound count = %d, want 1 (no duplicate)", count)
	}
	if !blobMarker(t, gdb, "lucxTunnel_naive") {
		t.Fatal("legacy naive blob must be marked when an inbound already exists")
	}
}
