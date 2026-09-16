// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const CsqttKey = "csqtt"

func CsqttConfigFromInbound(ib *model.Inbound) (CsqttConfig, bool) {
	if ib == nil || ib.Protocol != model.Csqtt {
		return CsqttConfig{}, false
	}
	cfg := DefaultCsqttConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
		var keys map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &keys) == nil {
			if _, ok := keys["routeThroughXray"]; !ok {
				cfg.RouteThroughXray = true
			}
		}
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	if ib.Port > 0 {
		if host, p, err := net.SplitHostPort(cfg.ListenAddr); err == nil {
			if p == "46000" || p == "0" {
				cfg.ListenAddr = net.JoinHostPort(host, strconv.Itoa(ib.Port))
			}
		}
	}
	return cfg.Merge(), true
}

func CsqttInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := CsqttConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	if !ib.Enable {
		return Instance{Core: Csqtt, Key: CsqttKey, Enabled: false}, true
	}
	if err := cfg.Validate(); err != nil {
		return Instance{Core: Csqtt, Key: CsqttKey, Enabled: false}, true
	}
	if strings.TrimSpace(cfg.Password) == "" {
		if c2, err := cfg.EnsurePassword(); err == nil {
			cfg = c2
		}
	}
	if strings.TrimSpace(cfg.WebPass) == "" {
		if c2, err := cfg.EnsureWebPass(); err == nil {
			cfg = c2
		}
	}
	if strings.TrimSpace(cfg.ConfigDir) == "" {
		cfg.ConfigDir = dataDirFor(CsqttKey, Csqtt)
	}
	inst := Instance{
		Core:    Csqtt,
		Key:     CsqttKey,
		Enabled: true,
		Args:    cfg.BuildArgs(),
	}
	if cfg.RouteThroughXray {
		inst.RouteThroughXray = true
		inst.TunName = CsqttTunName(ib.Id)
		inst.RouteTable = csqttRouteTable
		inst.RouteIfaces = []string{csqttIface}
	}
	return inst, true
}
