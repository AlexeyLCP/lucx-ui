// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestOlcrtcRenderYAMLSocks(t *testing.T) {
	cfg := DefaultOlcrtcConfig()
	cfg.RoomID = "r"
	cfg.CryptoKey = strings.Repeat("a", 64)
	if cfg.RouteThroughXray {
		t.Fatal("default RouteThroughXray must be false (Telemost direct egress)")
	}
	got := cfg.RenderYAML("data")
	if strings.Contains(got, "socks:") {
		t.Fatal("default YAML must not enable socks")
	}
	cfg.RouteThroughXray = true
	cfg.RouteXrayPort = 51080
	got = cfg.RenderYAML("data")
	for _, want := range []string{`proxy_addr: "127.0.0.1"`, "proxy_port: 51080", "socks:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	cfg.RouteThroughXray = false
	got = cfg.RenderYAML("data")
	if strings.Contains(got, "socks:") {
		t.Fatal("socks must be omitted when not routed")
	}
}

func TestOlcrtcConfigFromInbound_DefaultNoRoute(t *testing.T) {
	ib := &model.Inbound{
		Protocol: model.Olcrtc,
		Settings: `{"provider":"telemost","roomId":"r","cryptoKey":"` + strings.Repeat("a", 64) + `","transport":"vp8channel","dns":"8.8.8.8:53"}`,
	}
	cfg, ok := OlcrtcConfigFromInbound(ib)
	if !ok {
		t.Fatal("expected ok")
	}
	if cfg.RouteThroughXray {
		t.Fatal("absent routeThroughXray must stay false")
	}
}
