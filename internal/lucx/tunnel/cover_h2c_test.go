// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestCoverRoutesReachXHTTPOverH2C(t *testing.T) {
	others := []*model.Inbound{
		{Id: 2, Enable: true, Listen: "127.0.0.1", Port: 1443, Protocol: model.VLESS, StreamSettings: `{"network":"ws","security":"none"}`},
		{Id: 30, Enable: true, Listen: "127.0.0.1", Port: 1446, Protocol: model.VLESS, StreamSettings: `{"network":"xhttp","security":"none"}`},
	}
	routes := markH2CRoutes([]CoverRoute{
		{Path: "/ws", Dest: "127.0.0.1:1443"},
		{Path: "/xh", Dest: "127.0.0.1:1446"},
	}, others)
	if routes[0].H2C || !routes[1].H2C {
		t.Fatalf("want only the XHTTP route on h2c, got %+v", routes)
	}

	var b strings.Builder
	writeHTTPPanelRoutes(&b, routes, "\t")
	out := b.String()
	ws := out[:strings.Index(out, "handle /xh*")]
	xh := out[strings.Index(out, "handle /xh*"):]
	if !strings.Contains(ws, "reverse_proxy 127.0.0.1:1443 {") || strings.Contains(ws, "h2c") {
		t.Errorf("WS route must stay HTTP/1.1 (upgrade):\n%s", ws)
	}
	for _, want := range []string{"reverse_proxy h2c://127.0.0.1:1446 {", "flush_interval -1", "versions h2c 2", "read_timeout 0", "write_timeout 0"} {
		if !strings.Contains(xh, want) {
			t.Errorf("XHTTP route missing %q:\n%s", want, xh)
		}
	}
	if !strings.Contains(ws, "flush_interval -1") {
		t.Errorf("WS route must flush at once:\n%s", ws)
	}
}

func TestCoverNeverCompressesPathRoutes(t *testing.T) {
	out := RenderCoverSite("site.example.com", "/c.pem", "/k.pem", coverAttach{
		httpsPort: 8443, bind: "l4chan/cover-1 127.0.0.1",
		routes: []CoverRoute{{Path: "/xh", Dest: "127.0.0.1:1446", H2C: true}, {Path: "/ws*", Dest: "127.0.0.1:1443"}},
	})
	if !strings.Contains(out, "@site_pages not path /xh* /ws*\n") || !strings.Contains(out, "encode @site_pages zstd gzip\n") {
		t.Fatalf("proxy paths must be excluded from encode:\n%s", out)
	}
	if strings.Contains(out, "\tencode zstd gzip\n") {
		t.Fatalf("no unconditional encode when the site has routes:\n%s", out)
	}
	plain := RenderCoverSite("site.example.com", "/c.pem", "/k.pem", coverAttach{httpsPort: 8443})
	if !strings.Contains(plain, "\tencode zstd gzip\n") {
		t.Fatalf("a site without routes still compresses its pages:\n%s", plain)
	}
}
