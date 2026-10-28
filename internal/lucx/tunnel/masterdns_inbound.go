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
)

const MasterdnsKey = "masterdns"

func MasterdnsConfigFromInbound(ib *model.Inbound) (MasterdnsConfig, bool) {
	if ib == nil || ib.Protocol != model.Masterdns {
		return MasterdnsConfig{}, false
	}
	cfg := DefaultMasterdnsConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	return cfg.Merge(), true
}

// MasterdnsInstanceFromInbound renders the server TOML + secret into the work
// dir (single-credential key like csqtt: one instance per host).
func MasterdnsInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := MasterdnsConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	if !ib.Enable {
		return Instance{Core: Masterdns, Key: MasterdnsKey, Enabled: false}, true
	}
	port := ib.Port
	if err := cfg.Validate(port); err != nil {
		return Instance{Core: Masterdns, Key: MasterdnsKey, Enabled: false}, true
	}
	if strings.TrimSpace(cfg.Secret) == "" {
		if c2, err := cfg.EnsureSecret(); err == nil {
			cfg = c2
		}
	}
	if err := syncMasterdnsFiles(cfg, MasterdnsKey, port); err != nil {
		return Instance{Core: Masterdns, Key: MasterdnsKey, Enabled: false}, true
	}
	return Instance{
		Core:    Masterdns,
		Key:     MasterdnsKey,
		Enabled: true,
		Args:    cfg.BuildArgs(MasterdnsKey),
	}, true
}
