// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

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
		"matching_timeout 15s",
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
	if class != "" {
		// A plaintext ws client never sends a TLS ClientHello — the SNI mux
		// can never route it; it must stay public, not die on loopback.
		t.Fatalf("ws+none must stay public: %s", class)
	}
}

func TestClassifyInbound_NoProxyAndSkip(t *testing.T) {
	// Backends that can't parse PROXY v1 must get a raw proxy route.
	for _, tc := range []struct {
		name    string
		ib      *model.Inbound
		class   string
		noProxy bool
	}{
		{"anytls", &model.Inbound{Protocol: model.Anytls, Settings: `{"sni":"a.ex.com"}`}, ClassPassthrough, true},
		{"trusttunnel", &model.Inbound{Protocol: model.TrustTunnel, Settings: `{"hostname":"t.ex.com"}`}, ClassPassthrough, true},
		{"xhttp+reality", &model.Inbound{Protocol: model.VLESS, StreamSettings: `{"network":"xhttp","security":"reality","realitySettings":{"serverNames":["x.ex.com"]}}`}, ClassPassthrough, true},
		{"tcp+reality", &model.Inbound{Protocol: model.VLESS, StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["x.ex.com"]}}`}, ClassPassthrough, false},
		{"grpc+reality", &model.Inbound{Protocol: model.VLESS, StreamSettings: `{"network":"grpc","security":"reality","realitySettings":{"serverNames":["x.ex.com"]}}`}, ClassPassthrough, false},
		{"kcp+tls", &model.Inbound{Protocol: model.VMESS, StreamSettings: `{"network":"kcp","security":"tls"}`}, "", false},
		{"naive behindCover", &model.Inbound{Protocol: model.Naive, Settings: `{"domain":"n.ex.com","behindCover":true}`}, "", false},
		{"naive raw", &model.Inbound{Protocol: model.Naive, Settings: `{"domain":"n.ex.com","useRawConfig":true}`}, "", false},
		{"tproxy behindCover", &model.Inbound{Protocol: model.Tproxy, Settings: `{"hostname":"p.ex.com","behindCover":true}`}, "", false},
	} {
		got := ClassifyInbound(tc.ib)
		if got.Class != tc.class || got.NoProxy != tc.noProxy {
			t.Fatalf("%s: class=%q noProxy=%v", tc.name, got.Class, got.NoProxy)
		}
	}
}

func TestRenderGatewayCaddyfile_NoProxy(t *testing.T) {
	got := RenderGatewayCaddyfile(443, []GatewayRoute{
		{SNI: "raw.example.com", Dest: "127.0.0.1:1443", NoProxy: true},
		{SNI: "proxy.example.com", Dest: "127.0.0.1:1444"},
	}, "", "")
	if !strings.Contains(got, "proxy 127.0.0.1:1443\n") {
		t.Fatalf("NoProxy route must be a bare proxy line:\n%s", got)
	}
	if !strings.Contains(got, "proxy 127.0.0.1:1444 {\n\t\t\t\t\tproxy_protocol v1") {
		t.Fatalf("normal route must keep PROXY v1:\n%s", got)
	}
}

func TestInboundUsesTCP(t *testing.T) {
	if InboundUsesTCP(&model.Inbound{Protocol: model.Hysteria, Port: 443}) {
		t.Fatal("hysteria is UDP only")
	}
	if InboundUsesTCP(&model.Inbound{Protocol: model.Naive, Port: 443, Settings: `{"behindCover":true}`}) {
		t.Fatal("behindCover naive owns no listener")
	}
	if !InboundUsesTCP(&model.Inbound{Protocol: model.Mieru, Port: 443}) {
		t.Fatal("mieru holds TCP")
	}
	if !InboundUsesTCP(&model.Inbound{Protocol: model.VLESS, Port: 443, StreamSettings: `{"network":"tcp","security":"reality"}`}) {
		t.Fatal("vless tcp holds TCP")
	}
}

func TestSetNaiveCert(t *testing.T) {
	ib := &model.Inbound{Protocol: model.Naive, Settings: `{"domain":"n.ex.com","useAcme":true,"acmeEmail":"a@b.c"}`}
	SetNaiveCert(ib, "/c.pem", "/k.pem")
	if !strings.Contains(ib.Settings, `"useAcme":false`) ||
		!strings.Contains(ib.Settings, `"certFile":"/c.pem"`) ||
		!strings.Contains(ib.Settings, `"keyFile":"/k.pem"`) {
		t.Fatalf("%s", ib.Settings)
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
	reality := &model.Inbound{
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.microsoft.com"],"dest":"www.microsoft.com:443"}}`,
	}
	if got := ResolveGatewayPublicHost(" Node.Example.com ", "saved.com", "", nil); got != "node.example.com" {
		t.Fatalf("req: %s", got)
	}
	if got := ResolveGatewayPublicHost("", "Saved.com", "", []*model.Inbound{cover}); got != "saved.com" {
		t.Fatalf("saved: %s", got)
	}
	if got := ResolveGatewayPublicHost("", "", "", []*model.Inbound{cover}); got != "vpn.example.com" {
		t.Fatalf("cover: %s", got)
	}
	if got := ResolveGatewayPublicHost("", "www.microsoft.com", "", []*model.Inbound{reality, cover}); got != "vpn.example.com" {
		t.Fatalf("decoy saved must not beat cover: %s", got)
	}
	if got := ResolveGatewayPublicHost("", "", "panel.example.com", []*model.Inbound{reality}); got != "panel.example.com" {
		t.Fatalf("panel domain over decoy: %s", got)
	}
	if got := ResolveGatewayPublicHost("", "", "", []*model.Inbound{reality}); got != "" {
		t.Fatalf("decoy alone is not a public host: %s", got)
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
	xh := SetAcceptProxyProtocol(`{"network":"xhttp","security":"reality","xhttpSettings":{"path":"/"}}`, true)
	if !strings.Contains(xh, `"sockopt"`) || !strings.Contains(xh, `"acceptProxyProtocol":true`) {
		t.Fatalf("xhttp: %s", xh)
	}
}

func TestRoutesFromPreview_Selected(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, SNI: "a.example.com", NewPort: 1443, NoProxy: true},
		{InboundID: 2, SNI: "b.example.com", NewPort: 8443},
	}
	got := RoutesFromPreview(rows, map[int]bool{1: true, 2: true})
	if len(got) != 2 || !got[0].NoProxy || got[1].NoProxy {
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
	inst, ok := GatewayInstanceFromInbound(ib, nil, nil, "", "")
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

func TestSetInboundSNI(t *testing.T) {
	cover := &model.Inbound{Protocol: model.Cover, Settings: `{"hostname":"old.example.com"}`}
	SetInboundSNI(cover, "New.Example.com")
	if !strings.Contains(cover.Settings, `"hostname":"new.example.com"`) {
		t.Fatalf("%s", cover.Settings)
	}
	vless := &model.Inbound{
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["www.microsoft.com"],"dest":"www.microsoft.com:443"}}`,
	}
	SetInboundSNI(vless, "vpn.example.com")
	if !strings.Contains(vless.StreamSettings, `"serverNames":["vpn.example.com"]`) {
		t.Fatalf("serverNames: %s", vless.StreamSettings)
	}
	if !strings.Contains(vless.StreamSettings, `"dest":"www.microsoft.com:443"`) {
		t.Fatalf("dest: %s", vless.StreamSettings)
	}
}

func TestSNIClash(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, SNI: "a.example.com"},
		{InboundID: 2, Class: ClassCaddy, SNI: "a.example.com"},
	}
	if got := SNIClash(rows, map[int]bool{1: true, 2: true}); got != "a.example.com" {
		t.Fatalf("%q", got)
	}
	if got := SNIClash(rows, map[int]bool{1: true}); got != "" {
		t.Fatalf("one selected: %q", got)
	}
}

func TestRoutesFromPreview_DestSNIToXray(t *testing.T) {
	rows := []PreviewRow{
		{InboundID: 1, Class: ClassPassthrough, SNI: "i.s-microsoft.com", SNIs: []string{"i.s-microsoft.com"}, NewPort: 1443},
		{InboundID: 2, Class: ClassCaddy, SNI: "vpn.example.com", NewPort: 443},
	}
	got := RoutesFromPreview(rows, map[int]bool{1: true, 2: true})
	by := map[string]string{}
	for _, r := range got {
		by[r.SNI] = r.Dest
	}
	if by["i.s-microsoft.com"] != "127.0.0.1:1443" || by["vpn.example.com"] != "127.0.0.1:443" {
		t.Fatalf("%+v", got)
	}
}

func TestRenderUnifiedGatewayCaddyfile(t *testing.T) {
	sites := []string{`:8443, "naive.example.com:8443" {
	bind l4chan/naive-1
	tls "/c.pem" "/k.pem"
	route {
		forward_proxy {
			basic_auth "u" "p"
		}
	}
}`}
	got := RenderUnifiedGatewayCaddyfile(443, []GatewayRoute{
		{SNI: "naive.example.com", Dest: "127.0.0.1:8443", Chan: "naive-1"},
		{SNI: "vless.example.com", Dest: "127.0.0.1:1443"},
		{SNI: "any.example.com", Dest: "127.0.0.1:1444", NoProxy: true},
	}, "cover-2", "203.0.113.5", sites)
	for _, need := range []string{
		"auto_https off",
		"protocols h1 h2",
		"203.0.113.5:443",
		"matching_timeout 15s",
		"tls sni naive.example.com",
		"l4http naive-1",
		"proxy 127.0.0.1:1443 {\n\t\t\t\t\tproxy_protocol v1",
		"proxy 127.0.0.1:1444\n",
		"l4http cover-2",
		"bind l4chan/naive-1",
		"forward_proxy",
	} {
		if !strings.Contains(got, need) {
			t.Fatalf("missing %q:\n%s", need, got)
		}
	}
}

func TestRenderUnifiedGatewayCaddyfile_NoFallbackDrops(t *testing.T) {
	got := RenderUnifiedGatewayCaddyfile(443, []GatewayRoute{
		{SNI: "a.example.com", Dest: "127.0.0.1:8443", Chan: "naive-1"},
	}, "", "", nil)
	if !strings.Contains(got, "proxy 127.0.0.1:1") {
		t.Fatalf("expected drop fallback:\n%s", got)
	}
}

func TestRenderGatewayCaddyfile_StripsChan(t *testing.T) {
	got := RenderGatewayCaddyfile(443, []GatewayRoute{
		{SNI: "a.example.com", Dest: "127.0.0.1:8443", Chan: "naive-1"},
	}, "", "")
	if strings.Contains(got, "l4http") || !strings.Contains(got, "proxy 127.0.0.1:8443") {
		t.Fatalf("legacy render leaked chan route:\n%s", got)
	}
}

func TestBuildPreview_SetsChan(t *testing.T) {
	rows := BuildPreview(443, "pub.example.com", []*model.Inbound{
		{
			Id: 1, Protocol: model.Naive, Port: 8443, Enable: true,
			Settings: `{"domain":"n.example.com","authUser":"u","authPass":"p","certFile":"/c","keyFile":"/k"}`,
		},
		{
			Id: 2, Protocol: model.Anytls, Port: 8444, Enable: true,
			Settings: `{"sni":"a.example.com","certFile":"/c","keyFile":"/k"}`,
		},
	}, "")
	byID := map[int]PreviewRow{}
	for _, r := range rows {
		byID[r.InboundID] = r
	}
	if byID[1].Chan != "naive-1" {
		t.Fatalf("naive chan: %+v", byID[1])
	}
	if byID[2].Chan != "" {
		t.Fatalf("passthrough row got chan: %+v", byID[2])
	}
	routes := RoutesFromPreview(rows, map[int]bool{1: true, 2: true})
	var n, a *GatewayRoute
	for i := range routes {
		switch routes[i].SNI {
		case "n.example.com":
			n = &routes[i]
		case "a.example.com":
			a = &routes[i]
		}
	}
	if n == nil || n.Chan != "naive-1" || n.Dest == "" {
		t.Fatalf("naive route: %+v", n)
	}
	if a == nil || a.Chan != "" || !a.NoProxy {
		t.Fatalf("anytls route: %+v", a)
	}
}

func stubGatewayChan(t *testing.T, ok bool) {
	t.Helper()
	old := GatewaySupportsChan
	GatewaySupportsChan = func() bool { return ok }
	t.Cleanup(func() { GatewaySupportsChan = old })
}

func TestGatewayInstance_UnifiedSites(t *testing.T) {
	stubGatewayChan(t, true)
	dir := t.TempDir()
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "cov.example.com", "n.example.com")
	cover := &model.Inbound{
		Id: 2, Protocol: model.Cover, Port: 8443, Enable: true, Listen: "127.0.0.1",
		Settings: `{"hostname":"cov.example.com","siteSource":"upstream","siteUpstream":"http://127.0.0.1:8080"}`,
	}
	naive := &model.Inbound{
		Id: 1, Protocol: model.Naive, Port: 8444, Enable: true, Listen: "127.0.0.1",
		Settings: `{"domain":"n.example.com","authUser":"u","authPass":"p"}`,
	}
	gw := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Port: 443, Enable: true,
		Settings: `{"enabled":true,"publicHost":"pub.example.com","unified":true,"routes":[{"sni":"n.example.com","dest":"127.0.0.1:8444","chan":"naive-1"},{"sni":"cov.example.com","dest":"127.0.0.1:8443","chan":"cover-2"}],"snapshot":[{"inboundId":1},{"inboundId":2}]}`,
	}
	inst, ok := GatewayInstanceFromInbound(gw, []*model.Inbound{naive, cover}, []byte("secret"), cert, key)
	if !ok || !inst.Enabled {
		t.Fatalf("ok=%v enabled=%v", ok, inst.Enabled)
	}
	for _, need := range []string{
		"l4http naive-1",
		"l4http cover-2",
		"bind l4chan/naive-1",
		"bind l4chan/cover-2",
		"forward_proxy",
		"basic_auth \"u\" \"p\"",
	} {
		if !strings.Contains(inst.ConfigText, need) {
			t.Fatalf("missing %q:\n%s", need, inst.ConfigText)
		}
	}
	if inst.FingerprintExtra == "" {
		t.Fatalf("no cert fingerprint")
	}
}

func TestGatewayInstance_UnifiedCoverServesStealDest(t *testing.T) {
	stubGatewayChan(t, true)
	dir := t.TempDir()
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "cov.example.com")
	cover := &model.Inbound{
		Id: 2, Protocol: model.Cover, Port: 8443, Enable: true, Listen: "127.0.0.1",
		Settings: `{"hostname":"cov.example.com","siteSource":"upstream","siteUpstream":"http://127.0.0.1:8080"}`,
	}
	gw := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Port: 443, Enable: true,
		Settings: `{"enabled":true,"unified":true,"routes":[{"sni":"cov.example.com","dest":"127.0.0.1:8443","chan":"cover-2"}],"snapshot":[{"inboundId":2}]}`,
	}
	inst, ok := GatewayInstanceFromInbound(gw, []*model.Inbound{cover}, nil, cert, key)
	if !ok || !inst.Enabled {
		t.Fatalf("ok=%v enabled=%v", ok, inst.Enabled)
	}
	steal := gatewayLoopbackDest(cover.Port)
	host, port, _ := strings.Cut(steal, ":")
	// A bare hostname would also bind 127.0.0.1:443 — the gateway's own port.
	for _, need := range []string{
		"\tbind l4chan/cover-2 " + host + "\n",
		":" + port + ", \"cov.example.com:" + port + "\" {\n",
	} {
		if !strings.Contains(inst.ConfigText, need) {
			t.Fatalf("steal dest %s not served, missing %q:\n%s", steal, need, inst.ConfigText)
		}
	}
}

func TestStandaloneCoverInstance_OffWhileAbsorbed(t *testing.T) {
	stubGatewayChan(t, true)
	dir := t.TempDir()
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "cov.example.com")
	cover := &model.Inbound{
		Id: 2, Protocol: model.Cover, Port: 443, Enable: true, Listen: "127.0.0.1",
		Settings: `{"enabled":true,"hostname":"cov.example.com","siteSource":"upstream","siteUpstream":"http://127.0.0.1:8080"}`,
	}
	gw := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Port: 443, Enable: true,
		Settings: `{"enabled":true,"unified":true,"snapshot":[{"inboundId":2}]}`,
	}
	for _, tc := range []struct {
		name string
		all  []*model.Inbound
		want bool
	}{
		{"alone", []*model.Inbound{cover}, true},
		// Its 127.0.0.1:443 would share the gateway's socket via SO_REUSEPORT.
		{"absorbed by unified gateway", []*model.Inbound{cover, gw}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, ok := StandaloneCoverInstance(cover, tc.all, nil, cert, key)
			if !ok || inst.Enabled != tc.want {
				t.Fatalf("ok=%v enabled=%v, want enabled=%v", ok, inst.Enabled, tc.want)
			}
		})
	}
}

func TestGatewayInstance_LegacyIgnoresChan(t *testing.T) {
	stubGatewayChan(t, true)
	gw := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Port: 443, Enable: true,
		Settings: `{"enabled":true,"routes":[{"sni":"n.example.com","dest":"127.0.0.1:8444","chan":"naive-1"}],"snapshot":[{"inboundId":1}]}`,
	}
	inst, ok := GatewayInstanceFromInbound(gw, nil, nil, "", "")
	if !ok || !inst.Enabled {
		t.Fatalf("ok=%v enabled=%v", ok, inst.Enabled)
	}
	if strings.Contains(inst.ConfigText, "l4http") {
		t.Fatalf("legacy gateway emitted l4http:\n%s", inst.ConfigText)
	}
}

func TestGatewayAbsorbed(t *testing.T) {
	stubGatewayChan(t, true)
	naive := &model.Inbound{
		Id: 1, Protocol: model.Naive, Port: 8444, Listen: "127.0.0.1",
		Settings: `{"domain":"n.example.com"}`,
	}
	gwUnified := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Enable: true,
		Settings: `{"enabled":true,"unified":true,"snapshot":[{"inboundId":1}]}`,
	}
	gwLegacy := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Enable: true,
		Settings: `{"enabled":true,"snapshot":[{"inboundId":1}]}`,
	}
	if !GatewayAbsorbed(naive, []*model.Inbound{gwUnified, naive}) {
		t.Fatalf("unified gateway should absorb naive")
	}
	if GatewayAbsorbed(naive, []*model.Inbound{gwLegacy, naive}) {
		t.Fatalf("legacy gateway must not absorb")
	}
	naive.Listen = ""
	if GatewayAbsorbed(naive, []*model.Inbound{gwUnified, naive}) {
		t.Fatalf("public-listen inbound is never absorbed")
	}
	naive.Listen = "127.0.0.1"
	any := &model.Inbound{
		Id: 3, Protocol: model.Anytls, Port: 8445, Listen: "127.0.0.1",
		Settings: `{"sni":"a.example.com"}`,
	}
	if GatewayAbsorbed(any, []*model.Inbound{gwUnified}) {
		t.Fatalf("passthrough inbound is never absorbed")
	}
}

// TestDumpUnifiedGatewayCaddyfile writes a rendered unified Caddyfile to
// $LUCX_DUMP for manual adapt/run verification with the merged binary.
func TestDumpUnifiedGatewayCaddyfile(t *testing.T) {
	stubGatewayChan(t, true)
	out := os.Getenv("LUCX_DUMP")
	if out == "" {
		t.Skip("LUCX_DUMP not set")
	}
	dir := t.TempDir()
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "cov.test.local", "naive.test.local")
	cover := &model.Inbound{
		Id: 2, Protocol: model.Cover, Port: 8443, Enable: true, Listen: "127.0.0.1",
		Settings: `{"hostname":"cov.test.local","siteSource":"upstream","siteUpstream":"http://127.0.0.1:18080"}`,
	}
	naive := &model.Inbound{
		Id: 1, Protocol: model.Naive, Port: 8444, Enable: true, Listen: "127.0.0.1",
		Settings: `{"domain":"naive.test.local","authUser":"u","authPass":"p"}`,
	}
	any := &model.Inbound{
		Id: 3, Protocol: model.Anytls, Port: 8445, Enable: true, Listen: "127.0.0.1",
		Settings: `{"sni":"any.test.local"}`,
	}
	gw := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Port: 14443, Enable: true, Listen: "127.0.0.1",
		Settings: `{"enabled":true,"publicHost":"pub.test.local","unified":true,"bindIP":"127.0.0.1","routes":[{"sni":"naive.test.local","dest":"127.0.0.1:8444","chan":"naive-1"},{"sni":"cov.test.local","dest":"127.0.0.1:8443","chan":"cover-2"},{"sni":"any.test.local","dest":"127.0.0.1:8445","noProxy":true}],"snapshot":[{"inboundId":1},{"inboundId":2},{"inboundId":3}]}`,
	}
	inst, ok := GatewayInstanceFromInbound(gw, []*model.Inbound{naive, cover, any}, []byte("secret"), cert, key)
	if !ok || !inst.Enabled {
		t.Fatalf("ok=%v enabled=%v", ok, inst.Enabled)
	}
	// cert paths are inside t.TempDir — copy to stable paths for manual runs
	certStable := `C:/Temp/lucx-test.crt`
	keyStable := `C:/Temp/lucx-test.key`
	esc := func(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }
	body := strings.ReplaceAll(inst.ConfigText, esc(cert), esc(certStable))
	body = strings.ReplaceAll(body, esc(key), esc(keyStable))
	cb, _ := os.ReadFile(cert)
	kb, _ := os.ReadFile(key)
	_ = os.WriteFile(certStable, cb, 0o644)
	_ = os.WriteFile(keyStable, kb, 0o644)
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s\n%s", out, body)
}
