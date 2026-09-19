// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRenderGatewayCaddyfile_SNIAndDrop(t *testing.T) {
	got := RenderGatewayCaddyfile(443, []GatewayRoute{
		{SNI: "www.microsoft.com", Dest: "127.0.0.1:1443"},
		{SNI: "vpn.example.com", Dest: "127.0.0.1:8443"},
		{SNI: "www.microsoft.com", Dest: "127.0.0.1:9"},
	}, "", "")
	for _, need := range []string{
		"admin off",
		"layer4",
		":443",
		"tls sni www.microsoft.com",
		"proxy 127.0.0.1:1443",
		"tls sni vpn.example.com",
		"proxy 127.0.0.1:8443",
		"proxy 127.0.0.1:1",
		"proxy_protocol v1",
	} {
		if !strings.Contains(got, need) {
			t.Fatalf("missing %q:\n%s", need, got)
		}
	}
	if strings.Count(got, "tls sni www.microsoft.com") != 1 {
		t.Fatalf("duplicate SNI:\n%s", got)
	}
	got = RenderGatewayCaddyfile(443, []GatewayRoute{{SNI: "vpn.example.com", Dest: "127.0.0.1:8443"}}, "127.0.0.1:8443", "")
	if strings.Count(got, "proxy 127.0.0.1:8443") < 1 {
		t.Fatalf("cover fallback:\n%s", got)
	}
}

func TestClassify_RealityVsCoverVsUDP(t *testing.T) {
	reality := &model.Inbound{
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.microsoft.com"],"dest":"www.microsoft.com:443"}}`,
	}
	class, sni := Classify(reality)
	if class != ClassPassthrough || sni != "www.microsoft.com" {
		t.Fatalf("reality: %s %s", class, sni)
	}
	cover := &model.Inbound{
		Protocol: model.Cover,
		Settings: `{"hostname":"vpn.example.com"}`,
	}
	class, sni = Classify(cover)
	if class != ClassCaddy || sni != "vpn.example.com" {
		t.Fatalf("cover: %s %s", class, sni)
	}
	class, sni = Classify(&model.Inbound{Protocol: model.Anytls, Settings: `{"sni":"vpn.example.com"}`})
	if class != ClassPassthrough || sni != "vpn.example.com" {
		t.Fatalf("anytls: %s %s", class, sni)
	}
	class, sni = Classify(&model.Inbound{Protocol: model.TrustTunnel, Settings: `{"hostname":"tt.example.com","listen":"0.0.0.0:8443"}`})
	if class != ClassPassthrough || sni != "tt.example.com" {
		t.Fatalf("trusttunnel: %s %s", class, sni)
	}
	class, _ = Classify(&model.Inbound{Protocol: model.AWG})
	if class != "" {
		t.Fatalf("awg: %s", class)
	}
	ws := &model.Inbound{
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"ws","security":"none"}`,
	}
	class, _ = Classify(ws)
	if class != ClassCaddy {
		t.Fatalf("ws: %s", class)
	}
}

func TestBuildPreview_MovesPublic443(t *testing.T) {
	rows := BuildPreview(443, "node.example.com", []*model.Inbound{
		{
			Id: 1, Protocol: model.VLESS, Port: 443, Remark: "R",
			StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.microsoft.com"]}}`,
		},
		{Id: 2, Protocol: model.Cover, Port: 443, Settings: `{"hostname":"vpn.example.com"}`},
		{Id: 3, Protocol: model.AWG, Port: 443},
	}, "")
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	byID := map[int]PreviewRow{}
	for _, r := range rows {
		byID[r.InboundID] = r
	}
	r := byID[1]
	if r.NewListen != "127.0.0.1" || r.NewPort == 443 || r.Class != ClassPassthrough {
		t.Fatalf("reality row: %+v", r)
	}
	if r.HostAddress != "node.example.com" || r.HostPort != 443 {
		t.Fatalf("hosts: %+v", r)
	}
	if r.StealDest != "127.0.0.1:"+strconv.Itoa(byID[2].NewPort) {
		t.Fatalf("steal dest %q cover port %d", r.StealDest, byID[2].NewPort)
	}
	c := byID[2]
	if c.Class != ClassCaddy || c.NewPort == 443 {
		t.Fatalf("cover row: %+v", c)
	}
}

func TestBuildPreview_SkipPublicNonSNI(t *testing.T) {
	rows := BuildPreview(443, "node.example.com", []*model.Inbound{
		{Id: 3, Protocol: model.Hysteria, Enable: true, Port: 4443, Remark: "hy"},
		{Id: 4, Protocol: model.AWG, Enable: false, Port: 51820},
	}, "")
	if len(rows) != 1 || rows[0].Class != ClassSkip || rows[0].OldPort != 4443 {
		t.Fatalf("%+v", rows)
	}
}

func TestCoverFallback(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Protocol: "vless", NewPort: 1443},
		{InboundID: 2, Protocol: "cover", NewPort: 8443},
	}
	if got := CoverFallback(rows, map[int]bool{2: true}); got != "127.0.0.1:8443" {
		t.Fatalf("%s", got)
	}
	if got := CoverFallback(rows, map[int]bool{1: true}); got != "" {
		t.Fatalf("no cover selected: %s", got)
	}
}

func TestResolveGatewayPublicHost(t *testing.T) {
	cover := &model.Inbound{Protocol: model.Cover, Settings: `{"hostname":"vpn.example.com"}`}
	if got := ResolveGatewayPublicHost(" Node.Example.com ", "saved.com", nil); got != "node.example.com" {
		t.Fatalf("req: %s", got)
	}
	if got := ResolveGatewayPublicHost("", "Saved.com", []*model.Inbound{cover}); got != "saved.com" {
		t.Fatalf("saved: %s", got)
	}
	if got := ResolveGatewayPublicHost("", "", []*model.Inbound{cover}); got != "vpn.example.com" {
		t.Fatalf("cover: %s", got)
	}
}

func TestSetRealityDest(t *testing.T) {
	in := `{"network":"tcp","security":"reality","realitySettings":{"dest":"www.microsoft.com:443"}}`
	got := SetRealityDest(in, "127.0.0.1:8443")
	if !strings.Contains(got, `"dest":"127.0.0.1:8443"`) || !strings.Contains(got, `"target":"127.0.0.1:8443"`) {
		t.Fatalf("%s", got)
	}
}

func TestSetAcceptProxyProtocol(t *testing.T) {
	in := `{"network":"tcp","security":"reality"}`
	got := SetAcceptProxyProtocol(in, true)
	if !strings.Contains(got, `"acceptProxyProtocol":true`) || !strings.Contains(got, `"tcpSettings"`) {
		t.Fatalf("%s", got)
	}
	off := SetAcceptProxyProtocol(got, false)
	if strings.Contains(off, "acceptProxyProtocol") {
		t.Fatalf("%s", off)
	}
}

func TestRoutesFromPreview_Selected(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, SNI: "a.example.com", NewPort: 1443},
		{InboundID: 2, SNI: "b.example.com", NewPort: 8443},
	}
	got := RoutesFromPreview(rows, map[int]bool{2: true})
	if len(got) != 1 || got[0].SNI != "b.example.com" || got[0].Dest != "127.0.0.1:8443" {
		t.Fatalf("%+v", got)
	}
}

func TestRoutesFromPreview_AllRealitySNIs(t *testing.T) {
	rows := []PreviewRow{{
		InboundID: 1, SNI: "www.microsoft.com",
		SNIs: []string{"www.microsoft.com", "microsoft.com"}, NewPort: 1443,
	}}
	got := RoutesFromPreview(rows, map[int]bool{1: true})
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestGatewayInstance_DisabledUntilSnapshot(t *testing.T) {
	ib := &model.Inbound{Id: 9, Protocol: model.Gateway, Enable: true, Port: 443, Settings: `{}`}
	inst, ok := GatewayInstanceFromInbound(ib, nil)
	if !ok || inst.Enabled {
		t.Fatalf("ok=%v enabled=%v", ok, inst.Enabled)
	}
}

func TestRenderGatewayCaddyfile_BindIP(t *testing.T) {
	got := RenderGatewayCaddyfile(443, []GatewayRoute{{SNI: "vpn.example.com", Dest: "127.0.0.1:443"}}, "127.0.0.1:443", "203.0.113.5")
	if !strings.Contains(got, "203.0.113.5:443") {
		t.Fatalf("%s", got)
	}
}

func TestBuildPreview_BindIPKeepsCover443(t *testing.T) {
	rows := BuildPreview(443, "node.example.com", []*model.Inbound{
		{
			Id: 1, Protocol: model.VLESS, Port: 443,
			StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.microsoft.com"]}}`,
		},
		{Id: 2, Protocol: model.Cover, Port: 443, Settings: `{"hostname":"vpn.example.com"}`},
	}, "203.0.113.5")
	byID := map[int]PreviewRow{}
	for _, r := range rows {
		byID[r.InboundID] = r
	}
	if byID[2].NewPort != 443 || byID[2].NewListen != "127.0.0.1" {
		t.Fatalf("cover should keep 443: %+v", byID[2])
	}
	if byID[1].NewPort == 443 {
		t.Fatalf("reality should leave 443: %+v", byID[1])
	}
}

func TestBuildPreview_BindIPKeepsSolo443(t *testing.T) {
	rows := BuildPreview(443, "node.example.com", []*model.Inbound{
		{
			Id: 1, Protocol: model.VLESS, Port: 443,
			StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.microsoft.com"]}}`,
		},
	}, "203.0.113.5")
	if len(rows) != 1 || rows[0].NewPort != 443 || rows[0].NewListen != "127.0.0.1" {
		t.Fatalf("%+v", rows)
	}
}

func TestRoutesFromPreview_CaddyKeepsPublicHost(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, SNI: "www.microsoft.com", SNIs: []string{"www.microsoft.com"}, HostAddress: "vpn.example.com", NewPort: 1443},
		{InboundID: 2, Class: ClassCaddy, SNI: "vpn.example.com", NewPort: 443},
	}
	sel := map[int]bool{1: true, 2: true}
	got := RoutesFromPreview(rows, sel)
	by := map[string]string{}
	for _, r := range got {
		by[r.SNI] = r.Dest
	}
	if by["www.microsoft.com"] != "127.0.0.1:1443" || by["vpn.example.com"] != "127.0.0.1:443" {
		t.Fatalf("%+v", got)
	}
	if !CaddyClaimsHost(rows, sel, "vpn.example.com") {
		t.Fatal("cover must own public host")
	}
}

func TestRoutesFromPreview_VLESSOwnsPublicHostWhenNoCaddy(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, SNI: "www.microsoft.com", SNIs: []string{"www.microsoft.com"}, HostAddress: "vpn.example.com", NewPort: 443},
	}
	got := RoutesFromPreview(rows, map[int]bool{1: true})
	by := map[string]string{}
	for _, r := range got {
		by[r.SNI] = r.Dest
	}
	if by["www.microsoft.com"] != "127.0.0.1:443" || by["vpn.example.com"] != "127.0.0.1:443" {
		t.Fatalf("%+v", got)
	}
}

func TestAddRealityServerName(t *testing.T) {
	in := `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.microsoft.com"]}}`
	got := AddRealityServerName(in, "vpn.example.com")
	if !strings.Contains(got, `"vpn.example.com"`) || !strings.Contains(got, `"www.microsoft.com"`) {
		t.Fatalf("%s", got)
	}
	same := AddRealityServerName(got, "vpn.example.com")
	if strings.Count(same, "vpn.example.com") != 1 {
		t.Fatalf("dup: %s", same)
	}
}
