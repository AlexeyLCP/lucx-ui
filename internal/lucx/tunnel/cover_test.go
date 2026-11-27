// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRenderCoverCaddyfile_Site(t *testing.T) {
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{publicDir: "/var/www/site"})
	for _, need := range []string{
		"admin off", "auto_https off", "skip_install_trust",
		":80", "redir https://{host}{uri} permanent",
		"shop.example.com:443", "tls", "file_server", `root * "/var/www/site"`,
	} {
		if !strings.Contains(got, need) {
			t.Fatalf("missing %q in:\n%s", need, got)
		}
	}
	if strings.Contains(got, "proxy_protocol") {
		t.Fatalf("public cover must not wrap PROXY:\n%s", got)
	}
}

func TestGatewayPanelRoutes(t *testing.T) {
	gw := &model.Inbound{
		Protocol: model.Gateway, Enable: true,
		Settings: `{"snapshot":[{"inboundId":1}],"hidePanel":true,"panelRoutes":[{"path":"/p","dest":"127.0.0.1:2053"}]}`,
	}
	got := gatewayPanelRoutes([]*model.Inbound{gw})
	if len(got) != 1 || got[0].Path != "/p" || got[0].Dest != "127.0.0.1:2053" {
		t.Fatalf("%+v", got)
	}
}

func TestRenderCoverCaddyfile_PanelHTTPS(t *testing.T) {
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{
		publicDir: "/var/www/site",
		routes:    []CoverRoute{{Path: "/abc123", Dest: "https://127.0.0.1:2053"}},
	})
	for _, need := range []string{"handle /abc123*", "reverse_proxy https://127.0.0.1:2053", "tls_insecure_skip_verify"} {
		if !strings.Contains(got, need) {
			t.Fatalf("missing %q in:\n%s", need, got)
		}
	}
}

func TestRenderCoverCaddyfile_LoopbackBind(t *testing.T) {
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{
		publicDir: "/var/www/site", skipHTTP: true, httpsPort: 8443,
	})
	if strings.Contains(got, ":80") {
		t.Fatalf("loopback cover still has :80:\n%s", got)
	}
	if !strings.Contains(got, "bind 127.0.0.1") || !strings.Contains(got, ":8443") {
		t.Fatalf("missing bind/port:\n%s", got)
	}
	if !strings.Contains(got, "proxy_protocol") {
		t.Fatalf("loopback cover needs PROXY protocol:\n%s", got)
	}
	if !strings.Contains(got, "protocols h1 h2") {
		t.Fatalf("loopback cover must pin h1/h2:\n%s", got)
	}
}

func TestRenderCoverCaddyfile_TproxyWins(t *testing.T) {
	naive := DefaultNaiveConfig()
	naive.AuthUser = "u"
	naive.AuthPass = "p"
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{
		tproxyRelay:    24002,
		naives:         []coverNaiveAttachment{{config: naive}},
		publicDir:      "/var/www/site",
		routes:         []CoverRoute{{Path: "/p", Dest: "https://127.0.0.1:2053"}},
		publicUpstream: "http://127.0.0.1:3000",
	})
	for _, need := range []string{"reverse_proxy 127.0.0.1:24002", "header_down -Via", "header_down -Server", "protocols h1 h2", "encode @site_pages zstd gzip", "handle /p*", "https://127.0.0.1:2053", "header_up Host {http.request.host}"} {
		if !strings.Contains(got, need) {
			t.Fatalf("tproxy caddy missing %q:\n%s", need, got)
		}
	}
	for _, no := range []string{"file_server", "forward_proxy", "127.0.0.1:3000"} {
		if strings.Contains(got, no) {
			t.Fatalf("tproxy must own the host, found %q in:\n%s", no, got)
		}
	}
}

func TestEnsureDefaultDecoy(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDefaultDecoy(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil || !strings.Contains(string(b), "Welcome to nginx!") {
		t.Fatalf("seed: %v %q", err, b)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDefaultDecoy(dir); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "index.html"))
	if string(b) != "mine" {
		t.Fatalf("overwrote upload: %q", b)
	}
}

func TestRenderCoverCaddyfile_NaiveAndPath(t *testing.T) {
	naive := DefaultNaiveConfig()
	naive.AuthUser = "u"
	naive.AuthPass = "p"
	naive.ProbeResistance = true
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{
		naives:    []coverNaiveAttachment{{config: naive}},
		publicDir: "/site",
		routes:    []CoverRoute{{Path: "/ws", Dest: "127.0.0.1:10000"}},
	})
	for _, need := range []string{
		`:443, "shop.example.com"`, "forward_proxy", "basic_auth",
		"handle /ws*", "reverse_proxy 127.0.0.1:10000",
		"file_server", `root * "/site"`, "encode @site_pages zstd gzip",
	} {
		if !strings.Contains(got, need) {
			t.Fatalf("missing %q in:\n%s", need, got)
		}
	}
	if strings.Contains(got, "protocols h1 h2") {
		t.Fatalf("H3 on: cover must not restrict protocols:\n%s", got)
	}
	if strings.Contains(got, "shop.example.com:443") {
		t.Fatalf("host:443 site address kills naive padding:\n%s", got)
	}
}

func TestCoverInstanceBehindCoverSkipsOwnCaddy(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })

	site := CoverSiteDir(3)
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "shop.example.com")

	cover := &model.Inbound{
		Id: 3, Protocol: model.Cover, Enable: true, Port: 443,
		Settings: `{"hostname":"shop.example.com","siteSource":"zip"}`,
	}
	naive := &model.Inbound{
		Id: 4, Protocol: model.Naive, Enable: true, Port: 443,
		Settings: `{"domain":"shop.example.com","behindCover":true,"clients":[{"email":"a@x","enable":true}]}`,
	}
	tproxy := &model.Inbound{
		Id: 5, Protocol: model.Tproxy, Enable: true, Port: 443,
		Settings: `{"hostname":"shop.example.com","secret":"000102030405060708090a0b0c0d0e0f","behindCover":true,"siteSource":"zip"}`,
	}

	if !NaiveFrontedByCover(naive, []*model.Inbound{cover, naive, tproxy}, []byte("secret"), cert, key) {
		t.Fatal("dead tproxy must not steal naive from cover")
	}

	insts, ok := TproxyInstancesFromInbound(tproxy, cert, key)
	if !ok {
		t.Fatal("tproxy instances")
	}
	for _, inst := range insts {
		if inst.Enabled {
			t.Fatal("tproxy without site must stay down")
		}
	}

	cInst, ok := CoverInstanceFromInbound(cover, []*model.Inbound{naive, tproxy}, []byte("secret"), cert, key)
	if !ok || !cInst.Enabled {
		t.Fatalf("cover instance: enabled=%v ok=%v", cInst.Enabled, ok)
	}
	if !strings.Contains(cInst.ConfigText, "forward_proxy") {
		t.Fatalf("cover must keep naive when tproxy is down:\n%s", cInst.ConfigText)
	}
	if strings.Contains(cInst.ConfigText, "reverse_proxy 127.0.0.1:"+strconv.Itoa(tproxyLoopback(tproxy.Id, 2))) {
		t.Fatal("dead tproxy must not own cover")
	}
}

func TestCoverInstanceKeepsAllNaivesBehindCover(t *testing.T) {
	dir := t.TempDir()
	const host = "site.example.com"
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), host)
	cover := &model.Inbound{
		Id: 11, Protocol: model.Cover, Enable: true, Port: 443,
		Settings: `{"hostname":"site.example.com","siteSource":"upstream","siteUpstream":"http://127.0.0.1:8080"}`,
	}
	secret := []byte("test-secret")
	var all []*model.Inbound
	for i := 0; i < 3; i++ {
		id, bridge := 12+i, 34401+i
		ib := &model.Inbound{
			Id: id, Protocol: model.Naive, Enable: true, Port: 443,
			Tag:      "naive-" + strconv.Itoa(id),
			Settings: `{"domain":"site.example.com","behindCover":true,"probeResistance":true,"routeThroughXray":true,"routeXrayPort":` + strconv.Itoa(bridge) + `,"clients":[{"email":"same@x","enable":true}]}`,
		}
		all = append(all, ib)
	}
	// Supply the rows in reverse order to check stable render ordering.
	others := []*model.Inbound{all[2], all[1], all[0], cover}
	inst, ok := CoverInstanceFromInbound(cover, others, secret, cert, key)
	if !ok || !inst.Enabled {
		t.Fatal("cover is not enabled")
	}
	blocks := strings.Split(inst.ConfigText, "forward_proxy @naiveConnect {")
	if len(blocks) != 4 {
		t.Fatalf("expected three handlers, got %d", len(blocks)-1)
	}
	for i, ib := range all {
		pair := InboundAuthPair(secret, ib, "same@x")
		if !strings.Contains(blocks[i+1], "basic_auth "+caddyToken(pair.User)+" "+caddyToken(pair.Pass)) ||
			!strings.Contains(blocks[i+1], "@127.0.0.1:"+strconv.Itoa(34401+i)) {
			t.Fatalf("wrong credentials or bridge in handler %d", i)
		}
		if !NaiveFrontedByCover(ib, others, secret, cert, key) {
			t.Fatalf("inbound #%d not attached", ib.Id)
		}
	}
	if !strings.Contains(inst.ConfigText, "reverse_proxy 127.0.0.1:8080") {
		t.Fatal("ordinary Cover site was lost")
	}
	all[1].Enable = false
	inst, ok = CoverInstanceFromInbound(cover, others, secret, cert, key)
	if !ok || !inst.Enabled || strings.Count(inst.ConfigText, "forward_proxy @naiveConnect {") != 2 {
		t.Fatal("disabling one inbound must keep the other two")
	}
	if NaiveFrontedByCover(all[1], others, secret, cert, key) {
		t.Fatal("disabled inbound is still attached")
	}
	all[2].Settings = strings.ReplaceAll(all[2].Settings, `"enable":true`, `"enable":false`)
	inst, ok = CoverInstanceFromInbound(cover, others, secret, cert, key)
	if !ok || !inst.Enabled || strings.Count(inst.ConfigText, "forward_proxy {") != 1 {
		t.Fatal("revoking the last client must remove its handler")
	}
	if NaiveFrontedByCover(all[2], others, secret, cert, key) {
		t.Fatal("inbound without credentials is still attached")
	}
}

func TestValidateCoverNaives(t *testing.T) {
	const host = "site.example.com"
	makeInbound := func(id int, settings string) *model.Inbound {
		return &model.Inbound{
			Id: id, Protocol: model.Naive, Enable: true, Port: 443,
			Tag: "naive-" + strconv.Itoa(id), Settings: settings,
		}
	}
	first := makeInbound(1, `{"domain":"site.example.com","behindCover":true,"authUser":"u1","authPass":"p1","probeResistance":true,"routeThroughXray":true,"routeXrayPort":34401}`)
	second := makeInbound(2, `{"domain":"site.example.com","behindCover":true,"authUser":"u2","authPass":"p2","probeResistance":true,"routeThroughXray":true,"routeXrayPort":34402}`)
	base := second.Settings
	for _, tt := range []struct {
		name, settings, want string
	}{
		{"valid", base, ""},
		{"duplicate credentials", strings.ReplaceAll(strings.ReplaceAll(base, "u2", "u1"), "p2", "p1"), "duplicate credentials"},
		{"duplicate port", strings.ReplaceAll(base, "34402", "34401"), "duplicate Xray bridge port"},
		{"missing bridge", strings.ReplaceAll(base, "34402", "0"), "missing Xray bridge"},
		{"authorization fallback", strings.ReplaceAll(base, `"probeResistance":true`, `"probeResistance":false`), "incompatible authorization"},
		{"incomplete credentials", strings.ReplaceAll(base, `"p2"`, `""`), "incomplete credentials"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			second.Settings = tt.settings
			err := ValidateCoverNaives(host, []*model.Inbound{first, second}, []byte("test-secret"))
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
	second.Settings = `{"domain":"site.example.com","behindCover":true,"clients":[{"email":"x","enable":false}]}`
	if got := coverNaivesFor(host, []*model.Inbound{first, second}, []byte("test-secret")); len(got) != 1 {
		t.Fatal("an inbound without credentials must not render a handler")
	}
}

func TestCoverTproxyWinsWhenStackReady(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })
	stubMtproxyBinary(t, dir)

	if err := os.MkdirAll(mtproxyAssetsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mtproxyAssetsDir(), "proxy-secret"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mtproxyAssetsDir(), "proxy-multi.conf"), []byte("c"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, site := range []string{CoverSiteDir(3), TproxySiteDir(5)} {
		if err := os.MkdirAll(site, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html/>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "shop.example.com")
	cover := &model.Inbound{
		Id: 3, Protocol: model.Cover, Enable: true, Port: 443,
		Settings: `{"hostname":"shop.example.com","siteSource":"zip"}`,
	}
	naive := &model.Inbound{
		Id: 4, Protocol: model.Naive, Enable: true, Port: 443,
		Settings: `{"domain":"shop.example.com","behindCover":true,"clients":[{"email":"a@x","enable":true}]}`,
	}
	tproxy := &model.Inbound{
		Id: 5, Protocol: model.Tproxy, Enable: true, Port: 443,
		Settings: `{"hostname":"shop.example.com","secret":"000102030405060708090a0b0c0d0e0f","behindCover":true,"siteSource":"zip"}`,
	}

	if NaiveFrontedByCover(naive, []*model.Inbound{cover, naive, tproxy}, nil, cert, key) {
		t.Fatal("live tproxy must own the host")
	}
	cInst, ok := CoverInstanceFromInbound(cover, []*model.Inbound{naive, tproxy}, nil, cert, key)
	if !ok || !cInst.Enabled {
		t.Fatalf("cover instance: enabled=%v ok=%v", cInst.Enabled, ok)
	}
	if !strings.Contains(cInst.ConfigText, "reverse_proxy 127.0.0.1:"+strconv.Itoa(tproxyLoopback(5, 2))) {
		t.Fatalf("cover should proxy tproxy, got:\n%s", cInst.ConfigText)
	}
	if strings.Contains(cInst.ConfigText, "forward_proxy") {
		t.Fatal("tproxy hostname must not keep naive")
	}
}

func TestCoverAttachesNaiveEmptyDomain(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })

	site := CoverSiteDir(3)
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "shop.example.com")
	cover := &model.Inbound{
		Id: 3, Protocol: model.Cover, Enable: true, Port: 443,
		Settings: `{"hostname":"shop.example.com","siteSource":"zip"}`,
	}
	naive := &model.Inbound{
		Id: 4, Protocol: model.Naive, Enable: true, Port: 443,
		Settings: `{"behindCover":true,"clients":[{"email":"a@x","enable":true}]}`,
	}
	cInst, ok := CoverInstanceFromInbound(cover, []*model.Inbound{naive}, []byte("secret"), cert, key)
	if !ok || !cInst.Enabled {
		t.Fatalf("cover: %+v ok=%v", cInst, ok)
	}
	if !strings.Contains(cInst.ConfigText, "forward_proxy") {
		t.Fatalf("empty naive domain must still attach:\n%s", cInst.ConfigText)
	}
}

func TestNaiveMatchesCover_EmptyDomain(t *testing.T) {
	n := DefaultNaiveConfig()
	n.BehindCover = true
	if !naiveMatchesCover(n, "shop.example.com") {
		t.Fatal("empty domain must match cover hostname")
	}
	n.Domain = "other.example.com"
	if naiveMatchesCover(n, "shop.example.com") {
		t.Fatal("mismatch")
	}
	n.Domain = "shop.example.com"
	if !naiveMatchesCover(n, "shop.example.com") {
		t.Fatal("same hostname")
	}
}

func TestSettingsBehindCover(t *testing.T) {
	if !SettingsBehindCover(model.Naive, `{"behindCover":true}`) {
		t.Fatal("naive")
	}
	if SettingsBehindCover(model.Naive, `{}`) {
		t.Fatal("empty")
	}
	if SettingsBehindCover(model.Cover, `{"behindCover":true}`) {
		t.Fatal("cover itself")
	}
}

func TestCoverInstanceFromInbound_PanelRoutes(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })
	site := CoverSiteDir(3)
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "shop.example.com")
	cover := &model.Inbound{
		Id: 3, Protocol: model.Cover, Enable: true, Port: 443,
		Settings: `{"hostname":"shop.example.com","siteSource":"zip"}`,
	}
	gw := &model.Inbound{
		Id: 9, Protocol: model.Gateway, Enable: true, Port: 443,
		Settings: `{"snapshot":[{"inboundId":3}],"hidePanel":true,"panelRoutes":[{"path":"/abc","dest":"https://127.0.0.1:2053"}]}`,
	}
	cInst, ok := CoverInstanceFromInbound(cover, []*model.Inbound{gw}, nil, cert, key)
	if !ok || !cInst.Enabled {
		t.Fatalf("cover: %+v ok=%v", cInst, ok)
	}
	if !strings.Contains(cInst.ConfigText, "handle /abc*") || !strings.Contains(cInst.ConfigText, "https://127.0.0.1:2053") {
		t.Fatalf("panel routes:\n%s", cInst.ConfigText)
	}
}
