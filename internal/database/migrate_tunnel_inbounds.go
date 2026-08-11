// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	tunnelOlcrtcSettingKey = "lucxTunnel_olcrtc"
	tunnelQwdttSettingKey  = "lucxTunnel_qwdtt"
)

// migrateOlcrtcTunnelToInbound promotes lucxTunnel_olcrtc → protocol=olcrtc inbound.
func migrateOlcrtcTunnelToInbound() {
	migrateTunnelSettingsToInbound(tunnelOlcrtcSettingKey, model.Olcrtc, "olcRTC", 0)
}

// migrateQwdttTunnelToInbound promotes lucxTunnel_qwdtt → protocol=qwdtt inbound.
func migrateQwdttTunnelToInbound() {
	migrateTunnelSettingsToInbound(tunnelQwdttSettingKey, model.Qwdtt, "qWDTT", 56000)
}

func migrateTunnelSettingsToInbound(settingKey string, proto model.Protocol, defaultRemark string, defaultPort int) {
	if db == nil {
		return
	}
	var count int64
	if err := db.Model(&model.Inbound{}).Where("protocol = ?", proto).Count(&count).Error; err != nil {
		logger.Warning("migrate ", proto, " inbound: count failed:", err)
		return
	}
	if count > 0 {
		return
	}
	var setting model.Setting
	if err := db.Where("key = ?", settingKey).First(&setting).Error; err != nil {
		return
	}
	if strings.TrimSpace(setting.Value) == "" {
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		logger.Warning("migrate ", proto, " inbound: corrupt JSON:", err)
		return
	}
	if v, ok := cfg["migratedToInbound"].(bool); ok && v {
		return
	}

	port := defaultPort
	if p, ok := cfg["port"].(float64); ok && p > 0 {
		port = int(p)
	}
	// qWDTT: parse listenAddr for port.
	if proto == model.Qwdtt {
		if la, ok := cfg["listenAddr"].(string); ok {
			if _, portStr, err := net.SplitHostPort(la); err == nil {
				if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
					port = p
				}
			}
		}
	}
	if proto == model.Olcrtc {
		port = 0
	}

	remark, _ := cfg["remark"].(string)
	if strings.TrimSpace(remark) == "" {
		remark = defaultRemark
	}
	enabled, _ := cfg["enabled"].(bool)
	delete(cfg, "enabled")

	bs, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logger.Warning("migrate ", proto, " inbound: marshal failed:", err)
		return
	}
	ib := &model.Inbound{
		Remark:   remark,
		Enable:   enabled,
		Port:     port,
		Protocol: proto,
		Settings: string(bs),
		Tag:      string(proto) + "-migrated",
	}
	if err := db.Create(ib).Error; err != nil {
		logger.Warning("migrate ", proto, " inbound: create failed:", err)
		return
	}
	ib.Tag = "inbound-" + string(proto) + "-" + strconv.Itoa(ib.Id)
	_ = db.Model(ib).Update("tag", ib.Tag).Error

	cfg["migratedToInbound"] = true
	cfg["migratedInboundId"] = ib.Id
	if marked, err := json.Marshal(cfg); err == nil {
		_ = db.Model(&model.Setting{}).Where("key = ?", settingKey).Update("value", string(marked)).Error
	}
	logger.Info("migrate ", proto, " inbound: promoted ", settingKey, " → inbound id=", ib.Id)
}
