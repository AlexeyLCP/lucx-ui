// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Андрей (382 report): a gateway applied BEFORE the subscription was enabled
// (or without a sub route) kept HidePanel=true, and gatewayHidesPanel() then
// rewrote every subscription link to scheme://host without the sub port —
// dead links on panels that still serve the sub on 2096. The port may be
// hidden only when the applied PanelRoutes actually carry the sub path.
func gatewayRow(t *testing.T, panelRoutes string) {
	t.Helper()
	db := database.GetDB()
	settings := `{"hostname":"g","hidePanel":true,"snapshot":[{}]`
	if panelRoutes != "" {
		settings += `,"panelRoutes":` + panelRoutes
	}
	settings += `}`
	ib := &model.Inbound{UserId: 1, Tag: "gw", Enable: true, Port: 443, Protocol: model.Gateway, Settings: settings}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create gateway inbound: %v", err)
	}
}

func setSubSettings(t *testing.T, subPort int) {
	t.Helper()
	db := database.GetDB()
	rows := []model.Setting{
		{Key: "subPort", Value: strconv.Itoa(subPort)},
		{Key: "subEnable", Value: "true"},
		{Key: "subPath", Value: "/sub/"},
	}
	for _, r := range rows {
		if err := db.Where(model.Setting{Key: r.Key}).Assign(model.Setting{Value: r.Value}).FirstOrCreate(&r).Error; err != nil {
			t.Fatalf("set %s: %v", r.Key, err)
		}
	}
}

// TestBuildSubURIBase_HidesSubOnlyWhenActuallyRouted pins the split: HidePanel
// without a sub route keeps the sub port; with the sub route in the applied
// PanelRoutes the link goes scheme://host (port 443 omitted).
func TestBuildSubURIBase_HidesSubOnlyWhenActuallyRouted(t *testing.T) {
	setupConflictDB(t)
	setSubSettings(t, 2096)
	const host = "wrr1.antizwit.online"

	gatewayRow(t, `[{"path":"/panel-base/","dest":"http://127.0.0.1:22150"}]`)
	base := (&SettingService{}).BuildSubURIBase(host)
	if !strings.HasSuffix(base, ":2096") {
		t.Fatalf("panel-only HidePanel must keep the sub port, base = %q", base)
	}

	db := database.GetDB()
	routes := `[{"path":"/panel-base/","dest":"http://127.0.0.1:22150"},{"path":"/sub/","dest":"https://127.0.0.1:2096"}]`
	if err := db.Model(&model.Inbound{}).Where("protocol = ?", model.Gateway).
		Update("settings", `{"hostname":"g","hidePanel":true,"snapshot":[{}],"panelRoutes":`+routes+`}`).Error; err != nil {
		t.Fatalf("update gateway routes: %v", err)
	}
	base = (&SettingService{}).BuildSubURIBase(host)
	if strings.Contains(base, "2096") {
		t.Fatalf("sub routed behind the gateway must omit the port, base = %q", base)
	}
	if !strings.HasPrefix(base, "https://"+host) {
		t.Fatalf("sub behind gateway must be https base = %q", base)
	}
}

func TestGatewayHidesSub_TruthTable(t *testing.T) {
	setupConflictDB(t)
	setSubSettings(t, 2096)

	gatewayRow(t, "") // snapshot, hidePanel, no panelRoutes
	if gatewayHidesSub() {
		t.Fatal("no panelRoutes at all: sub must not be considered hidden")
	}

	db := database.GetDB()
	replaceRoutes := func(routes string) {
		t.Helper()
		if err := db.Model(&model.Inbound{}).Where("protocol = ?", model.Gateway).
			Update("settings", `{"hostname":"g","hidePanel":true,"snapshot":[{}],"panelRoutes":`+routes+`}`).Error; err != nil {
			t.Fatalf("update gateway routes: %v", err)
		}
	}
	replaceRoutes(`[{"path":"/x/","dest":"http://127.0.0.1:22150"}]`)
	if gatewayHidesSub() {
		t.Fatal("panel-only routes: sub must not be considered hidden")
	}
	replaceRoutes(`[{"path":"/sub/","dest":"https://127.0.0.1:2096"}]`)
	if !gatewayHidesSub() {
		t.Fatal("applied sub route: sub IS hidden behind the gateway")
	}
}
