// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func GatewayKey(id int) string { return "gateway-" + strconv.Itoa(id) }

func GatewayConfigFromInbound(ib *model.Inbound) (GatewayConfig, bool) {
	if ib == nil || ib.Protocol != model.Gateway {
		return GatewayConfig{}, false
	}
	cfg := DefaultGatewayConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	return cfg.Merge(), true
}

func GatewayInstanceFromInbound(ib *model.Inbound, others []*model.Inbound) (Instance, bool) {
	cfg, ok := GatewayConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := GatewayKey(ib.Id)
	disabled := Instance{Core: Gateway, Key: key, Enabled: false}
	if !cfg.Enabled || !cfg.Applied() {
		return disabled, true
	}
	port := ib.Port
	if port <= 0 {
		port = gatewayDefaultPort
	}
	bindIP := strings.TrimSpace(cfg.BindIP)
	rows := BuildPreview(port, cfg.PublicHost, others, bindIP)
	selected := map[int]bool{}
	for _, s := range cfg.Snapshot {
		selected[s.InboundID] = true
	}
	routes := cfg.Routes
	if len(routes) == 0 {
		routes = RoutesFromPreview(rows, selected)
	}
	if len(routes) == 0 {
		logger.Warningf("tunnel: gateway-%d disabled: no SNI routes", ib.Id)
		return disabled, true
	}
	confPath := configPathFor(key, Gateway)
	pidPath := filepath.Join(workDir(), key+".pid")
	fallback := cfg.Fallback
	if fallback == "" {
		fallback = CoverFallback(rows, selected)
	}
	conf := RenderNginxConf(port, absPath(pidPath), routes, fallback, bindIP)
	probe := port
	if bindIP != "" {
		probe = 0
	}
	return Instance{
		Core:       Gateway,
		Key:        key,
		Enabled:    true,
		ConfigText: conf,
		Args:       []string{"-g", "daemon off;", "-c", absPath(confPath)},
		ProbePort:  probe,
	}, true
}
