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

func OpenfluxKey(inboundID int) string {
	return fmt.Sprintf("openflux-%d", inboundID)
}

func OpenfluxConfigFromInbound(ib *model.Inbound) (OpenfluxConfig, bool) {
	if ib == nil || ib.Protocol != model.Openflux {
		return OpenfluxConfig{}, false
	}
	cfg := DefaultOpenfluxConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	return cfg.Merge(), true
}

func OpenfluxInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := OpenfluxConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := OpenfluxKey(ib.Id)
	if !ib.Enable {
		return Instance{Core: Openflux, Key: key, Enabled: false}, true
	}
	port := ib.Port
	if port <= 0 {
		port = openfluxDefaultPort
	}
	if err := cfg.Validate(port); err != nil {
		return Instance{Core: Openflux, Key: key, Enabled: false}, true
	}
	return Instance{
		Core:       Openflux,
		Key:        key,
		Enabled:    true,
		ConfigText: cfg.RenderConf(key, port),
		Args:       []string{"--config", configPathFor(key, Openflux)},
		ExtraFiles: map[string]string{cfg.secretFileName(key): cfg.Secret + "\n"},
		ProbePort:  port,
	}, true
}
