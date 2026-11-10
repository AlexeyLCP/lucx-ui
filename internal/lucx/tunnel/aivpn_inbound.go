// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const AivpnKey = "aivpn"

func AivpnConfigFromInbound(ib *model.Inbound) (AivpnConfig, bool) {
	if ib == nil || ib.Protocol != model.Aivpn {
		return AivpnConfig{}, false
	}
	cfg := DefaultAivpnConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	return cfg.Merge(), true
}

// AivpnInstanceFromInbound renders the server.json + mint-once server key
// into the work dir (single-instance key like csqtt/masterdns: one instance
// per host). The subnet is id-derived when unset, so re-renders never change
// an already-issued network config; the key file is created once and never
// touched again (Rule 0).
func AivpnInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := AivpnConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	if !ib.Enable {
		return Instance{Core: Aivpn, Key: AivpnKey, Enabled: false}, true
	}
	subnet, err := cfg.ResolveSubnet(ib.Id)
	if err != nil {
		logger.Warning("aivpn: inbound ", ib.Id, " subnet invalid: ", err)
		return Instance{Core: Aivpn, Key: AivpnKey, Enabled: false}, true
	}
	cfg.Subnet = subnet
	if err := cfg.Validate(ib.Port); err != nil {
		logger.Warning("aivpn: inbound ", ib.Id, " config invalid: ", err)
		return Instance{Core: Aivpn, Key: AivpnKey, Enabled: false}, true
	}
	serverIP := cfg.ServerIP
	if serverIP == "" {
		if ip := detectOutboundIPv4(); ip != "" {
			serverIP = ip
		}
	}
	if serverIP != "" {
		cfg.ServerIP = serverIP
	}
	if aivpnMasksMissing() {
		logger.Warning("aivpn: no mask profiles in ", aivpnMasksPath(),
			" — update the panel so the release ships them, or place mask JSON files there")
		return Instance{Core: Aivpn, Key: AivpnKey, Enabled: false}, true
	}
	if err := EnsureAivpnKeyFile(AivpnKey); err != nil {
		logger.Warning("aivpn: ensure server key failed: ", err)
		return Instance{Core: Aivpn, Key: AivpnKey, Enabled: false}, true
	}
	return Instance{
		Core:       Aivpn,
		Key:        AivpnKey,
		Enabled:    true,
		ConfigText: cfg.RenderConfig(AivpnKey, ib.Port, subnet),
		Args:       cfg.BuildArgs(AivpnKey),
	}, true
}
