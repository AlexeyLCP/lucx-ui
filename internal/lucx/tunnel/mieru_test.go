// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMieruValidate(t *testing.T) {
	cases := []struct {
		name    string
		mut     func(*MieruConfig)
		wantErr bool
	}{
		{"default ok", func(c *MieruConfig) {}, false},
		{"no bindings", func(c *MieruConfig) { c.PortBindings = nil }, true},
		{"bad protocol", func(c *MieruConfig) { c.PortBindings[0].Protocol = "QUIC" }, true},
		{"port too low", func(c *MieruConfig) { c.PortBindings[0].Port = 80 }, true},
		{"range ok", func(c *MieruConfig) {
			c.PortBindings = []MieruPortBinding{{PortRange: "20000-20010", Protocol: "UDP"}}
		}, false},
		{"range inverted", func(c *MieruConfig) {
			c.PortBindings = []MieruPortBinding{{PortRange: "20010-20000", Protocol: "UDP"}}
		}, true},
		{"mtu low", func(c *MieruConfig) { c.MTU = 1000 }, true},
		{"mtu high", func(c *MieruConfig) { c.MTU = 9000 }, true},
		{"bad log level", func(c *MieruConfig) { c.LoggingLevel = "TRACE" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultMieruConfig()
			tc.mut(&cfg)
			err := cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestMieruValidateInboundRequiresClient(t *testing.T) {
	cfg := DefaultMieruConfig()
	if err := cfg.ValidateInbound(false); err == nil {
		t.Fatal("ValidateInbound without clients must fail")
	}
	if err := cfg.ValidateInbound(true); err != nil {
		t.Fatalf("ValidateInbound with clients: %v", err)
	}
}

func TestMieruRenderJSON(t *testing.T) {
	cfg := DefaultMieruConfig()
	users := []AuthPair{{User: "miABC", Pass: "pass1"}}
	var got mitaServerConfig
	if err := json.Unmarshal([]byte(cfg.RenderJSON(users)), &got); err != nil {
		t.Fatalf("RenderJSON produced invalid JSON: %v", err)
	}
	if len(got.PortBindings) != 1 || got.PortBindings[0].Port != 20100 || got.PortBindings[0].Protocol != "TCP" {
		t.Fatalf("portBindings = %+v", got.PortBindings)
	}
	if len(got.Users) != 1 || got.Users[0].Name != "miABC" || got.Users[0].Password != "pass1" {
		t.Fatalf("users = %+v", got.Users)
	}
	if got.Egress != nil {
		t.Fatal("egress must be absent when not routed")
	}

	cfg.RouteThroughXray = true
	cfg.RouteXrayPort = 12345
	if err := json.Unmarshal([]byte(cfg.RenderJSON(users)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Egress == nil || len(got.Egress.Proxies) != 1 {
		t.Fatalf("routed config must carry egress proxy, got %+v", got.Egress)
	}
	p := got.Egress.Proxies[0]
	if p.Protocol != "SOCKS5_PROXY_PROTOCOL" || p.Host != "127.0.0.1" || p.Port != 12345 {
		t.Fatalf("egress proxy = %+v", p)
	}
	if len(got.Egress.Rules) != 1 || got.Egress.Rules[0].Action != "PROXY" {
		t.Fatalf("egress rules = %+v", got.Egress.Rules)
	}
}

func TestMieruClientLink(t *testing.T) {
	cfg := DefaultMieruConfig()
	cfg.PortBindings = []MieruPortBinding{
		{Port: 6666, Protocol: "TCP"},
		{PortRange: "9998-9999", Protocol: "UDP"},
	}
	link := cfg.ClientLink("1.2.3.4", AuthPair{User: "miuser", Pass: "mipass"}, "client@mail")
	if !strings.HasPrefix(link, "mierus://miuser:mipass@1.2.3.4?") {
		t.Fatalf("link prefix wrong: %s", link)
	}
	for _, want := range []string{"profile=default", "mtu=1400", "port=6666", "protocol=TCP", "port=9998-9999", "protocol=UDP"} {
		if !strings.Contains(link, want) {
			t.Fatalf("link missing %q: %s", want, link)
		}
	}
	if !strings.HasSuffix(link, "#client%40mail") && !strings.HasSuffix(link, "#client@mail") {
		t.Fatalf("link fragment wrong: %s", link)
	}
	if cfg.ClientLink("", AuthPair{User: "u", Pass: "p"}, "") != "" {
		t.Fatal("empty host must yield empty link")
	}
}

func TestMieruPortHelpers(t *testing.T) {
	if lo, hi, ok := MieruPortRangeBounds("20000-20010"); !ok || lo != 20000 || hi != 20010 {
		t.Fatalf("bounds = %d %d %v", lo, hi, ok)
	}
	if _, _, ok := MieruPortRangeBounds("garbage"); ok {
		t.Fatal("garbage range must fail")
	}
	cfg := MieruConfig{PortBindings: []MieruPortBinding{{PortRange: "30000-30005", Protocol: "TCP"}}}
	if MieruPrimaryPort(cfg) != 30000 {
		t.Fatalf("primary port = %d", MieruPrimaryPort(cfg))
	}
}

func TestMieruClientAuthScoped(t *testing.T) {
	secret := []byte("panel-secret")
	a := MieruClientAuth(secret, 7, "user@mail")
	b := MieruClientAuth(secret, 7, "user@mail")
	if a != b {
		t.Fatal("MieruClientAuth must be deterministic")
	}
	if !strings.HasPrefix(a.User, "mi") {
		t.Fatalf("mieru user prefix = %q", a.User)
	}
	// Different inbound / email / core must not collide.
	if MieruClientAuth(secret, 8, "user@mail") == a {
		t.Fatal("different inbound must derive different creds")
	}
	if MieruClientAuth(secret, 7, "other@mail") == a {
		t.Fatal("different email must derive different creds")
	}
	if TrustTunnelClientAuth(secret, 7, "user@mail").User == a.User {
		t.Fatal("trusttunnel creds must not collide with mieru")
	}
	if ClientAuthForInbound(secret, 7, "user@mail").User == a.User {
		t.Fatal("naive creds must not collide with mieru")
	}
}

func TestAbsPath(t *testing.T) {
	if got := absPath(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	already := filepath.FromSlash("/already/abs")
	if !filepath.IsAbs(already) {
		already, _ = filepath.Abs("already-abs-marker")
	}
	if got := absPath(already); got != already {
		t.Fatalf("abs passthrough: %q want %q", got, already)
	}
	got := absPath(filepath.Join("bin", "tunnel", "mieru-4-data", "mita.sock"))
	if !filepath.IsAbs(got) || !strings.Contains(got, "mita.sock") {
		t.Fatalf("relative resolved poorly: %q", got)
	}
}

func TestParseMieruUsersTable(t *testing.T) {
	out := `User  LastActive  1DayDown  1DayUp  7DaysDown  7DaysUp  30DaysDown  30DaysUp
miabc123456  2026-08-13T10:00:00Z  1.5MiB  200.0KiB  2.0MiB  300.0KiB  2.0MiB  300.0KiB
mideadbeef0  -  0B  0B  0B  0B  0B  0B`
	got := parseMieruUsersTable(out)
	if len(got) != 2 {
		t.Fatalf("parsed %d users, want 2", len(got))
	}
	a, ok := got["miabc123456"]
	if !ok {
		t.Fatal("user miabc123456 missing")
	}
	if a.down != int64(1.5*(1<<20)) {
		t.Fatalf("down = %d", a.down)
	}
	if a.up != int64(200*(1<<10)) {
		t.Fatalf("up = %d", a.up)
	}
	if a.lastActive.IsZero() {
		t.Fatal("lastActive must parse")
	}
	b := got["mideadbeef0"]
	if !b.lastActive.IsZero() || b.up != 0 || b.down != 0 {
		t.Fatalf("zero user parsed wrong: %+v", b)
	}
	spaced := parseMieruUsersTable("User LastActive 1DayDown 1DayUp\nmiold  2026-08-13T10:00:00Z  1.5 MiB  200.0 KiB")
	if spaced["miold"].down != int64(1.5*(1<<20)) || spaced["miold"].up != int64(200*(1<<10)) {
		t.Fatalf("spaced format: %+v", spaced["miold"])
	}
	if parseMieruByteCount("garbage", "MiB") != 0 {
		t.Fatal("garbage number must yield 0")
	}
	if parseMieruByteCount("2", "ZiB") != 0 {
		t.Fatal("unknown unit must yield 0")
	}
}

func mieruTestInbound(settings string, enable bool) *model.Inbound {
	return &model.Inbound{
		Id:       12,
		Enable:   enable,
		Protocol: model.Mieru,
		Remark:   "mieru test",
		Settings: settings,
	}
}

func TestMieruInstanceFromInbound(t *testing.T) {
	secret := []byte("panel-secret")
	settings := `{"portBindings":[{"port":20100,"protocol":"TCP"}],"mtu":1400,"clients":[{"email":"u@m","enable":true}]}`

	inst, ok := MieruInstanceFromInbound(mieruTestInbound(settings, true), secret)
	if !ok || !inst.Enabled {
		t.Fatalf("valid inbound must produce enabled instance: ok=%v enabled=%v", ok, inst.Enabled)
	}
	if inst.Key != "mieru-12" || inst.Core != Mieru {
		t.Fatalf("instance key/core = %s %s", inst.Key, inst.Core)
	}
	if !strings.Contains(inst.ConfigText, `"name"`) {
		t.Fatal("config must contain derived user")
	}
	if inst.ProbePort != 20100 {
		t.Fatalf("probe port = %d", inst.ProbePort)
	}

	// Disabled inbound → Enabled:false instance (converge down, no error).
	inst, ok = MieruInstanceFromInbound(mieruTestInbound(settings, false), secret)
	if !ok || inst.Enabled {
		t.Fatal("disabled inbound must yield Enabled:false")
	}

	// No clients → Enabled:false (mita without users is inert).
	inst, _ = MieruInstanceFromInbound(mieruTestInbound(`{"portBindings":[{"port":20100,"protocol":"TCP"}]}`, true), secret)
	if inst.Enabled {
		t.Fatal("clientless mieru inbound must be Enabled:false")
	}

	// Non-mieru inbound → ok=false.
	other := mieruTestInbound(settings, true)
	other.Protocol = model.VLESS
	if _, ok := MieruInstanceFromInbound(other, secret); ok {
		t.Fatal("non-mieru inbound must return ok=false")
	}
}
