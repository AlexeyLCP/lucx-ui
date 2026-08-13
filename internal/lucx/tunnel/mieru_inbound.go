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

// MieruKey returns the manager key for a mieru inbound id (mieru-12).
func MieruKey(inboundId int) string {
	return fmt.Sprintf("mieru-%d", inboundId)
}

// mieruInboundSettings is the JSON shape of inbound.settings for
// protocol=mieru: MieruConfig plus multi-client clients[] (naive pattern).
type mieruInboundSettings struct {
	Remark           string             `json:"remark"`
	PortBindings     []MieruPortBinding `json:"portBindings"`
	MTU              int                `json:"mtu"`
	LoggingLevel     string             `json:"loggingLevel"`
	RouteThroughXray bool               `json:"routeThroughXray"`
	RouteXrayPort    int                `json:"routeXrayPort"`
	OutboundTag      string             `json:"outboundTag"`
	Clients          []struct {
		Email  string `json:"email"`
		Enable bool   `json:"enable"`
	} `json:"clients"`
}

// MieruConfigFromInbound maps an inbound row to MieruConfig.
func MieruConfigFromInbound(ib *model.Inbound) (MieruConfig, bool) {
	if ib == nil || ib.Protocol != model.Mieru {
		return MieruConfig{}, false
	}
	var s mieruInboundSettings
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &s)
	}
	cfg := MieruConfig{
		Remark:           firstNonEmpty(s.Remark, ib.Remark),
		Enabled:          ib.Enable,
		PortBindings:     s.PortBindings,
		MTU:              s.MTU,
		LoggingLevel:     s.LoggingLevel,
		RouteThroughXray: s.RouteThroughXray,
		RouteXrayPort:    s.RouteXrayPort,
		OutboundTag:      s.OutboundTag,
	}.Merge()
	return cfg, true
}

// MieruInstanceFromInbound builds a supervised Instance for a mieru inbound.
// secret derives the per-client mita users. ok is false only for a non-mieru
// row; disabled / invalid / clientless inbounds yield an Enabled:false
// instance so reconcile converges them down instead of erroring.
func MieruInstanceFromInbound(ib *model.Inbound, secret []byte) (Instance, bool) {
	cfg, ok := MieruConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := MieruKey(ib.Id)
	if !ib.Enable {
		return Instance{Core: Mieru, Key: key, Enabled: false}, true
	}

	var users []AuthPair
	if len(secret) > 0 {
		var s mieruInboundSettings
		_ = json.Unmarshal([]byte(ib.Settings), &s)
		for _, c := range s.Clients {
			if !c.Enable || strings.TrimSpace(c.Email) == "" {
				continue
			}
			users = append(users, MieruClientAuth(secret, ib.Id, c.Email))
		}
	}

	if err := cfg.ValidateInbound(len(users) > 0); err != nil {
		return Instance{Core: Mieru, Key: key, Enabled: false}, true
	}

	return Instance{
		Core:       Mieru,
		Key:        key,
		Enabled:    true,
		ConfigText: cfg.RenderJSON(users),
		ProbePort:  mieruProbePort(cfg),
	}, true
}

// mieruProbePort picks the first single-port TCP binding for the listening
// probe (ranges and UDP bindings are not probeable). Zero disables probing.
func mieruProbePort(cfg MieruConfig) int {
	for _, b := range cfg.PortBindings {
		if b.Port > 0 && strings.EqualFold(strings.TrimSpace(b.Protocol), "TCP") {
			return b.Port
		}
	}
	return 0
}
