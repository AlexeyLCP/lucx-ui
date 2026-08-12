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

// QwdttKey is the single manager key for the panel's qWDTT inbound
// (multi-instance is not supported: TUN + multi-port + root).
const QwdttKey = "qwdtt"

// QwdttConfigFromInbound maps an inbound row to QwdttConfig.
func QwdttConfigFromInbound(ib *model.Inbound) (QwdttConfig, bool) {
	if ib == nil || ib.Protocol != model.Qwdtt {
		return QwdttConfig{}, false
	}
	cfg := DefaultQwdttConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
		// encoding/json zeroes bool when the key is absent — restore default true
		// so pre-routeThroughXray rows and bare saves still egress via Xray.
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
	// Prefer inbound.Port as DTLS listen port when settings listen is default
	// and inbound port is set (panel form may only set Port).
	if ib.Port > 0 {
		if host, p, err := net.SplitHostPort(cfg.ListenAddr); err == nil {
			if p == "56000" || p == "0" {
				cfg.ListenAddr = net.JoinHostPort(host, strconv.Itoa(ib.Port))
			}
		}
	}
	return cfg.Merge(), true
}

// QwdttInstanceFromInbound builds a supervised Instance for the single qWDTT
// inbound. Always uses QwdttKey.
func QwdttInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := QwdttConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	if !ib.Enable {
		return Instance{Core: Qwdtt, Key: QwdttKey, Enabled: false}, true
	}
	if err := cfg.Validate(); err != nil {
		return Instance{Core: Qwdtt, Key: QwdttKey, Enabled: false}, true
	}
	if strings.TrimSpace(cfg.Password) == "" {
		if c2, err := cfg.EnsurePassword(); err == nil {
			cfg = c2
		}
	}
	// Per-inbound state dir under multi-key layout (even for single key).
	if strings.TrimSpace(cfg.ConfigDir) == "" {
		cfg.ConfigDir = dataDirFor(QwdttKey, Qwdtt)
	}
	inst := Instance{
		Core:      Qwdtt,
		Key:       QwdttKey,
		Enabled:   true,
		Args:      cfg.BuildArgs(),
		ProbePort: 0,
	}
	if cfg.RouteThroughXray {
		inst.RouteThroughXray = true
		inst.TunName = QwdttTunName(ib.Id)
		inst.RouteTable = QwdttRouteTable(ib.Id)
		inst.RouteIfaces = []string{qwdttIfaceWG, qwdttIfaceRaw}
	}
	return inst, true
}

// QwdttDTLSPort returns the DTLS listen port from config (for inbound.Port).
func QwdttDTLSPort(cfg QwdttConfig) int {
	if _, port, err := net.SplitHostPort(cfg.ListenAddr); err == nil {
		if p, err := strconv.Atoi(port); err == nil {
			return p
		}
	}
	return 56000
}
