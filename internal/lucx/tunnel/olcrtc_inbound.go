// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// OlcrtcKey returns the manager key for an olcRTC inbound id.
func OlcrtcKey(inboundId int) string {
	return fmt.Sprintf("olcrtc-%d", inboundId)
}

// OlcrtcConfigFromInbound maps an inbound row to OlcrtcConfig.
func OlcrtcConfigFromInbound(ib *model.Inbound) (OlcrtcConfig, bool) {
	if ib == nil || ib.Protocol != model.Olcrtc {
		return OlcrtcConfig{}, false
	}
	cfg := DefaultOlcrtcConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	return cfg.Merge(), true
}

// OlcrtcInstanceFromInbound builds a supervised Instance for an olcRTC inbound.
func OlcrtcInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := OlcrtcConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := OlcrtcKey(ib.Id)
	if !ib.Enable {
		return Instance{Core: Olcrtc, Key: key, Enabled: false}, true
	}
	// Telemost only speaks vp8channel — coerce before Validate so a bad form
	// save does not leave the sidecar permanently disabled (Enabled:false).
	if cfg.Provider == "telemost" && cfg.Transport != "vp8channel" {
		cfg.Transport = "vp8channel"
	}
	cfg = cfg.ClampVP8()
	if err := cfg.Validate(); err != nil {
		return Instance{Core: Olcrtc, Key: key, Enabled: false}, true
	}
	// Ensure key for URI completeness when empty (caller should persist).
	if strings.TrimSpace(cfg.CryptoKey) == "" {
		if k, err := GenerateCryptoKey(); err == nil {
			cfg.CryptoKey = k
		}
	}
	data := dataDirFor(key, Olcrtc)
	return Instance{
		Core:       Olcrtc,
		Key:        key,
		Enabled:    true,
		ConfigText: cfg.RenderYAML(data),
		ProbePort:  0,
	}, true
}
