// Copyright (c) 2026 LucX-UI Project.
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

func TestCoverLocalSiteLoginPageAndHeaders(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "404.html"), []byte("nf"), 0o644)
	a := coverAttach{
		publicDir: dir, httpsPort: 8443, localPort: 18081,
		routes: []CoverRoute{{Path: "/ws", Dest: "127.0.0.1:1443"}},
	}
	out := RenderCoverCaddyfile("site.example.com", "/c.pem", "/k.pem", a)
	for _, want := range []string{
		"servers 127.0.0.1:18081 {\n\t\tprotocols h1\n",
		"servers 127.0.0.1:18082 {\n\t\tprotocols h2c\n",
		"http://:18081 {\n\tbind 127.0.0.1\n",
		"http://:18082 {\n\tbind 127.0.0.1\n",
		"rewrite * /404.html",
		"header -Server",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "nginx") {
		t.Errorf("must not claim nginx:\n%s", out)
	}
	local := out[strings.Index(out, "http://:18081"):]
	if strings.Contains(local, "handle /ws") {
		t.Errorf("path routes must stay off the local site:\n%s", local)
	}

	a.loginPage = true
	out = RenderCoverCaddyfile("site.example.com", "/c.pem", "/k.pem", a)
	for _, want := range []string{"@post method POST", "rewrite * /error.html", "rewrite * /index.html", "@file file"} {
		if !strings.Contains(out, want) {
			t.Errorf("login page missing %q:\n%s", want, out)
		}
	}
}

func TestUnifiedGatewayOwnsPort80OnItsIPs(t *testing.T) {
	out := RenderUnifiedGatewayCaddyfile(443, []GatewayRoute{{SNI: "a.example.com", Chan: "cover-1"}}, "cover-1",
		[]string{"203.0.113.5", "203.0.113.6"}, []string{":8443, a.example.com:8443 {\n}"}, "\tservers 127.0.0.1:18081 {\n\t\tprotocols h1\n\t}\n")
	for _, want := range []string{"http:// {\n\tbind 203.0.113.5 203.0.113.6\n", "acme-challenge", "redir https://{host}{uri} permanent", "servers 127.0.0.1:18081"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if g, l := strings.Index(out, "servers 127.0.0.1:18081"), strings.Index(out, "layer4"); g < 0 || g > l {
		t.Errorf("local servers must be in the global block before layer4:\n%s", out)
	}
	if none := RenderUnifiedGatewayCaddyfile(443, nil, "", []string{"203.0.113.5"}, nil); strings.Contains(none, "http://") {
		t.Errorf("no site, no :80 block:\n%s", none)
	}
}

func TestStandaloneCoverBindsItsOwnIP(t *testing.T) {
	out := RenderCoverCaddyfile("site.example.com", "/c.pem", "/k.pem", coverAttach{httpsPort: 443, listenIP: "203.0.113.7"})
	if c := strings.Count(out, "\tbind 203.0.113.7\n"); c != 2 {
		t.Errorf("want :80 and :443 bound to the inbound IP (2 binds), got %d:\n%s", c, out)
	}
}

func TestGatewayPort80CoversPanelIP(t *testing.T) {
	out := renderUnifiedGateway(443, []GatewayRoute{{SNI: "a.example.com", Chan: "cover-1"}}, "cover-1",
		[]string{"203.0.113.5"}, []string{"203.0.113.5", "203.0.113.8"}, []string{":8443, a.example.com:8443 {\n}"})
	if !strings.Contains(out, "http:// {\n\tbind 203.0.113.5 203.0.113.8\n") {
		t.Errorf(":80 must cover the panel IP too:\n%s", out)
	}
	if strings.Contains(out, "203.0.113.8:443") {
		t.Errorf("the panel IP's :443 must stay the panel's:\n%s", out)
	}
}
