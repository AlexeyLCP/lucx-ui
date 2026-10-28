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

func TestMasterdnsValidate(t *testing.T) {
	good := MasterdnsConfig{Domains: "v.Example.com, d2.example.com", Secret: "0123456789abcdef0123456789abcdef"}
	if err := good.Validate(18445); err != nil {
		t.Fatalf("good config rejected: %v", err)
	}
	bad := []struct {
		name string
		cfg  MasterdnsConfig
		port int
	}{
		{"no domains", MasterdnsConfig{Domains: " ", Secret: "0123456789abcdef0123456789abcdef"}, 53},
		{"bad domain", MasterdnsConfig{Domains: "v.exa_mple.com", Secret: "0123456789abcdef0123456789abcdef"}, 53},
		{"label leading hyphen", MasterdnsConfig{Domains: "-v.example.com", Secret: "0123456789abcdef0123456789abcdef"}, 53},
		{"short secret", MasterdnsConfig{Domains: "v.example.com", Secret: "abc", Encryption: 2}, 53},
		{"bad port", MasterdnsConfig{Domains: "v.example.com", Secret: "0123456789abcdef0123456789abcdef"}, 0},
	}
	for _, c := range bad {
		if err := c.cfg.Validate(c.port); err == nil {
			t.Errorf("%s must be rejected", c.name)
		}
	}
}

func TestMasterdnsRenderServerConf(t *testing.T) {
	routed := MasterdnsConfig{
		Domains:          "V.Domain.com, x.domain.com",
		Secret:           "0123456789abcdef0123456789abcdef",
		Encryption:       2,
		RouteThroughXray: true,
		RouteXrayPort:    51000,
	}
	conf := routed.RenderServerConf(MasterdnsKey, 5300)
	for _, want := range []string{
		`DOMAIN = ["v.domain.com", "x.domain.com"]`,
		`PROTOCOL_TYPE = "SOCKS5"`,
		`DATA_ENCRYPTION_METHOD = 2`,
		`ENCRYPTION_KEY_FILE = `,
		`UDP_PORT = 5300`,
		`USE_EXTERNAL_SOCKS5 = true`,
		`FORWARD_IP = "127.0.0.1"`,
		`FORWARD_PORT = 51000`,
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("routed conf missing %q:\n%s", want, conf)
		}
	}
	direct := MasterdnsConfig{Domains: "v.example.com", Encryption: 1}
	directConf := direct.RenderServerConf(MasterdnsKey, 53)
	if !strings.Contains(directConf, "USE_EXTERNAL_SOCKS5 = false") || strings.Contains(directConf, "FORWARD_PORT") {
		t.Fatalf("direct conf wrong:\n%s", directConf)
	}
}

func TestMasterdnsClientTOMLAgreesWithServer(t *testing.T) {
	cfg := MasterdnsConfig{Domains: "v.example.com", Secret: "0123456789abcdef0123456789abcdef", Encryption: 3}
	toml := cfg.RenderClientTOML()
	if !strings.Contains(toml, `DOMAINS = ["v.example.com"]`) ||
		!strings.Contains(toml, `DATA_ENCRYPTION_METHOD = 3`) ||
		!strings.Contains(toml, `ENCRYPTION_KEY = "0123456789abcdef0123456789abcdef"`) {
		t.Fatalf("client TOML wrong:\n%s", toml)
	}
	if !strings.Contains(cfg.RenderClientResolvers(), "8.8.8.8:53") {
		t.Fatalf("default resolvers missing")
	}
	cfg.Resolvers = "9.9.9.9:53, 1.0.0.1:53"
	if !strings.Contains(cfg.RenderClientResolvers(), "9.9.9.9:53\n1.0.0.1:53") {
		t.Fatalf("operator resolvers not rendered: %q", cfg.RenderClientResolvers())
	}
}

func TestMasterdnsInstanceFromInbound(t *testing.T) {
	// Redirect the work dir: the instance call renders the TOML + secret to
	// disk and must never touch the real bin folder from a unit test.
	dir := t.TempDir()
	prev := tunnelDir
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })
	enable := true
	ib := &model.Inbound{
		Protocol: model.Masterdns,
		Enable:   enable,
		Port:     5300,
		Settings: `{"domains":"v.example.com","secret":"0123456789abcdef0123456789abcdef","encryption":2}`,
	}
	inst, ok := MasterdnsInstanceFromInbound(ib)
	if !ok || !inst.Enabled || inst.Core != Masterdns || inst.Key != MasterdnsKey {
		t.Fatalf("instance %+v ok=%v", inst, ok)
	}
	if len(inst.Args) != 2 || inst.Args[0] != "-config" {
		t.Fatalf("args %v", inst.Args)
	}
	off := &model.Inbound{Protocol: model.Masterdns, Enable: false, Port: 5300, Settings: "{}"}
	instOff, ok := MasterdnsInstanceFromInbound(off)
	if !ok || instOff.Enabled {
		t.Fatalf("disabled instance %+v", instOff)
	}
	other := &model.Inbound{Protocol: model.Csqtt, Enable: true, Port: 46000, Settings: "{}"}
	if _, ok := MasterdnsInstanceFromInbound(other); ok {
		t.Fatal("non-masterdns inbound must not match")
	}
}
