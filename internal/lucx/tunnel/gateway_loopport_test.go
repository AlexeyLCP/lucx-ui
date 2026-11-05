// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestPreviewSkipsLoopbackPortsInUse(t *testing.T) {
	others := []*model.Inbound{
		{Id: 2, Enable: true, Listen: "127.0.0.1", Port: 1443, Protocol: model.VLESS, StreamSettings: `{"network":"ws","security":"none"}`},
		{Id: 30, Enable: true, Listen: "127.0.0.1", Port: 1446, Protocol: model.VLESS, StreamSettings: `{"network":"xhttp","security":"none"}`},
		{
			Id: 6, Enable: true, Listen: "203.0.113.9", Port: 443, Protocol: model.VLESS,
			StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"raw.example.com"}}`,
		},
	}
	for _, r := range BuildPreview(443, "", others, "203.0.113.9") {
		if r.InboundID != 6 {
			continue
		}
		if r.NewPort == 1443 || r.NewPort == 1446 {
			t.Fatalf("TCP inbound parked on a loopback port another inbound uses: %d", r.NewPort)
		}
		return
	}
	t.Fatal("TCP inbound missing from the preview")
}

func TestTrustTunnelAuthFailureStatus(t *testing.T) {
	c := TrustTunnelConfig{Hostname: "t.example.com", Listen: "127.0.0.1:1447", UpstreamProtocol: "http2", AuthFailureStatus: 405}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if out := c.RenderVpnToml("/c", "/r", "", ""); !strings.Contains(out, "auth_failure_status_code = 405\n") ||
		strings.Index(out, "auth_failure_status_code") > strings.Index(out, "[") {
		t.Errorf("want a top-level auth_failure_status_code = 405:\n%s", out)
	}
	c.AuthFailureStatus = 0
	if strings.Contains(c.RenderVpnToml("/c", "/r", "", ""), "auth_failure_status_code") {
		t.Error("0 keeps the stock 407 (no key)")
	}
	c.AuthFailureStatus = 500
	if c.Validate() == nil {
		t.Error("500 must be rejected")
	}
}

func TestTrustTunnelAuthFailureFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Protocol: model.TrustTunnel, Enable: true, Listen: "203.0.113.4", Port: 443,
		Settings: `{"hostname":"t.example.com","listen":"203.0.113.4:443","upstreamProtocol":"http2","authFailureStatus":405}`,
	}
	cfg, ok := TrustTunnelConfigFromInbound(ib)
	if !ok || cfg.AuthFailureStatus != 405 {
		t.Fatalf("authFailureStatus lost when reading the inbound: %+v", cfg.AuthFailureStatus)
	}
}

func TestRevertKeepsEditsMadeWhileMasked(t *testing.T) {
	orig := `{"port":443,"sni":"a.example.com","tls":{"fp":"chrome","alpn":["h2"]},"gone":1}`
	applied := `{"port":1445,"sni":"a.example.com","tls":{"fp":"chrome","alpn":["h2"]},"gone":1}`
	cur := `{"port":1445,"sni":"a.example.com","tls":{"fp":"firefox","alpn":["h2"]},"fallback":"127.0.0.1:18087"}`
	var got map[string]any
	if err := json.Unmarshal([]byte(MergeRevertJSON(orig, applied, cur)), &got); err != nil {
		t.Fatal(err)
	}
	if got["port"] != float64(443) {
		t.Errorf("the gateway's own change must be undone: port=%v", got["port"])
	}
	if got["fallback"] != "127.0.0.1:18087" {
		t.Errorf("a key added while masked must survive: %v", got["fallback"])
	}
	if got["tls"].(map[string]any)["fp"] != "firefox" {
		t.Errorf("a nested edit made while masked must survive: %v", got["tls"])
	}
	if _, ok := got["gone"]; ok {
		t.Error("a key removed while masked must stay removed")
	}
	if MergeRevertJSON(orig, applied, "not json") != orig {
		t.Error("non-JSON falls back to the snapshot")
	}
	sr := GatewaySnapshotRow{Settings: orig, StreamSettings: `{"a":1}`}
	if u := RevertUpdates(sr, &model.Inbound{Settings: cur}); u["settings"] != orig {
		t.Error("an old snapshot without AppliedSettings restores the snapshot as before")
	}
}
