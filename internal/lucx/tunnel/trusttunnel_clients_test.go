// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestTrustTunnelClientsScrapeAndDeltas(t *testing.T) {
	body := `[{"username":"tu1","ip":"203.0.113.7","sessions":2,"inbound":100,"outbound":1000},
		{"username":"tu2","ip":null,"sessions":0,"inbound":0,"outbound":0}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clients" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	_, portStr, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ := strconv.Atoi(portStr)

	rows, ok := scrapeTrustTunnelClients(port)
	if !ok || len(rows) != 2 || rows[0].IP == nil || *rows[0].IP != "203.0.113.7" || rows[1].IP != nil {
		t.Fatalf("rows = %+v ok=%v", rows, ok)
	}
	cur := &trustTunnelCursor{}
	first := trustTunnelUserDeltas(cur, rows)
	if first[0].Up != 0 || first[0].Down != 0 || first[0].Sessions != 2 || first[0].IP != "203.0.113.7" {
		t.Fatalf("first scrape must only set the base: %+v", first)
	}
	rows[0].Inbound, rows[0].Outbound = 150, 1600
	rows = append(rows, trustTunnelClientRow{Username: "tu3", Inbound: 5, Outbound: 6})
	got := trustTunnelUserDeltas(cur, rows)
	// inbound (100→150) is the download, outbound (1000→1600) the upload.
	if got[0].Up != 600 || got[0].Down != 50 || got[1].Up != 0 || got[2].Up != 6 || got[2].Down != 5 {
		t.Fatalf("deltas = %+v", got)
	}
	rows[0].Inbound, rows[0].Outbound = 10, 10 // endpoint restarted
	if got := trustTunnelUserDeltas(cur, rows); got[0].Up != 0 || got[0].Down != 0 {
		t.Fatalf("a counter reset must not overcount: %+v", got[0])
	}

	body = "not json"
	if _, ok := scrapeTrustTunnelClients(port); ok {
		t.Fatal("bad JSON accepted")
	}
	srv.Config.Handler = http.NotFoundHandler()
	if _, ok := scrapeTrustTunnelClients(port); ok {
		t.Fatal("404 (per_client_metrics off) must mean no per-user data")
	}
}

func TestTrustTunnelTomlEnablesPerClientMetrics(t *testing.T) {
	toml := TrustTunnelConfig{Hostname: "vpn.example.com", Listen: "127.0.0.1:1447"}.Merge().
		RenderVpnToml("/c", "/r", "", "127.0.0.1:14073")
	if !strings.Contains(toml, "[metrics]\naddress = \"127.0.0.1:14073\"\nrequest_timeout_secs = 3\nper_client_metrics = true\n") {
		t.Fatalf("toml = %s", toml)
	}
	if strings.Contains(TrustTunnelConfig{Hostname: "vpn.example.com"}.Merge().RenderVpnToml("/c", "/r", "", ""), "per_client_metrics") {
		t.Fatal("per_client_metrics without a metrics listener")
	}
}
