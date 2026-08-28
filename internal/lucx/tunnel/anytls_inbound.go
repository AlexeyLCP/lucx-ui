// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// AnytlsKey is the manager key of one AnyTls inbound instance.
func AnytlsKey(id int) string {
	return "anytls-" + strconv.Itoa(id)
}

// AnytlsConfigFromInbound maps an inbound row to AnytlsConfig.
func AnytlsConfigFromInbound(ib *model.Inbound) (AnytlsConfig, bool) {
	if ib == nil || ib.Protocol != model.Anytls {
		return AnytlsConfig{}, false
	}
	cfg := DefaultAnytlsConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	cfg = cfg.Merge()
	if ib.Port > 0 {
		cfg.Port = ib.Port
	}
	return cfg, true
}

// AnytlsInstanceFromInbound builds a supervised Instance for one AnyTls
// inbound. Disabled or invalid rows yield Enabled:false so reconcile
// converges them down.
func AnytlsInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := AnytlsConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := AnytlsKey(ib.Id)
	if !cfg.Enabled {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	if err := cfg.Validate(); err != nil {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	return Instance{
		Core:      Anytls,
		Key:       key,
		Enabled:   true,
		Args:      cfg.BuildArgs(),
		ProbePort: cfg.Port,
	}, true
}

// AnytlsPrimaryPort returns the TCP listen port for inbound.Port.
func AnytlsPrimaryPort(cfg AnytlsConfig) int {
	if cfg.Port > 0 {
		return cfg.Port
	}
	return 8443
}
