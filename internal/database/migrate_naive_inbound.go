// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const tunnelNaiveSettingKey = "lucxTunnel_naive"

// migrateNaiveTunnelToInbound promotes the legacy global lucxTunnel_naive
// settings blob into one protocol=naive inbound row (mtproto model). Idempotent:
// skips when any naive inbound already exists or the settings key is empty.
// Clients are NOT auto-attached — operator attaches on the Clients page.
func migrateNaiveTunnelToInbound() {
	if db == nil {
		return
	}
	var count int64
	if err := db.Model(&model.Inbound{}).Where("protocol = ?", model.Naive).Count(&count).Error; err != nil {
		logger.Warning("migrate naive inbound: count failed:", err)
		return
	}
	if count > 0 {
		return
	}
	var setting model.Setting
	if err := db.Where("key = ?", tunnelNaiveSettingKey).First(&setting).Error; err != nil {
		return
	}
	if strings.TrimSpace(setting.Value) == "" {
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		logger.Warning("migrate naive inbound: corrupt settings JSON:", err)
		return
	}
	// Already migrated marker.
	if v, ok := cfg["migratedToInbound"].(bool); ok && v {
		return
	}

	port := 443
	if p, ok := cfg["port"].(float64); ok && p > 0 {
		port = int(p)
	}
	listen, _ := cfg["listen"].(string)
	remark, _ := cfg["remark"].(string)
	if strings.TrimSpace(remark) == "" {
		remark = "NaiveProxy"
	}
	enabled, _ := cfg["enabled"].(bool)

	// Settings for the inbound: drop top-level enabled (lives on inbound.Enable).
	delete(cfg, "enabled")
	cfg["clients"] = []any{}
	bs, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logger.Warning("migrate naive inbound: marshal settings failed:", err)
		return
	}

	ib := &model.Inbound{
		UserId:   firstPanelUserID(),
		Remark:   remark,
		Enable:   enabled,
		Listen:   listen,
		Port:     port,
		Protocol: model.Naive,
		Settings: string(bs),
		Tag:      "naive-migrated",
	}
	if err := db.Create(ib).Error; err != nil {
		logger.Warning("migrate naive inbound: create failed:", err)
		return
	}
	// Unique tag with id.
	ib.Tag = "inbound-naive-" + strconv.Itoa(ib.Id)
	_ = db.Model(ib).Update("tag", ib.Tag).Error

	cfg["migratedToInbound"] = true
	cfg["migratedInboundId"] = ib.Id
	if marked, err := json.Marshal(cfg); err == nil {
		_ = db.Model(&model.Setting{}).Where("key = ?", tunnelNaiveSettingKey).
			Update("value", string(marked)).Error
	}
	logger.Info("migrate naive inbound: promoted lucxTunnel_naive → inbound id=", ib.Id,
		" (attach clients on the Clients page)")
}
