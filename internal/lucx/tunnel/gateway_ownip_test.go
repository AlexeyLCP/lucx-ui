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

func TestBuildPreviewKeepsOwnIPAddress(t *testing.T) {
	ib := &model.Inbound{
		Id: 30, Enable: true, Protocol: model.VLESS, Listen: "10.0.0.6", Port: 443,
		StreamSettings: `{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"cdn003.example"}}`,
	}
	others := []*model.Inbound{ib}
	find := func(rows []PreviewRow) PreviewRow {
		for _, r := range rows {
			if r.InboundID == ib.Id {
				return r
			}
		}
		t.Fatalf("no row for inbound %d", ib.Id)
		return PreviewRow{}
	}

	// Gateway on the inbound's own IP: the existing client address stays.
	if got := find(BuildPreview(443, "panel.example", others, "10.0.0.6")).HostAddress; got != "10.0.0.6" {
		t.Errorf("own IP: HostAddress = %q, want 10.0.0.6", got)
	}
	// Gateway on another IP: clients must go through the public host.
	if got := find(BuildPreview(443, "panel.example", others, "10.0.0.1")).HostAddress; got != "panel.example" {
		t.Errorf("other IP: HostAddress = %q, want panel.example", got)
	}
}

func TestGatewayMultiIP(t *testing.T) {
	xhttp := &model.Inbound{
		Id: 30, Enable: true, Protocol: model.VLESS, Listen: "10.0.0.6", Port: 443,
		StreamSettings: `{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"cdn003.example"}}`,
	}
	grpc := &model.Inbound{
		Id: 5, Enable: true, Protocol: model.VLESS, Listen: "10.0.0.7", Port: 443,
		StreamSettings: `{"network":"grpc","security":"tls","tlsSettings":{"serverName":"cdn004.example"}}`,
	}
	rows := BuildPreview(443, "panel.example", []*model.Inbound{xhttp, grpc}, "10.0.0.6", "10.0.0.7")
	want := map[int]string{30: "10.0.0.6", 5: "10.0.0.7"}
	for _, r := range rows {
		if w, ok := want[r.InboundID]; ok && r.HostAddress != w {
			t.Errorf("inbound %d: HostAddress = %q, want %q", r.InboundID, r.HostAddress, w)
		}
	}

	conf := RenderGatewayCaddyfile(443, []GatewayRoute{{SNI: "cdn003.example", Dest: "127.0.0.1:1446"}}, "", "10.0.0.6", "10.0.0.7")
	if !strings.Contains(conf, "10.0.0.6:443 10.0.0.7:443 {") {
		t.Errorf("legacy render lacks both listens:\n%s", conf)
	}
	conf = RenderUnifiedGatewayCaddyfile(443, nil, "", []string{"10.0.0.6", "2001:db8::1"}, nil)
	if !strings.Contains(conf, "10.0.0.6:443 [2001:db8::1]:443 {") {
		t.Errorf("unified render lacks both listens:\n%s", conf)
	}

	cfg := GatewayConfig{BindIP: "10.0.0.6"}
	if got := cfg.Binds(); len(got) != 1 || got[0] != "10.0.0.6" {
		t.Errorf("old single-IP config: Binds() = %v", got)
	}
}

func TestTrustTunnelSiteUpstream(t *testing.T) {
	c := TrustTunnelConfig{Hostname: "cdn001.example", Listen: "10.0.0.4:443", UpstreamProtocol: "http2", SiteUpstream: "127.0.0.1:8081"}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid site upstream rejected: %v", err)
	}
	toml := c.RenderVpnToml("/c.toml", "/r.toml", "", "")
	if !strings.Contains(toml, "[reverse_proxy]\nserver_address = \"127.0.0.1:8081\"\npath_mask = \"/\"\n") {
		t.Errorf("reverse_proxy section missing:\n%s", toml)
	}
	c.SiteUpstream = ""
	if strings.Contains(c.RenderVpnToml("/c.toml", "/r.toml", "", ""), "reverse_proxy") {
		t.Error("empty site upstream must not render reverse_proxy")
	}
	ib := &model.Inbound{
		Id: 18, Enable: true, Protocol: model.TrustTunnel, Listen: "10.0.0.4", Port: 443,
		Settings: `{"hostname":"cdn001.example","listen":"10.0.0.4:443","siteUpstream":"127.0.0.1:8081"}`,
	}
	if got, _ := TrustTunnelConfigFromInbound(ib); got.SiteUpstream != "127.0.0.1:8081" {
		t.Errorf("inbound settings lost siteUpstream: %q", got.SiteUpstream)
	}
	c.SiteUpstream = "nope"
	if c.Validate() == nil {
		t.Error("site upstream without port must be rejected")
	}
}

func TestAnytlsHonoursListenIP(t *testing.T) {
	for listen, want := range map[string]string{
		"10.0.0.7":  "10.0.0.7:8301",
		"127.0.0.1": "127.0.0.1:8301",
		"":          "0.0.0.0:8301",
		"0.0.0.0":   "0.0.0.0:8301",
	} {
		ib := &model.Inbound{Id: 19, Enable: true, Protocol: model.Anytls, Listen: listen, Port: 8301, Settings: `{"password":"x"}`}
		cfg, _ := AnytlsConfigFromInbound(ib)
		if got := cfg.ListenAddr(); got != want {
			t.Errorf("listen %q: got %s, want %s", listen, got, want)
		}
	}
}

func TestMieruHonoursListenIP(t *testing.T) {
	for listen, want := range map[string]string{
		"10.0.0.5": "10.0.0.5",
		"":         "",
		"0.0.0.0":  "",
	} {
		ib := &model.Inbound{
			Id: 17, Enable: true, Protocol: model.Mieru, Listen: listen, Port: 8443,
			Settings: `{"portBindings":[{"port":8443,"protocol":"TCP"}]}`,
		}
		cfg, _ := MieruConfigFromInbound(ib)
		var out struct {
			ListenIPAddress *string `json:"listenIPAddress"`
		}
		if err := json.Unmarshal([]byte(cfg.RenderJSON([]AuthPair{{User: "u", Pass: "p"}})), &out); err != nil {
			t.Fatal(err)
		}
		got := ""
		if out.ListenIPAddress != nil {
			got = *out.ListenIPAddress
		}
		if got != want {
			t.Errorf("listen %q: listenIPAddress = %q, want %q", listen, got, want)
		}
	}
}
