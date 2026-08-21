// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestProbeHTTP_SocksDown(t *testing.T) {
	_, err := (&SidecarOutboundService{}).ProbeHTTP(t.Context(), 1, "http://127.0.0.1/")
	if err == nil {
		t.Fatal("probe through a closed socks port must fail")
	}
}

func TestInjectSidecarOutbounds_DisabledBecomesBlackhole(t *testing.T) {
	cfg := awgOutboundTestConfig()
	rows := []*model.SidecarOutbound{
		{Id: 1, Protocol: "naive", Tag: "naive-up", Enable: false, Settings: `{"socksPort":39111,"host":"n.example.org","port":443,"user":"a","pass":"b"}`},
		{Id: 2, Protocol: "mieru", Tag: "mieru-up", Enable: true, Settings: `{"socksPort":39222,"host":"1.2.3.4","port":6666,"user":"a","pass":"b"}`},
	}
	injectSidecarOutbounds(cfg, rows)
	if proto := outboundProtocolByTag(t, cfg, "naive-up"); proto != "blackhole" {
		t.Fatalf("disabled sidecar must be blackhole, got %q", proto)
	}
	if proto := outboundProtocolByTag(t, cfg, "mieru-up"); proto != "socks" {
		t.Fatalf("enabled sidecar must be socks, got %q", proto)
	}
	out := serializeAwgOutbounds(t, cfg)
	if !strings.Contains(out, `"port":39222`) {
		t.Error("socks port missing")
	}
}

func TestInjectSidecarOutbounds_IncompleteSkipped(t *testing.T) {
	cfg := awgOutboundTestConfig()
	before := string(cfg.OutboundConfigs)
	injectSidecarOutbounds(cfg, []*model.SidecarOutbound{
		{Id: 1, Protocol: "naive", Tag: "broken", Enable: true, Settings: `{"host":"h"}`},
	})
	if string(cfg.OutboundConfigs) != before {
		t.Fatalf("incomplete settings must be a no-op, got %s", cfg.OutboundConfigs)
	}
}

func TestInjectSidecarOutbounds_PreservesTemplate(t *testing.T) {
	cfg := awgOutboundTestConfig()
	injectSidecarOutbounds(cfg, []*model.SidecarOutbound{
		{Id: 1, Protocol: "naive", Tag: "nv", Enable: true, Settings: `{"socksPort":39111,"host":"n.example.org","port":443,"user":"a","pass":"b"}`},
	})
	out := serializeAwgOutbounds(t, cfg)
	if !strings.Contains(out, `"tag":"direct"`) || !strings.Contains(out, `"tag":"nv"`) {
		t.Fatalf("template + sidecar tags missing: %s", out)
	}
}
