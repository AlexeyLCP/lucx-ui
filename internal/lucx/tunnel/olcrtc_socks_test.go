// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
package tunnel

import (
	"strings"
	"testing"
)

func TestOlcrtcRenderYAMLSocks(t *testing.T) {
	cfg := DefaultOlcrtcConfig()
	cfg.RoomID = "r"
	cfg.CryptoKey = strings.Repeat("a", 64)
	cfg.RouteThroughXray = true
	cfg.RouteXrayPort = 51080
	got := cfg.RenderYAML("data")
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
