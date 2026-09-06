// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os"
	"path/filepath"
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
}

func TestRenderCoverCaddyfile_TproxyWins(t *testing.T) {
	naive := DefaultNaiveConfig()
	naive.AuthUser = "u"
	naive.AuthPass = "p"
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{
		tproxyRelay:    24002,
		naive:          &naive,
		publicDir:      "/var/www/site",
		routes:         []CoverRoute{{Path: "/ws", Dest: "127.0.0.1:10000"}},
		publicUpstream: "http://127.0.0.1:3000",
	})
	for _, need := range []string{"reverse_proxy 127.0.0.1:24002", "header -Via", "protocols h1 h2", "encode zstd gzip"} {
		if !strings.Contains(got, need) {
			t.Fatalf("tproxy caddy missing %q:\n%s", need, got)
		}
	}
	for _, no := range []string{"file_server", "forward_proxy", "handle /ws", "127.0.0.1:3000"} {
		if strings.Contains(got, no) {
			t.Fatalf("tproxy must own the host, found %q in:\n%s", no, got)
		}
	}
}

func TestRenderCoverCaddyfile_NaiveAndPath(t *testing.T) {
	naive := DefaultNaiveConfig()
	naive.AuthUser = "u"
	naive.AuthPass = "p"
	naive.ProbeResistance = true
	got := RenderCoverCaddyfile("shop.example.com", "/c.pem", "/k.pem", coverAttach{
		naive:     &naive,
		publicDir: "/site",
		routes:    []CoverRoute{{Path: "/ws", Dest: "127.0.0.1:10000"}},
	})
	for _, need := range []string{
		`:443, "shop.example.com"`, "forward_proxy", "basic_auth",
		"handle /ws*", "reverse_proxy 127.0.0.1:10000",
		"file_server", `root * "/site"`, "encode zstd gzip",
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

	if !NaiveFrontedByCover(naive, []*model.Inbound{cover, naive, tproxy}, nil, cert, key) {
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

	cInst, ok := CoverInstanceFromInbound(cover, []*model.Inbound{naive, tproxy}, nil, cert, key)
	if !ok || !cInst.Enabled {
		t.Fatalf("cover instance: enabled=%v ok=%v", cInst.Enabled, ok)
	}
	if !strings.Contains(cInst.ConfigText, "forward_proxy") {
		t.Fatalf("cover must keep naive when tproxy is down:\n%s", cInst.ConfigText)
	}
	if strings.Contains(cInst.ConfigText, "reverse_proxy 127.0.0.1:") {
		t.Fatal("dead tproxy must not own cover")
	}
}

func TestCoverTproxyWinsWhenStackReady(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })

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
	if !strings.Contains(cInst.ConfigText, "reverse_proxy 127.0.0.1:") {
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
	cInst, ok := CoverInstanceFromInbound(cover, []*model.Inbound{naive}, nil, cert, key)
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
