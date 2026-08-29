// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGetSubs_Naive_RemarkAndHostPort(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const subId = "sub-naive"
	const email = "lesovoi_freedom"

	db := database.GetDB()
	ib := &model.Inbound{
		UserId:   1,
		Tag:      "naive-in",
		Enable:   true,
		Port:     8443,
		Remark:   "naive-in",
		Protocol: model.Naive,
		Settings: `{"domain":"n.example.org","certFile":"/c","keyFile":"/k",` +
			`"clients":[{"email":"` + email + `","enable":true}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	s := NewSubService("")
	links, _, _, _, err := s.GetSubs(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	got := strings.Join(links, "\n")
	if !strings.HasPrefix(got, "naive+https://") {
		t.Fatalf("want naive+https link, got %q", got)
	}
	if !strings.Contains(got, "@n.example.org:8443") {
		t.Errorf("no host: inbound port, got %q", got)
	}
	if !strings.Contains(got, "#naive-in-"+email) {
		t.Errorf("remark must be inbound-email, got %q", got)
	}

	if err := db.Create(&model.Host{
		InboundId: ib.Id,
		Remark:    "cdn",
		Address:   "cdn.example.com",
		Port:      443,
	}).Error; err != nil {
		t.Fatalf("seed host: %v", err)
	}
	links, _, _, _, err = s.GetSubs(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs with host: %v", err)
	}
	got = strings.Join(links, "\n")
	if !strings.Contains(got, "@cdn.example.com:443") {
		t.Errorf("host dest+port must win, got %q", got)
	}
	if strings.Contains(got, "n.example.org:8443") {
		t.Errorf("inbound listen port must not leak into the share URL, got %q", got)
	}
}

func TestGetSubs_Mieru_HostPort(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const subId = "sub-mieru"
	const email = "lesovoi_freedom"
	db := database.GetDB()
	ib := &model.Inbound{
		UserId:   1,
		Tag:      "mieru-in",
		Enable:   true,
		Port:     20100,
		Remark:   "mieru-in",
		Protocol: model.Mieru,
		Settings: `{"portBindings":[{"port":20100,"protocol":"TCP"}],` +
			`"clients":[{"email":"` + email + `","enable":true}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}
	if err := db.Create(&model.Host{
		InboundId: ib.Id,
		Remark:    "cdn",
		Address:   "cdn.example.com",
		Port:      443,
	}).Error; err != nil {
		t.Fatalf("seed host: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	got := strings.Join(links, "\n")
	if !strings.Contains(got, "mierus://") || !strings.Contains(got, "@cdn.example.com?") {
		t.Errorf("host dest must win, got %q", got)
	}
	if !strings.Contains(got, "port=443") {
		t.Errorf("host port must replace listen port, got %q", got)
	}
	if strings.Contains(got, "port=20100") {
		t.Errorf("inbound listen port must not leak, got %q", got)
	}
}

func TestGetSubs_TrustTunnel_HostPort(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const subId = "sub-tt-host"
	const email = "lesovoi_freedom"
	db := database.GetDB()
	ib := &model.Inbound{
		UserId:   1,
		Tag:      "tt-in",
		Enable:   true,
		Port:     8443,
		Remark:   "tt-in",
		Protocol: model.TrustTunnel,
		Settings: `{"hostname":"tt.example.com","listen":"0.0.0.0:8443","upstreamProtocol":"http2",` +
			`"clients":[{"email":"` + email + `","enable":true}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}
	if err := db.Create(&model.Host{
		InboundId: ib.Id,
		Remark:    "cdn",
		Address:   "cdn.example.com",
		Port:      443,
	}).Error; err != nil {
		t.Fatalf("seed host: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	got := strings.Join(links, "\n")
	if !strings.Contains(got, "@cdn.example.com:443") {
		t.Errorf("host dest+port must win, got %q", got)
	}
	if strings.Contains(got, ":8443") {
		t.Errorf("inbound listen port must not leak, got %q", got)
	}
}
