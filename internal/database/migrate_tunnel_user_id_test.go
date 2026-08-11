// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRepairOrphanTunnelInboundUserIDs(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:orphan_uid?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&model.User{}, &model.Inbound{}); err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&model.User{Username: "admin", Password: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	var admin model.User
	if err := gdb.First(&admin).Error; err != nil {
		t.Fatal(err)
	}

	orphans := []*model.Inbound{
		{UserId: 0, Remark: "n", Port: 443, Protocol: model.Naive, Tag: "n0", Settings: `{}`},
		{UserId: 0, Remark: "o", Port: 0, Protocol: model.Olcrtc, Tag: "o0", Settings: `{}`},
		{UserId: 0, Remark: "q", Port: 56000, Protocol: model.Qwdtt, Tag: "q0", Settings: `{}`},
		{UserId: admin.Id, Remark: "ok", Port: 8443, Protocol: model.Naive, Tag: "n1", Settings: `{}`},
		{UserId: 0, Remark: "vless", Port: 10000, Protocol: model.VLESS, Tag: "v0", Settings: `{}`},
	}
	for _, ib := range orphans {
		if err := gdb.Create(ib).Error; err != nil {
			t.Fatal(err)
		}
	}

	prev := db
	db = gdb
	t.Cleanup(func() { db = prev })

	repairOrphanTunnelInboundUserIDs()

	var fixed []model.Inbound
	if err := gdb.Where("protocol IN ?", []model.Protocol{model.Naive, model.Olcrtc, model.Qwdtt}).Find(&fixed).Error; err != nil {
		t.Fatal(err)
	}
	for _, ib := range fixed {
		if ib.UserId != admin.Id {
			t.Fatalf("inbound %s user_id=%d want %d", ib.Tag, ib.UserId, admin.Id)
		}
	}
	var vless model.Inbound
	if err := gdb.Where("tag = ?", "v0").First(&vless).Error; err != nil {
		t.Fatal(err)
	}
	if vless.UserId != 0 {
		t.Fatalf("non-tunnel orphan should stay user_id=0, got %d", vless.UserId)
	}
}

func TestFirstPanelUserID_Fallback(t *testing.T) {
	prev := db
	db = nil
	t.Cleanup(func() { db = prev })
	if got := firstPanelUserID(); got != 1 {
		t.Fatalf("nil db fallback = %d, want 1", got)
	}
}
