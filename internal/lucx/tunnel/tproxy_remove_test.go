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
)

func TestRemoveTproxyInstall(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tproxy-server")
	site := filepath.Join(root, "tproxy-site")
	nginx := filepath.Join(root, "sites-enabled")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nginx, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"listen":"127.0.0.1:18080","public_hostname":"n9.example","public_dir":"` + filepath.ToSlash(site) + `"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	vhost := "server { listen 443 ssl; location / { proxy_pass http://127.0.0.1:18080; } }\n"
	if err := os.WriteFile(filepath.Join(nginx, "webproxy"), []byte(vhost), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "sites-available")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	shared := "server { location /a { proxy_pass http://127.0.0.1:18080; } location /b { proxy_pass http://127.0.0.1:9; } }\n"
	if err := os.WriteFile(filepath.Join(other, "shared"), []byte(shared), 0o644); err != nil {
		t.Fatal(err)
	}
	var cmds []string
	err := removeTproxyInstall(filepath.Join(dir, "config.json"), tproxyRemoveOpts{
		nginxRoots: []string{nginx, other},
		run: func(name string, args ...string) error {
			cmds = append(cmds, name+" "+strings.Join(args, " "))
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("config dir still there: %v", err)
	}
	if _, err := os.Stat(site); !os.IsNotExist(err) {
		t.Fatalf("site dir still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nginx, "webproxy")); !os.IsNotExist(err) {
		t.Fatal("nginx vhost still there")
	}
	if _, err := os.Stat(filepath.Join(other, "shared")); err != nil {
		t.Fatal("shared nginx site was removed")
	}
	joined := strings.Join(cmds, "\n")
	if !strings.Contains(joined, "systemctl disable --now tproxy-server.service") {
		t.Fatalf("units not stopped:\n%s", joined)
	}
	if !strings.Contains(joined, "systemctl reload nginx") {
		t.Fatalf("nginx not reloaded:\n%s", joined)
	}
}

func TestRemoveTproxyInstallRefusesOtherDir(t *testing.T) {
	dir := t.TempDir()
	if err := removeTproxyInstall(filepath.Join(dir, "config.json"), tproxyRemoveOpts{
		run: func(string, ...string) error { t.Fatal("ran a command"); return nil },
	}); err == nil {
		t.Fatal("expected refuse")
	}
}
