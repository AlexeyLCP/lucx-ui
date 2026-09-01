// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"
)

func TestNameRegistry(t *testing.T) {
	if !Naive.Valid() {
		t.Error("Naive must be valid")
	}
	if Name("nope").Valid() {
		t.Error("unknown name must be invalid")
	}
	if Naive.DisplayName() != "NaiveProxy" {
		t.Errorf("DisplayName = %q", Naive.DisplayName())
	}
	if got := Naive.BinaryName(); !strings.HasPrefix(got, "caddy-naive-") {
		t.Errorf("BinaryName = %q", got)
	}
	if !Olcrtc.Valid() || Olcrtc.DisplayName() != "olcRTC" {
		t.Errorf("Olcrtc Valid/DisplayName broken: %v %q", Olcrtc.Valid(), Olcrtc.DisplayName())
	}
	if got := All(); len(got) != 6 || got[0] != Naive || got[1] != Olcrtc || got[2] != Qwdtt || got[3] != Mieru || got[4] != TrustTunnel || got[5] != Anytls {
		t.Errorf("All() = %v", got)
	}
	if got := Olcrtc.BinaryName(); !strings.HasPrefix(got, "olcrtc-") {
		t.Errorf("Olcrtc.BinaryName = %q", got)
	}
	if !Mieru.Valid() || Mieru.DisplayName() != "mieru" {
		t.Errorf("Mieru Valid/DisplayName broken: %v %q", Mieru.Valid(), Mieru.DisplayName())
	}
	if got := Mieru.BinaryName(); !strings.HasPrefix(got, "mieru-") {
		t.Errorf("Mieru.BinaryName = %q", got)
	}
	if !TrustTunnel.Valid() || TrustTunnel.DisplayName() != "TrustTunnel" {
		t.Errorf("TrustTunnel Valid/DisplayName broken: %v %q", TrustTunnel.Valid(), TrustTunnel.DisplayName())
	}
	if got := TrustTunnel.BinaryName(); !strings.HasPrefix(got, "trusttunnel-") {
		t.Errorf("TrustTunnel.BinaryName = %q", got)
	}
	if !Anytls.Valid() || Anytls.DisplayName() != "AnyTLS" {
		t.Errorf("Anytls Valid/DisplayName broken: %v %q", Anytls.Valid(), Anytls.DisplayName())
	}
	if got := Anytls.BinaryName(); !strings.HasPrefix(got, "anytls-") {
		t.Errorf("Anytls.BinaryName = %q", got)
	}
}

func TestDefaultNaiveConfig(t *testing.T) {
	cfg := DefaultNaiveConfig()
	if cfg.Port != 443 {
		t.Errorf("Port = %d, want 443", cfg.Port)
	}
	if !cfg.EnableH3 || !cfg.ProbeResistance {
		t.Error("H3/probe_resistance must default on")
	}
	if cfg.LogLevel != "WARN" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
}

func TestNaiveMergeFillsZeroFields(t *testing.T) {
	got := NaiveConfig{}.Merge()
	if got.Port != 443 || got.LogLevel != "WARN" {
		t.Errorf("Merge left zero defaults: %+v", got)
	}
	custom := NaiveConfig{Port: 8443, LogLevel: "ERROR"}
	got = custom.Merge()
	if got.Port != 8443 || got.LogLevel != "ERROR" {
		t.Errorf("Merge clobbered explicit values: %+v", got)
	}
}

func TestNaiveValidate(t *testing.T) {
	base := func() NaiveConfig {
		c := DefaultNaiveConfig()
		c.AuthUser = "alice"
		c.AuthPass = "s3cret"
		c.CertFile = "/etc/ssl/cert.pem"
		c.KeyFile = "/etc/ssl/key.pem"
		return c
	}

	if err := base().Validate(); err != nil {
		t.Fatalf("valid manual config rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*NaiveConfig)
	}{
		{"no user", func(c *NaiveConfig) { c.AuthUser = " " }},
		{"no pass", func(c *NaiveConfig) { c.AuthPass = "" }},
		{"bad port", func(c *NaiveConfig) { c.Port = 70000 }},
		{"zero port", func(c *NaiveConfig) { c.Port = 0 }},
		{"manual without cert", func(c *NaiveConfig) { c.CertFile = "" }},
		{"manual without key", func(c *NaiveConfig) { c.KeyFile = "" }},
		{"bad log level", func(c *NaiveConfig) { c.LogLevel = "LOUD" }},
		{"acme without domain", func(c *NaiveConfig) { c.UseAcme = true; c.Domain = "" }},
		{"acme on custom port", func(c *NaiveConfig) { c.UseAcme = true; c.Domain = "n.example.org"; c.Port = 8443 }},
		{"domain brace inject", func(c *NaiveConfig) { c.Domain = "x.com {\n foo" }},
		{"domain comma inject", func(c *NaiveConfig) { c.Domain = "x.com, :80" }},
		{"listen not ip", func(c *NaiveConfig) { c.Listen = "not-an-ip" }},
		{"email newline", func(c *NaiveConfig) { c.AcmeEmail = "a@b.com\nfoo" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	acme := base()
	acme.UseAcme = true
	acme.Domain = "n.example.org"
	acme.CertFile = ""
	acme.KeyFile = ""
	if err := acme.Validate(); err != nil {
		t.Fatalf("valid ACME config rejected: %v", err)
	}

	raw := NaiveConfig{UseRawConfig: true, RawConfig: ":443 {\n}\n"}
	if err := raw.Validate(); err != nil {
		t.Fatalf("raw config with text rejected: %v", err)
	}
	raw.RawConfig = "   "
	if err := raw.Validate(); err == nil {
		t.Fatal("empty raw config must be rejected")
	}
	rawRouted := NaiveConfig{UseRawConfig: true, RawConfig: ":443 {\n}\n", RouteThroughXray: true}
	if err := rawRouted.Validate(); err == nil {
		t.Fatal("raw mode + routeThroughXray must be rejected")
	}
}

func TestRenderCaddyfileUpstreamWhenRouted(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.AuthUser = "alice"
	cfg.AuthPass = "s3cret"
	cfg.CertFile = "/c.pem"
	cfg.KeyFile = "/k.pem"
	cfg.RouteThroughXray = true
	cfg.RouteXrayPort = 50123

	got := cfg.RenderCaddyfile(nil, "")
	if !strings.Contains(got, "upstream socks5://") || !strings.Contains(got, "@127.0.0.1:50123") {
		t.Fatalf("routed Caddyfile missing authenticated socks upstream:\n%s", got)
	}

	cfg.RouteThroughXray = false
	got = cfg.RenderCaddyfile(nil, "")
	if strings.Contains(got, "upstream socks5://") {
		t.Fatalf("unrouted Caddyfile must not render upstream:\n%s", got)
	}

	cfg.RouteThroughXray = true
	cfg.RouteXrayPort = 0
	got = cfg.RenderCaddyfile(nil, "")
	if strings.Contains(got, "upstream socks5://") {
		t.Fatalf("routed without port must not render upstream:\n%s", got)
	}
}

func TestNaiveEgressTagStable(t *testing.T) {
	if NaiveEgressTag != "lucx-tunnel-naive" {
		t.Fatalf("NaiveEgressTag = %q, operators match this in routing rules", NaiveEgressTag)
	}
}

func TestRenderCaddyfileManual(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.AuthUser = "alice"
	cfg.AuthPass = "s3cret"
	cfg.CertFile = "/etc/ssl/cert.pem"
	cfg.KeyFile = "/etc/ssl/key.pem"
	cfg.Domain = "n.example.org"

	got := cfg.RenderCaddyfile(nil, "")
	for _, want := range []string{
		"admin off",
		"skip_install_trust",
		"auto_https off",
		"level WARN",
		`:443, "n.example.org" {`,
		"tls \"/etc/ssl/cert.pem\" \"/etc/ssl/key.pem\"",
		"forward_proxy {",
		"basic_auth \"alice\" \"s3cret\"",
		"hide_ip",
		"hide_via",
		"probe_resistance",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Caddyfile missing %q:\n%s", want, got)
		}
	}
	// The naive forward_proxy fork has no `padding` subdirective — padding is
	// engaged per connection via the client's Padding header. Rendering one
	// makes `caddy adapt` reject the config (caught by E2E, lucx.91).
	if strings.Contains(got, "padding") {
		t.Errorf("Caddyfile must not render a padding subdirective:\n%s", got)
	}
	// A wildcard listen must NOT render as an explicit 0.0.0.0 site address or
	// a bind directive (Caddy would treat it as a host matcher).
	if strings.Contains(got, "0.0.0.0") || strings.Contains(got, "bind") {
		t.Errorf("wildcard listen leaked into site block:\n%s", got)
	}
	// H3 enabled by default: no protocols restriction.
	if strings.Contains(got, "protocols h1 h2") {
		t.Errorf("H3 enabled but protocols restricted:\n%s", got)
	}
}

func TestRenderCaddyfileBindAndH3Off(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.AuthUser = "u"
	cfg.AuthPass = "p"
	cfg.CertFile = "/c.pem"
	cfg.KeyFile = "/k.pem"
	cfg.Domain = "n.example.org"
	cfg.Listen = "10.0.0.5"
	cfg.Port = 8443
	cfg.EnableH3 = false
	cfg.ProbeResistance = false

	got := cfg.RenderCaddyfile(nil, "")
	for _, want := range []string{
		`:8443, "n.example.org:8443" {`,
		`bind "10.0.0.5"`,
		"protocols h1 h2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Caddyfile missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "probe_resistance") {
		t.Errorf("disabled options rendered:\n%s", got)
	}
	// A bare domain address would default to :443 and open a second listener.
	if strings.Contains(got, ", n.example.org {") {
		t.Errorf("domain address must carry the custom port:\n%s", got)
	}
}

func TestRenderCaddyfileAcme(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.UseAcme = true
	cfg.Domain = "n.example.org"
	cfg.AcmeEmail = "admin@example.org"
	cfg.AuthUser = "u"
	cfg.AuthPass = "p"

	got := cfg.RenderCaddyfile(nil, "")
	if !strings.Contains(got, `"n.example.org" {`) {
		t.Errorf("ACME site address must be the domain:\n%s", got)
	}
	if !strings.Contains(got, `tls "admin@example.org"`) {
		t.Errorf("ACME email missing:\n%s", got)
	}
	if strings.Contains(got, ":443,") {
		t.Errorf("ACME mode must not add a bare :443 address:\n%s", got)
	}
	if strings.Contains(got, "auto_https off") {
		t.Errorf("ACME mode must keep automatic HTTPS for HTTP-01:\n%s", got)
	}

	cfg.AcmeEmail = ""
	got = cfg.RenderCaddyfile(nil, "")
	if strings.Contains(got, "tls") {
		t.Errorf("ACME without email must rely on automatic HTTPS:\n%s", got)
	}
}

func TestRenderCaddyfileEscapesCredentials(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.AuthUser = `al"ice`
	cfg.AuthPass = "pass word\n\\"
	cfg.CertFile = "/c.pem"
	cfg.KeyFile = "/k.pem"

	got := cfg.RenderCaddyfile(nil, "")
	if !strings.Contains(got, `basic_auth "al\"ice" "pass word\n\\"`) {
		t.Errorf("credentials not escaped:\n%s", got)
	}
}

func TestRenderCaddyfileRawMode(t *testing.T) {
	cfg := NaiveConfig{UseRawConfig: true, RawConfig: ":8443 {\n\trespond \"ok\"\n}"}
	got := cfg.RenderCaddyfile(nil, "")
	if !strings.HasPrefix(got, "{\n\tadmin off\n\tskip_install_trust\n}\n\n") {
		t.Errorf("raw mode must prepend admin off:\n%q", got)
	}
	if !strings.Contains(got, ":8443 {\n\trespond \"ok\"\n}") {
		t.Errorf("raw site block missing:\n%q", got)
	}
	hostile := NaiveConfig{UseRawConfig: true, RawConfig: "{\n\tadmin 0.0.0.0:2019\n}\n:443 {\n}\n"}
	got = hostile.RenderCaddyfile(nil, "")
	if strings.Contains(got, "0.0.0.0:2019") {
		t.Errorf("raw admin listener survived:\n%s", got)
	}
	if !strings.Contains(got, "admin off") || !strings.Contains(got, "skip_install_trust") {
		t.Errorf("forced global missing:\n%s", got)
	}
}

func TestClientAuth(t *testing.T) {
	secret := []byte("panel-secret")
	a := ClientAuth(secret, "alice@example.com")
	b := ClientAuth(secret, "alice@example.com")
	if a != b {
		t.Error("ClientAuth must be deterministic for the same secret+email")
	}
	if !strings.HasPrefix(a.User, "nx") || len(a.User) != 12 {
		t.Errorf("unexpected username shape: %q", a.User)
	}
	if len(a.Pass) != 27 {
		t.Errorf("password length = %d, want 27", len(a.Pass))
	}
	if strings.Contains(a.User, "alice") || strings.Contains(a.Pass, "alice") {
		t.Error("credentials must not leak the email")
	}
	other := ClientAuth(secret, "bob@example.com")
	if other == a {
		t.Error("different emails must derive different credentials")
	}
	rotated := ClientAuth([]byte("rotated-secret"), "alice@example.com")
	if rotated == a {
		t.Error("rotating the panel secret must rotate credentials")
	}
}

func TestRenderCaddyfileExtraAuth(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.AuthUser = "svc"
	cfg.AuthPass = "svcpass"
	cfg.CertFile = "/c.pem"
	cfg.KeyFile = "/k.pem"
	extra := []AuthPair{
		{User: "nx0000000001", Pass: "client-one-pass"},
		{User: "nx0000000002", Pass: "client two pass"},
	}
	got := cfg.RenderCaddyfile(extra, "")
	for _, want := range []string{
		"basic_auth \"svc\" \"svcpass\"",
		"basic_auth \"nx0000000001\" \"client-one-pass\"",
		"basic_auth \"nx0000000002\" \"client two pass\"",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Caddyfile missing %q:\n%s", want, got)
		}
	}
	// Service pair first, then client pairs, all before hide_ip.
	if strings.Index(got, "basic_auth \"svc\"") > strings.Index(got, "basic_auth \"nx0000000001\"") {
		t.Errorf("service credentials must precede client pairs:\n%s", got)
	}
	if strings.Index(got, "basic_auth \"nx0000000002\"") > strings.Index(got, "hide_ip") {
		t.Errorf("all basic_auth lines must stay inside forward_proxy:\n%s", got)
	}
}

func TestNaiveClientURL(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.Domain = "n.example.org"
	cfg.AuthUser = "alice"
	cfg.AuthPass = "s3cret"
	if got, want := cfg.ClientURL(), "naive+https://alice:s3cret@n.example.org:443"; got != want {
		t.Errorf("ClientURL = %q, want %q", got, want)
	}

	cfg.Port = 8443
	if got, want := cfg.ClientURL(), "naive+https://alice:s3cret@n.example.org:8443"; got != want {
		t.Errorf("ClientURL = %q, want %q", got, want)
	}

	cfg.AuthPass = "p@ss:w ord"
	got := cfg.ClientURL()
	if !strings.Contains(got, "p%40ss%3Aw%20ord") {
		t.Errorf("ClientURL must escape the password: %q", got)
	}

	cfg.Domain = ""
	if got := cfg.ClientURL(); got != "" {
		t.Errorf("ClientURL without domain = %q, want empty", got)
	}
}

func TestNaiveClientURLForRemark(t *testing.T) {
	cfg := DefaultNaiveConfig()
	cfg.Domain = "n.example.org"
	cfg.Port = 443
	got := cfg.ClientURLFor(AuthPair{User: "alice", Pass: "s3cret"}, "naive-in-user")
	if !strings.HasSuffix(got, "#naive-in-user") {
		t.Errorf("ClientURLFor fragment: %q", got)
	}
	if !strings.Contains(got, "@n.example.org:443") {
		t.Errorf("ClientURLFor host:port: %q", got)
	}
}

func TestInstanceFingerprint(t *testing.T) {
	a := Instance{Core: Naive, Enabled: true, ConfigText: "x", ExtraArgs: "--a"}
	b := Instance{Core: Naive, Enabled: true, ConfigText: "x", ExtraArgs: "--a"}
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("identical instances must share a fingerprint")
	}
	c := a
	c.ConfigText = "y"
	if a.Fingerprint() == c.Fingerprint() {
		t.Error("config change must move the fingerprint")
	}
	d := a
	d.ExtraArgs = "--b"
	if a.Fingerprint() == d.Fingerprint() {
		t.Error("extra args change must move the fingerprint")
	}
	// Enabled is runtime state, not config shape.
	e := a
	e.Enabled = false
	if a.Fingerprint() != e.Fingerprint() {
		t.Error("enabled flag must not move the fingerprint")
	}
}

// The forced global block must disarm the admin endpoint without touching
// anything else that happens to contain the word. Each case below is an
// ordinary Caddyfile the regex used to mangle into an unparseable one.
func TestForceCaddySafeGlobal_KeepsUnrelatedAdminWords(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		keep string
	}{
		{"site address", "admin.example.com {\n\troot * /srv\n}\n", "admin.example.com {"},
		{"credential", ":443 {\n\tbasic_auth admin S3cretPassw0rd\n}\n", "basic_auth admin S3cretPassw0rd"},
		{"path", ":443 {\n\ttls /etc/ssl/admin.crt /etc/ssl/admin.key\n}\n", "/etc/ssl/admin.key"},
		{"header value", ":443 {\n\theader X-Role admin\n}\n", "header X-Role admin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := forceCaddySafeGlobal(tc.raw)
			if !strings.Contains(got, tc.keep) {
				t.Errorf("hardening ate a line it does not own; want %q in:\n%s", tc.keep, got)
			}
			if !strings.Contains(got, "admin off") {
				t.Errorf("forced global missing:\n%s", got)
			}
		})
	}
}

// Caddy refuses a global options block that is not the first thing in the file,
// and a leading comment is the ordinary way to head a config.
func TestForceCaddySafeGlobal_LeadingCommentKeepsOneGlobalBlock(t *testing.T) {
	raw := "# my config\n{\n\tdebug\n}\n\n:8443 {\n\trespond \"ok\"\n}\n"
	got := forceCaddySafeGlobal(raw)
	if n := strings.Count(got, "skip_install_trust"); n != 1 {
		t.Fatalf("expected exactly one forced global block, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, "debug") {
		t.Errorf("the operator's own global options were dropped:\n%s", got)
	}
	if strings.Contains(got, "}\n\n{\n") || strings.HasPrefix(got, "{\n\tadmin off") {
		t.Errorf("a second global block was prepended ahead of the existing one:\n%s", got)
	}
}

// The real admin directive still has to go, in every position it can occupy.
func TestForceCaddySafeGlobal_StillDisarmsTheAdminEndpoint(t *testing.T) {
	for _, raw := range []string{
		"{\n\tadmin 0.0.0.0:2019\n}\n:443 {\n}\n",
		"{\n\tadmin unix//run/caddy.sock\n}\n:443 {\n}\n",
		"# lead\n{\n\tadmin :2019\n}\n:443 {\n}\n",
	} {
		got := forceCaddySafeGlobal(raw)
		if strings.Contains(got, "2019") || strings.Contains(got, "caddy.sock") {
			t.Errorf("raw admin listener survived:\n%s", got)
		}
		if !strings.Contains(got, "admin off") {
			t.Errorf("forced global missing:\n%s", got)
		}
	}
}
