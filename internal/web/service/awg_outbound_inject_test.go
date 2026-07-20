// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// awgOutboundTestConfig builds a minimal *xray.Config with the standard
// template outbounds (direct/block/api) so injection appends after them and the
// pre-existing array stays valid. Mirrors egressTestConfig but seeds
// OutboundConfigs, the field injectAwgOutbounds touches.
func awgOutboundTestConfig() *xray.Config {
	return &xray.Config{
		OutboundConfigs: json_util.RawMessage(`[
  {
    "protocol": "freedom",
    "tag": "direct"
  },
  {
    "protocol": "blackhole",
    "tag": "block"
  }
]`),
	}
}

// serializeAwgOutbounds returns the JSON of cfg.OutboundConfigs as a string for
// substring assertions. Empty outbounds become "" so contains() checks fail
// cleanly rather than matching on the literal "null".
func serializeAwgOutbounds(t *testing.T, cfg *xray.Config) string {
	t.Helper()
	if len(cfg.OutboundConfigs) == 0 {
		return ""
	}
	var raw []any
	if err := json.Unmarshal(cfg.OutboundConfigs, &raw); err != nil {
		t.Fatalf("outbounds unmarshal: %v\n%s", err, cfg.OutboundConfigs)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("outbounds marshal: %v", err)
	}
	return string(b)
}

// LUCX-HOOK: AWG injectAwgOutbounds tests, mirroring the egress suite above.

func TestInjectAwgOutbounds_DisabledSkipped(t *testing.T) {
	cfg := awgOutboundTestConfig()
	outbounds := []*model.AwgOutbound{
		{Id: 1, Tag: "awgo-1", Enable: false, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
		{Id: 2, Tag: "awgo-2", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	out := serializeAwgOutbounds(t, cfg)
	if strings.Contains(out, `"tag":"awgo-1"`) {
		t.Error("disabled outbound should not be injected")
	}
	if !strings.Contains(out, `"tag":"awgo-2"`) {
		t.Error("enabled outbound should be injected")
	}
}

func TestInjectAwgOutbounds_SendThroughStripsCIDR(t *testing.T) {
	cfg := awgOutboundTestConfig()
	outbounds := []*model.AwgOutbound{
		{Id: 1, Tag: "vpn-frankfurt", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	out := serializeAwgOutbounds(t, cfg)
	if strings.Contains(out, "10.9.0.5/32") {
		t.Error("sendThrough must strip CIDR mask, got:", out)
	}
	if !strings.Contains(out, `"10.9.0.5"`) {
		t.Error("sendThrough should contain the bare IP, got:", out)
	}
}

func TestInjectAwgOutbounds_UsesTagNotIfname(t *testing.T) {
	cfg := awgOutboundTestConfig()
	outbounds := []*model.AwgOutbound{
		{Id: 5, Tag: "vpn-frankfurt", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	out := serializeAwgOutbounds(t, cfg)
	if !strings.Contains(out, `"tag":"vpn-frankfurt"`) {
		t.Error("outbound tag should be the editable Tag, not awgo-5, got:", out)
	}
	if !strings.Contains(out, `"interface":"awgo-5"`) {
		t.Error("sockopt.interface should be awgo-{Id}, got:", out)
	}
}

func TestInjectAwgOutbounds_PreservesTemplateOutbounds(t *testing.T) {
	// Pre-existing template outbounds (direct/block) must survive the injection:
	// injectAwgOutbounds appends rather than overwriting, mirroring
	// mergeSubscriptionOutbounds' safety contract.
	cfg := awgOutboundTestConfig()
	outbounds := []*model.AwgOutbound{
		{Id: 1, Tag: "vpn-frankfurt", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	out := serializeAwgOutbounds(t, cfg)
	if !strings.Contains(out, `"tag":"direct"`) || !strings.Contains(out, `"tag":"block"`) {
		t.Error("template outbounds must be preserved, got:", out)
	}
	if !strings.Contains(out, `"tag":"vpn-frankfurt"`) {
		t.Error("injected outbound must be appended after template outbounds, got:", out)
	}
}

func TestInjectAwgOutbounds_IncompleteSettingsSkipped(t *testing.T) {
	// ClientInstanceFromOutbound returns ok=false when Address/PublicKey/Endpoint
	// is missing; the injector must skip such rows without mutating OutboundConfigs.
	cfg := awgOutboundTestConfig()
	before := string(cfg.OutboundConfigs)
	outbounds := []*model.AwgOutbound{
		{Id: 1, Tag: "vpn-broken", Enable: true, Settings: `{"privateKey":"k"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	if string(cfg.OutboundConfigs) != before {
		t.Fatalf("incomplete settings must be a no-op, got: %s", cfg.OutboundConfigs)
	}
}

// END LUCX-HOOK
