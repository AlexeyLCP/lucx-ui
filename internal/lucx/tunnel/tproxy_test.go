// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestTproxySiteDirOutsideDataDir(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })

	site := TproxySiteDir(18)
	data := dataDirFor(TproxyKey(18), Tproxy)
	if site == data || strings.HasPrefix(site, data+string(os.PathSeparator)) {
		t.Fatalf("site dir %q must not live under wiped data dir %q", site, data)
	}
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeManagedFiles(TproxyKey(18))
	if _, err := os.Stat(filepath.Join(site, "index.html")); err != nil {
		t.Fatalf("save/update Remove must not delete the camouflage site: %v", err)
	}
	RemoveTproxySite(18)
	if _, err := os.Stat(site); !os.IsNotExist(err) {
		t.Fatalf("inbound delete must remove the site, stat=%v", err)
	}
}

func TestExtractTproxySiteZipReplaces(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "site")
	writeZip := func(body string) []byte {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.Create("index.html")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
		_ = zw.Close()
		return buf.Bytes()
	}
	if err := ExtractTproxySiteZip(dest, writeZip("old")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "gone.css"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ExtractTproxySiteZip(dest, writeZip("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "index.html"))
	if err != nil || string(got) != "new" {
		t.Fatalf("index.html = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "gone.css")); !os.IsNotExist(err) {
		t.Fatal("old extra file must be gone after a new zip")
	}
}

func TestTproxyConfigRouteThroughXray(t *testing.T) {
	ib := &model.Inbound{
		Protocol: model.Tproxy,
		Enable:   true,
		Settings: `{"hostname":"proxy.example.com","secret":"000102030405060708090a0b0c0d0e0f","routeThroughXray":true,"routeXrayPort":39111,"outboundTag":"warp"}`,
	}
	cfg, ok := TproxyConfigFromInbound(ib)
	if !ok || !cfg.RouteThroughXray || cfg.RouteXrayPort != 39111 || cfg.OutboundTag != "warp" {
		t.Fatalf("got %+v ok=%v", cfg, ok)
	}
	plain := *ib
	plain.Settings = `{"hostname":"proxy.example.com","secret":"000102030405060708090a0b0c0d0e0f"}`
	cfg, ok = TproxyConfigFromInbound(&plain)
	if !ok || cfg.RouteThroughXray {
		t.Fatal("absent routeThroughXray must stay false")
	}
}

func TestTproxyClientLink(t *testing.T) {
	cfg := TproxyConfig{Hostname: "proxy.example.com", Secret: "000102030405060708090a0b0c0d0e0f"}
	got := cfg.ClientLink()
	if got != "https://t.me/webproxy?server=proxy.example.com&secret=000102030405060708090a0b0c0d0e0f" {
		t.Fatalf("link = %q", got)
	}
}

func TestTproxyValidate(t *testing.T) {
	cfg := DefaultTproxyConfig()
	cfg.Hostname = "Proxy.Example.COM"
	cfg.Secret = "000102030405060708090a0b0c0d0e0f"
	cfg = cfg.Merge()
	if cfg.Hostname != "proxy.example.com" {
		t.Fatalf("hostname = %q", cfg.Hostname)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Hostname = "https://evil"
	if err := cfg.Validate(); err == nil {
		t.Fatal("scheme in hostname must fail")
	}
	cfg.Hostname = "proxy.example.com"
	cfg.Secret = "zz"
	if err := cfg.Validate(); err == nil {
		t.Fatal("short secret must fail")
	}
	cfg.Secret = "000102030405060708090a0b0c0d0e0f"
	cfg.SiteSource = "upstream"
	cfg.SiteUpstream = "http://10.0.0.1:80"
	if err := cfg.Validate(); err == nil {
		t.Fatal("non-loopback upstream must fail")
	}
	cfg.SiteUpstream = "http://127.0.0.1:3000"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTproxyEnsureSecretStable(t *testing.T) {
	cfg := DefaultTproxyConfig()
	out, err := cfg.EnsureSecret()
	if err != nil || len(out.Secret) != 32 {
		t.Fatalf("EnsureSecret: %v %q", err, out.Secret)
	}
	again, err := out.EnsureSecret()
	if err != nil || again.Secret != out.Secret {
		t.Fatal("EnsureSecret must not rotate")
	}
}

func TestRenderTproxyCaddyfile(t *testing.T) {
	got := RenderTproxyCaddyfile("proxy.example.com", 443, "/c.pem", "/k.pem", 24002)
	for _, need := range []string{"admin off", "auto_https off", "proxy.example.com:443", "tls", "reverse_proxy 127.0.0.1:24002", "response_header_timeout 40s"} {
		if !strings.Contains(got, need) {
			t.Fatalf("caddyfile missing %q:\n%s", need, got)
		}
	}
}

func TestExtractTproxySiteZip(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "site")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("index.html")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("<html>ok</html>"))
	w, err = zw.Create("styles.css")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("body{}"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ExtractTproxySiteZip(dest, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := RequireIndexHTML(dest); err != nil {
		t.Fatal(err)
	}

	var slip bytes.Buffer
	zw = zip.NewWriter(&slip)
	w, err = zw.Create("../evil.html")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	if err := ExtractTproxySiteZip(filepath.Join(dir, "slip"), slip.Bytes()); err == nil {
		t.Fatal("zip-slip must fail")
	}
}

func TestMtproxyArgsAlwaysDropUser(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })

	if err := os.MkdirAll(mtproxyAssetsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mtproxyAssetsDir(), "proxy-secret"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mtproxyAssetsDir(), "proxy-multi.conf"), []byte("c"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := mtproxyArgs(24001, 24000, "000102030405060708090a0b0c0d0e0f")
	for i, a := range args {
		if a == "-u" && i+1 < len(args) && strings.TrimSpace(args[i+1]) != "" {
			return
		}
	}
	t.Fatalf("mtproxy args must carry -u <user> (engine default user must exist): %v", args)
}

func TestTproxyInstancesMissingSiteDisables(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })

	if err := os.MkdirAll(mtproxyAssetsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mtproxyAssetsDir(), "proxy-secret"), []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mtproxyAssetsDir(), "proxy-multi.conf"), []byte("c"), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "proxy.example.com")

	// siteSource=zip but no uploaded site dir → all three slots disabled.
	ib := &model.Inbound{
		Id:       5,
		Protocol: model.Tproxy,
		Enable:   true,
		Port:     443,
		Settings: `{"hostname":"proxy.example.com","secret":"000102030405060708090a0b0c0d0e0f","siteSource":"zip"}`,
	}
	insts, ok := TproxyInstancesFromInbound(ib, cert, key)
	if !ok || len(insts) != 3 {
		t.Fatalf("len=%d ok=%v", len(insts), ok)
	}
	for _, inst := range insts {
		if inst.Enabled {
			t.Fatalf("missing site must disable the stack: %+v", inst)
		}
	}
}

func TestTproxyInstancesFromInbound(t *testing.T) {
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
	site := TproxySiteDir(4)
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<html/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "proxy.example.com")

	ib := &model.Inbound{
		Id:       4,
		Protocol: model.Tproxy,
		Enable:   true,
		Port:     443,
		Settings: `{"hostname":"proxy.example.com","secret":"000102030405060708090a0b0c0d0e0f","siteSource":"zip"}`,
	}
	insts, ok := TproxyInstancesFromInbound(ib, cert, key)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(insts) != 3 {
		t.Fatalf("len=%d", len(insts))
	}
	if insts[0].Core != Mtproxy || insts[1].Core != Tproxy || insts[2].Core != TproxyCaddy {
		t.Fatalf("cores = %v %v %v", insts[0].Core, insts[1].Core, insts[2].Core)
	}
	for _, inst := range insts {
		if !inst.Enabled {
			t.Fatalf("want enabled: %+v", inst)
		}
	}
	if insts[1].Key != "tproxy-4" || insts[2].Key != "tproxycaddy-4" {
		t.Fatalf("keys %q %q", insts[1].Key, insts[2].Key)
	}
	if !strings.Contains(insts[2].ConfigText, "reverse_proxy 127.0.0.1:") {
		t.Fatalf("caddyfile = %s", insts[2].ConfigText)
	}

	off := *ib
	off.Enable = false
	insts, ok = TproxyInstancesFromInbound(&off, cert, key)
	if !ok || len(insts) != 3 || insts[0].Enabled {
		t.Fatalf("disabled must yield Enabled:false: %+v", insts)
	}
	if _, ok := TproxyInstancesFromInbound(&model.Inbound{Protocol: model.VLESS}, cert, key); ok {
		t.Fatal("non-tproxy must not map")
	}
}
