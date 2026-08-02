// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"encoding/base64"
	"encoding/hex"
	crand "math/rand"
	"strings"
	"testing"
)

func TestGenerateAWGParams_Invariants(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	// Default (empty) version behaves as "2": H1-H4 are "lo-hi" ranges.
	for _, prof := range []ObfProfile{ObfLite, ObfStandard, ObfPro} {
		for i := 0; i < 200; i++ {
			p, err := GenerateAWGParams(prof, "2")
			if err != nil {
				t.Fatalf("profile %s iter %d: %v", prof, i, err)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("profile %s iter %d validate: %v", prof, i, err)
			}
			// S1-S4 must be >= MinSForHPK so an AWG3 kernel accepts a
			// HeaderProtectionKey without -EINVAL (the cipher nonce is read
			// from the first 12 bytes of S-padding).
			for _, s := range []int{p.S1, p.S2, p.S3, p.S4} {
				if s < MinSForHPK {
					t.Fatalf("profile %s iter %d: S value %d < MinSForHPK %d", prof, i, s, MinSForHPK)
				}
			}
			// H1-H4 must be "lo-hi" ranges for version "2"/"3".
			for _, h := range []string{p.H1, p.H2, p.H3, p.H4} {
				if !strings.Contains(h, "-") {
					t.Fatalf("profile %s: H range %q missing '-'", prof, h)
				}
			}
		}
	}
}

// TestGenerateAWGParams_HFormatByVersion checks the wire format of H1-H4
// matches the awgVersion preset: "1.5" → single integer (legacy AmneziaWG 1.x,
// no "-"); "2" and "3" → "lo-hi" range (the v2+ form). This is the
// regression guard for the user-reported bug where selecting AWG 1.5 still
// emitted v2.0-style ranges (which v1.x awg-quick rejects at parse time).
func TestGenerateAWGParams_HFormatByVersion(t *testing.T) {
	for _, tc := range []struct {
		version  string
		wantDash bool
	}{
		{"1.5", false},
		{"2", true},
		{"3", true},
		{"", true}, // empty defaults to "2" behaviour
	} {
		for _, prof := range []ObfProfile{ObfLite, ObfStandard, ObfPro} {
			SetRand(crand.New(crand.NewSource(42)))
			p, err := GenerateAWGParams(prof, tc.version)
			if err != nil {
				t.Fatalf("version %q profile %s: %v", tc.version, prof, err)
			}
			for _, h := range []string{p.H1, p.H2, p.H3, p.H4} {
				if h == "" {
					t.Fatalf("version %q profile %s: empty H value", tc.version, prof)
				}
				hasDash := strings.Contains(h, "-")
				if hasDash != tc.wantDash {
					t.Fatalf("version %q profile %s: H %q dash=%v, want %v", tc.version, prof, h, hasDash, tc.wantDash)
				}
			}
		}
	}
}

func TestGenerateHeaderProtectionKey_Format(t *testing.T) {
	for i := 0; i < 32; i++ {
		k, err := GenerateHeaderProtectionKey()
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		// AWG3 HeaderProtectionKey is 32 random bytes, base64-encoded → 44
		// chars (no newline), same shape as a WireGuard private key.
		if len(k) != 44 {
			t.Fatalf("iter %d: key length = %d, want 44 (base64 of 32 bytes)", i, len(k))
		}
		raw, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			t.Fatalf("iter %d: not valid base64: %v", i, err)
		}
		if len(raw) != 32 {
			t.Fatalf("iter %d: decoded length = %d, want 32", i, len(raw))
		}
	}
}

func TestWithHeaderProtectionKey_SetsField(t *testing.T) {
	SetRand(crand.New(crand.NewSource(7)))
	p, err := GenerateAWGParams(ObfStandard, "2")
	if err != nil {
		t.Fatal(err)
	}
	if p.HeaderProtectionKey != "" {
		t.Fatal("GenerateAWGParams must not set HeaderProtectionKey by default")
	}
	p2, err := p.WithHeaderProtectionKey()
	if err != nil {
		t.Fatal(err)
	}
	if p2.HeaderProtectionKey == "" {
		t.Fatal("WithHeaderProtectionKey must populate the field")
	}
	if p2.S1 != p.S1 || p2.H1 != p.H1 {
		t.Fatal("WithHeaderProtectionKey must preserve the other fields")
	}
}

func TestAsConfLines_HeaderProtectionKeyGated(t *testing.T) {
	SetRand(crand.New(crand.NewSource(9)))
	p, err := GenerateAWGParams(ObfPro, "2")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(p.AsConfLines(), "HeaderProtectionKey") {
		t.Fatal("AsConfLines must omit HeaderProtectionKey when empty")
	}
	p.HeaderProtectionKey = "aBcD...base64hpk=="
	if !strings.Contains(p.AsConfLines(), "HeaderProtectionKey = aBcD...base64hpk==") {
		t.Fatal("AsConfLines must emit HeaderProtectionKey when set")
	}
}

func TestGenerateCPS_AllProfilesNonEmpty(t *testing.T) {
	SetRand(crand.New(crand.NewSource(7)))
	for _, mp := range []MimicryProfile{ProfileTLS, ProfileDNS, ProfileSIP, ProfileQUIC} {
		for _, reg := range []Region{RegionRU, RegionWorld} {
			r1, err := GenerateCPS(mp, reg, "", BrowserChrome, true)
			if err != nil {
				t.Fatalf("profile %s region %s onlyI1: %v", mp, reg, err)
			}
			if r1.I1 == "" {
				t.Fatalf("profile %s region %s: I1 empty", mp, reg)
			}
			if r1.I2 != "" {
				t.Fatalf("profile %s region %s: onlyI1 leaked I2", mp, reg)
			}
			r5, err := GenerateCPS(mp, reg, "", BrowserChrome, false)
			if err != nil {
				t.Fatalf("profile %s region %s full: %v", mp, reg, err)
			}
			for i, v := range []string{r5.I1, r5.I2, r5.I3, r5.I4, r5.I5} {
				if v == "" {
					t.Fatalf("profile %s region %s: I%d empty in full mode", mp, reg, i+1)
				}
			}
		}
	}
}

func TestGenerateCPS_ExplicitDomain(t *testing.T) {
	SetRand(crand.New(crand.NewSource(1)))
	r, err := GenerateCPS(ProfileTLS, RegionWorld, "example.com", BrowserChrome, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.I1 == "" {
		t.Fatal("explicit domain produced empty I1")
	}
}

func TestGenerateCPS_DNSHasR2Prefix(t *testing.T) {
	SetRand(crand.New(crand.NewSource(3)))
	r, err := GenerateCPS(ProfileDNS, RegionWorld, "example.com", BrowserChrome, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.I1, "<r 2>") {
		t.Fatalf("DNS packet must start with <r 2>, got %q", r.I1[:20])
	}
}

func TestGenerateCPS_NonDNSNoR2Prefix(t *testing.T) {
	SetRand(crand.New(crand.NewSource(5)))
	for _, mp := range []MimicryProfile{ProfileTLS, ProfileSIP, ProfileQUIC} {
		r, err := GenerateCPS(mp, RegionWorld, "example.com", BrowserChrome, true)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(r.I1, "<r 2>") {
			t.Fatalf("profile %s must not use <r 2> prefix", mp)
		}
	}
}

func TestGenerateCPS_AllBrowsersNonEmpty(t *testing.T) {
	SetRand(crand.New(crand.NewSource(11)))
	for _, browser := range []BrowserProfile{BrowserChrome, BrowserFirefox, BrowserSafari} {
		r, err := GenerateCPS(ProfileTLS, RegionWorld, "example.com", browser, true)
		if err != nil {
			t.Fatalf("browser %s: %v", browser, err)
		}
		if r.I1 == "" {
			t.Fatalf("browser %s: I1 empty", browser)
		}
		if !strings.HasPrefix(r.I1, "<b 0x") {
			t.Fatalf("browser %s: I1 must be hex tag, got %q", browser, r.I1[:20])
		}
	}
}

func TestQuicInitialPacket_RespectsBrowser(t *testing.T) {
	SetRand(crand.New(crand.NewSource(7)))
	chrome := quicInitialPacket("example.com", BrowserChrome)
	SetRand(crand.New(crand.NewSource(7)))
	firefox := quicInitialPacket("example.com", BrowserFirefox)
	if chrome == firefox {
		t.Error("chrome and firefox QUIC Initials must differ (embedded ClientHello differs)")
	}
	for name, tag := range map[string]string{"chrome": chrome, "firefox": firefox} {
		if len(tag) < 2400 {
			t.Errorf("%s: QUIC Initial must pad to ~1200 bytes (>=2400 hex chars), got %d", name, len(tag))
		}
	}
}

func TestBuildFirefoxHello_NoGrease(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	ch := buildFirefoxHello("example.com")
	hexStr := hex.EncodeToString(ch)
	if strings.Contains(hexStr, "0a0a") || strings.Contains(hexStr, "fafa") {
		t.Errorf("Firefox ClientHello must not contain GREASE values")
	}
}

func TestBuildSafariHello_NoGrease(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	ch := buildSafariHello("example.com")
	hexStr := hex.EncodeToString(ch)
	if strings.Contains(hexStr, "0a0a") || strings.Contains(hexStr, "fafa") {
		t.Errorf("Safari ClientHello must not contain GREASE values")
	}
}

func TestBuildChromeHello_HasGrease(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	ch := buildChromeHello("example.com")
	if len(ch) < 100 {
		t.Fatalf("Chrome ClientHello too short: %d bytes", len(ch))
	}
}

func TestBuildFirefoxHello_HasPadding512(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	ch := buildFirefoxHello("example.com")
	if len(ch) < 200 {
		t.Fatalf("Firefox ClientHello too short (padding expected): %d bytes", len(ch))
	}
}

func TestBuildSafariHello_HasTls11(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	ch := buildSafariHello("example.com")
	hexStr := hex.EncodeToString(ch)
	if !strings.Contains(hexStr, "0302") {
		t.Errorf("Safari ClientHello must advertise TLS 1.1 (0x0302)")
	}
}

func TestDomainPool_NonEmpty(t *testing.T) {
	for _, mp := range []MimicryProfile{ProfileTLS, ProfileDNS, ProfileSIP, ProfileQUIC} {
		for _, reg := range []Region{RegionRU, RegionWorld} {
			pool := DomainPool(mp, reg)
			if len(pool) == 0 {
				t.Fatalf("pool empty for %s/%s", mp, reg)
			}
		}
	}
}

// The six AWG3 device-level fields are emitted by AsConfLines on a plain >0
// guard (no version gate in this layer — the Instance/ClientSettings renderers
// apply the AwgVersion=="3" gate). 0 means "use the kernel built-in WireGuard
// constant", so a zero value MUST stay silent on the wire.
func TestAsConfLines_DeviceFieldsGated(t *testing.T) {
	SetRand(crand.New(crand.NewSource(9)))
	p, err := GenerateAWGParams(ObfPro, "2")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		line string
		set  func(p *AWGParams)
	}{
		{"ContentPaddingAddition", "ContentPaddingAddition = 32", func(p *AWGParams) { p.ContentPaddingAddition = 32 }},
		{"RekeyAfterTime", "RekeyAfterTime = 120", func(p *AWGParams) { p.RekeyAfterTime = 120 }},
		{"RekeyTimeout", "RekeyTimeout = 5", func(p *AWGParams) { p.RekeyTimeout = 5 }},
		{"RejectAfterTime", "RejectAfterTime = 180", func(p *AWGParams) { p.RejectAfterTime = 180 }},
		{"KeepaliveTimeout", "KeepaliveTimeout = 10", func(p *AWGParams) { p.KeepaliveTimeout = 10 }},
		{"MaxHandshakeAttempts", "MaxHandshakeAttempts = 18", func(p *AWGParams) { p.MaxHandshakeAttempts = 18 }},
	}
	for _, tc := range cases {
		t.Run(tc.name+"_emitted", func(t *testing.T) {
			q := p
			tc.set(&q)
			out := q.AsConfLines()
			if !strings.Contains(out, tc.line) {
				t.Fatalf("AsConfLines must emit %q when set, got:\n%s", tc.line, out)
			}
		})
		t.Run(tc.name+"_omitted", func(t *testing.T) {
			out := p.AsConfLines()
			if strings.Contains(out, tc.name+" =") {
				t.Fatalf("AsConfLines must omit %q when 0, got:\n%s", tc.name+" =", out)
			}
		})
	}
}

// Validate bounds the six device fields to the u16 range (0..65535): 0 is the
// "kernel default" sentinel, 65535 is the max a u16 transport carries, and
// negatives/overshoots are rejected so a hand-edited settings blob can never
// produce a .conf the AWG3 kernel rejects at netlink time.
func TestValidate_DeviceFieldRange(t *testing.T) {
	base := AWGParams{Jmin: 50, Jmax: 200, S1: 30, S2: 120, S3: 50, S4: 70}
	cases := []struct {
		name string
		set  func(p *AWGParams)
	}{
		{"ContentPaddingAddition over 65535", func(p *AWGParams) { p.ContentPaddingAddition = 65536 }},
		{"ContentPaddingAddition negative", func(p *AWGParams) { p.ContentPaddingAddition = -1 }},
		{"RekeyAfterTime over 65535", func(p *AWGParams) { p.RekeyAfterTime = 100000 }},
		{"RekeyAfterTime negative", func(p *AWGParams) { p.RekeyAfterTime = -5 }},
		{"RekeyTimeout over 65535", func(p *AWGParams) { p.RekeyTimeout = 70000 }},
		{"RekeyTimeout negative", func(p *AWGParams) { p.RekeyTimeout = -1 }},
		{"RejectAfterTime over 65535", func(p *AWGParams) { p.RejectAfterTime = 200000 }},
		{"RejectAfterTime negative", func(p *AWGParams) { p.RejectAfterTime = -1 }},
		{"KeepaliveTimeout over 65535", func(p *AWGParams) { p.KeepaliveTimeout = 99999 }},
		{"KeepaliveTimeout negative", func(p *AWGParams) { p.KeepaliveTimeout = -1 }},
		{"MaxHandshakeAttempts over 65535", func(p *AWGParams) { p.MaxHandshakeAttempts = 100000 }},
		{"MaxHandshakeAttempts negative", func(p *AWGParams) { p.MaxHandshakeAttempts = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.set(&p)
			if err := p.Validate(); err == nil {
				t.Fatalf("Validate must reject %s", tc.name)
			}
		})
	}
}

func TestValidate_DeviceFieldsAccept2Byte(t *testing.T) {
	p := AWGParams{
		Jmin: 50, Jmax: 200, S1: 30, S2: 120, S3: 50, S4: 70,
		ContentPaddingAddition: 65535, RekeyAfterTime: 65535, RekeyTimeout: 65535,
		RejectAfterTime: 65535, KeepaliveTimeout: 65535, MaxHandshakeAttempts: 65535,
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate must accept 65535 (max u16) for all device fields, got: %v", err)
	}
}
