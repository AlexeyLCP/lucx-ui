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

func TestInjectAwgOutbounds_DisabledBecomesBlackhole(t *testing.T) {
	cfg := awgOutboundTestConfig()
	outbounds := []*model.AwgOutbound{
		{Id: 1, Tag: "awgo-1", Enable: false, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
		{Id: 2, Tag: "awgo-2", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	if proto := outboundProtocolByTag(t, cfg, "awgo-1"); proto != "blackhole" {
		t.Fatalf("disabled tag must be blackhole, got %q", proto)
	}
	if proto := outboundProtocolByTag(t, cfg, "awgo-2"); proto != "freedom" {
		t.Fatalf("enabled tag must be freedom, got %q", proto)
	}
}

func outboundProtocolByTag(t *testing.T, cfg *xray.Config, tag string) string {
	t.Helper()
	var raw []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &raw); err != nil {
		t.Fatalf("outbounds unmarshal: %v", err)
	}
	for _, ob := range raw {
		if t, _ := ob["tag"].(string); t == tag {
			p, _ := ob["protocol"].(string)
			return p
		}
	}
	return ""
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

// outboundByTag returns the whole outbound object so a test can assert where a
// key sits, not merely that the value appears somewhere in the JSON.
func outboundByTag(t *testing.T, cfg *xray.Config, tag string) map[string]any {
	t.Helper()
	var raw []map[string]any
	if err := json.Unmarshal(cfg.OutboundConfigs, &raw); err != nil {
		t.Fatalf("outbounds unmarshal: %v", err)
	}
	for _, ob := range raw {
		if got, _ := ob["tag"].(string); got == tag {
			return ob
		}
	}
	t.Fatalf("outbound %q not found in %s", tag, cfg.OutboundConfigs)
	return nil
}

// Xray reads sendThrough from the outbound itself (OutboundDetourConfig);
// FreedomConfig has no such field and the loader drops unknown keys without a
// word. Nested under settings the value is silently lost, which turns the
// binding from fail-closed — a bad address refuses to bind and the connection
// errors out — into fail-open, leaking the host's real address whenever
// sockopt.interface also fails, and that failure is only logged.
func TestInjectAwgOutbounds_SendThroughSitsOnTheOutbound(t *testing.T) {
	cfg := awgOutboundTestConfig()
	injectAwgOutbounds(cfg, []*model.AwgOutbound{
		{Id: 1, Tag: "vpn-frankfurt", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
	})

	ob := outboundByTag(t, cfg, "vpn-frankfurt")
	if got, _ := ob["sendThrough"].(string); got != "10.9.0.5" {
		t.Errorf("sendThrough must sit on the outbound, got %q; outbound = %v", got, ob)
	}
	settings, _ := ob["settings"].(map[string]any)
	if _, nested := settings["sendThrough"]; nested {
		t.Error("sendThrough under settings is dropped by the config loader")
	}
}

// The blackhole decision is the only part of config generation that can differ
// between two builds of the same database, and RestartXray compares configs to
// decide whether to restart. It must follow recorded state, never a probe.
func TestInjectAwgOutbounds_BlackholeFollowsRecordedLiveness(t *testing.T) {
	saved := awgOutboundIsUp
	t.Cleanup(func() { awgOutboundIsUp = saved })

	for _, tc := range []struct {
		name      string
		up        bool
		wantProto string
	}{
		{"recorded up stays a tunnel", true, "freedom"},
		{"recorded down is fail-closed", false, "blackhole"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probed := 0
			awgOutboundIsUp = func(string) bool { probed++; return tc.up }
			cfg := awgOutboundTestConfig()
			injectAwgOutbounds(cfg, []*model.AwgOutbound{
				{Id: 1, Tag: "awgo-1", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
			})
			if got := outboundProtocolByTag(t, cfg, "awgo-1"); got != tc.wantProto {
				t.Errorf("protocol = %q, want %q", got, tc.wantProto)
			}
			if probed != 1 {
				t.Errorf("liveness must be consulted exactly once per outbound, got %d", probed)
			}
		})
	}
}
