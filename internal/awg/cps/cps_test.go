// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	crand "math/rand"
	"strings"
	"testing"
)

func TestGenerateAWGParams_Invariants(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	// Version "2" (no header protection key): H1-H4 are "lo-hi" ranges.
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
			// H1-H4 must be "lo-hi" ranges for version "2".
			for _, h := range []string{p.H1, p.H2, p.H3, p.H4} {
				if !strings.Contains(h, "-") {
					t.Fatalf("profile %s: H range %q missing '-'", prof, h)
				}
			}
		}
	}
}

// TestGenerateAWGParams_PacketSizesDistinct guards the AmneziaVPN
// isPacketSizeEqual invariant: the four total packet sizes (148+S1, 92+S2,
// 64+S3, 32+S4) must stay pairwise distinct across many generations.
func TestGenerateAWGParams_PacketSizesDistinct(t *testing.T) {
	SetRand(crand.New(crand.NewSource(1337)))
	for _, prof := range []ObfProfile{ObfLite, ObfStandard, ObfPro} {
		for i := 0; i < 500; i++ {
			p, err := GenerateAWGParams(prof, "2")
			if err != nil {
				t.Fatalf("profile %s iter %d: %v", prof, i, err)
			}
			sizes := []int{
				messageInitiationSize + p.S1,
				messageResponseSize + p.S2,
				messageCookieReplySize + p.S3,
				messageTransportSize + p.S4,
			}
			for a := 0; a < len(sizes); a++ {
				for b := a + 1; b < len(sizes); b++ {
					if sizes[a] == sizes[b] {
						t.Fatalf("profile %s iter %d: equal packet sizes %d (S1=%d S2=%d S3=%d S4=%d)",
							prof, i, sizes[a], p.S1, p.S2, p.S3, p.S4)
					}
				}
			}
		}
	}
}

// TestGenerateAWGParams_HNarrowBands guards against regressing back to the
// 2^29-wide H quadrants: every generated H ("1.5" singles and "2" ranges)
// must fit inside the modest disjoint bands, so the magic header never turns
// into a giant number an operator flags as over-obfuscation.
func TestGenerateAWGParams_HNarrowBands(t *testing.T) {
	SetRand(crand.New(crand.NewSource(7)))
	for _, version := range []string{"1.5", "2", "3", "3.1"} {
		for i := 0; i < 200; i++ {
			p, err := GenerateAWGParams(ObfPro, version)
			if err != nil {
				t.Fatalf("version %q iter %d: %v", version, i, err)
			}
			for n, h := range []string{p.H1, p.H2, p.H3, p.H4} {
				lo, hi := hBand(n)
				_ = hi
				var first int
				if strings.Contains(h, "-") {
					if _, err := fmt.Sscanf(strings.SplitN(h, "-", 2)[0], "%d", &first); err != nil {
						t.Fatalf("version %q iter %d: unparsable H %q", version, i, h)
					}
				} else {
					if _, err := fmt.Sscanf(h, "%d", &first); err != nil {
						t.Fatalf("version %q iter %d: unparsable H %q", version, i, h)
					}
				}
				if first < lo || first > lo+hBandWidth {
					t.Fatalf("version %q iter %d: H%d=%q outside narrow band [%d,%d]", version, i, n+1, h, lo, lo+hBandWidth)
				}
			}
		}
	}
}

// TestGenerateAWGParams_HFormatByVersion checks the wire format of H1-H4
// matches the awgVersion preset: "1.5" → single integer (legacy AmneziaWG 1.x,
// no "-"); "2"/"3"/"3.1" → "lo-hi" range. lucx.136 emitted "1"/"2"/"3"/"4"
// for v3 (HPK encrypts the header); testers flagged that as a broken
// generator, so v3 uses the same ranges as v2. This is also the regression
// guard for the user-reported bug where selecting AWG 1.5 still emitted
// v2.0-style ranges (which v1.x awg-quick rejects at parse time).
func TestGenerateAWGParams_HFormatByVersion(t *testing.T) {
	for _, tc := range []struct {
		version  string
		wantDash bool
	}{
		{"1.5", false},
		{"2", true},
		{"3", true},
		{"3.1", true},
		{"", true},
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
			if tc.version == "3" || tc.version == "3.1" {
				got := strings.Join([]string{p.H1, p.H2, p.H3, p.H4}, ",")
				if got == "1,2,3,4" {
					t.Fatalf("version %q: H1-H4 must not be the WireGuard defaults", tc.version)
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
			if r5.I1 == "" {
				t.Fatalf("profile %s region %s: I1 empty in full mode", mp, reg)
			}
			if sum := r5.PayloadSum(); sum > MaxIPayload {
				t.Fatalf("profile %s region %s: payload %d > %d", mp, reg, sum, MaxIPayload)
			}
		}
	}
}

func TestCPSPayloadBytes(t *testing.T) {
	if got := tagPayloadBytes("<b 0xaabbcc>"); got != 3 {
		t.Fatalf("hex tag = %d, want 3", got)
	}
	if got := tagPayloadBytes("<r 2><b 0x0011>"); got != 2 {
		t.Fatalf("dns tag = %d, want 2", got)
	}
	if got := tagPayloadBytes(""); got != 0 {
		t.Fatalf("empty = %d", got)
	}
}

func TestShrinkCPS_DropsTrailingUntilUnderLimit(t *testing.T) {
	big := strings.Repeat("aa", MaxIPayload+1)
	r := shrinkCPS(CPSResult{
		I1: "<b 0x" + big + ">",
		I2: "<b 0xaabb>",
		I3: "<b 0xccdd>",
		I4: "<b 0xeeff>",
		I5: "<b 0x1122>",
	})
	if r.I2 != "" || r.I3 != "" || r.I4 != "" || r.I5 != "" {
		t.Fatalf("must drop I2-I5 when I1 is already over the cap, got %+v", r)
	}
}

func TestGenerateCPS_FullStaysUnderIPayloadCap(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		SetRand(crand.New(crand.NewSource(seed)))
		for _, mp := range []MimicryProfile{ProfileTLS, ProfileDNS, ProfileSIP, ProfileQUIC} {
			r, err := GenerateCPS(mp, RegionWorld, "example.com", BrowserChrome, false)
			if err != nil {
				t.Fatalf("seed %d profile %s: %v", seed, mp, err)
			}
			if sum := r.PayloadSum(); sum > MaxIPayload {
				t.Fatalf("seed %d profile %s payload %d > %d", seed, mp, sum, MaxIPayload)
			}
			if r.I1 == "" {
				t.Fatalf("seed %d profile %s dropped I1", seed, mp)
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

// TestQuicInitialPacket_NoZeroPaddingRun guards the regression where the QUIC
// Initial padded its ~1200-byte minimum with open 0x00 bytes, producing a hex
// string with ~1700 consecutive zeros — a fingerprint no real client (whose
// payload is AEAD-encrypted) ever shows. The padding must be high-entropy: the
// longest "00" run in the hex should stay tiny relative to the packet. Chrome
// and Safari are tested — Firefox's embedded ClientHello pads to a 512-byte
// boundary with a legitimate (for the open TLS ClientHello) zero-filled
// padding extension, so it is excluded here.
func TestQuicInitialPacket_NoZeroPaddingRun(t *testing.T) {
	SetRand(crand.New(crand.NewSource(7)))
	for _, browser := range []BrowserProfile{BrowserChrome, BrowserSafari} {
		tag := quicInitialPacket("example.com", browser)
		raw, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSuffix(tag, ">"), "<b 0x"))
		if err != nil {
			t.Fatalf("%s: not valid hex: %v", browser, err)
		}
		maxRun, curRun := 0, 0
		for _, b := range raw {
			if b == 0x00 {
				curRun++
				if curRun > maxRun {
					maxRun = curRun
				}
			} else {
				curRun = 0
			}
		}
		if maxRun > 128 {
			t.Fatalf("%s: QUIC Initial has a %d-byte zero run — padding must be high-entropy", browser, maxRun)
		}
	}
}

func TestBuildFirefoxHello_NoGreaseCipher(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	ch := buildFirefoxHello("example.com")
	if cipherSuiteHasGrease(ch) {
		t.Error("Firefox cipher suites must not include GREASE (0x?a?a)")
	}
}

func TestBuildSafariHello_NoGreaseCipher(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	ch := buildSafariHello("example.com")
	if cipherSuiteHasGrease(ch) {
		t.Error("Safari cipher suites must not include GREASE (0x?a?a)")
	}
}

func cipherSuiteHasGrease(ch []byte) bool {
	if len(ch) < 44 {
		return false
	}
	// TLS record(5) + handshake hdr(4) + version(2) + random(32) + sid_len(1)
	off := 5 + 4 + 2 + 32
	if off >= len(ch) {
		return false
	}
	sidLen := int(ch[off])
	off++
	off += sidLen
	if off+2 > len(ch) {
		return false
	}
	csLen := int(ch[off])<<8 | int(ch[off+1])
	off += 2
	if off+csLen > len(ch) || csLen%2 != 0 {
		return false
	}
	for i := 0; i < csLen; i += 2 {
		cs := uint16(ch[off+i])<<8 | uint16(ch[off+i+1])
		if cs&0x0f0f == 0x0a0a {
			return true
		}
	}
	return false
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

func TestValidate_LegacyAmneziaSWithoutHPK(t *testing.T) {
	awg15 := AWGParams{Jmin: 10, Jmax: 50, S1: 59, S2: 106, S3: 0, S4: 0}
	if err := awg15.Validate(); err != nil {
		t.Fatalf("AWG 1.5 S3=S4=0 must import: %v", err)
	}
	awg2 := AWGParams{Jmin: 10, Jmax: 50, S1: 45, S2: 135, S3: 1, S4: 12}
	if err := awg2.Validate(); err != nil {
		t.Fatalf("AWG2 S3=1 must import: %v", err)
	}
	awg3 := awg2
	awg3.HeaderProtectionKey = "dGVzdC1ocGstMzItYnl0ZXMta2V5ISE="
	if err := awg3.Validate(); err == nil {
		t.Fatal("HPK + S3=1 must fail")
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

// parseAwg3Range parses an awg3Range output ("lo" or "lo-hi") into lo, hi.
func parseAwg3Range(t *testing.T, s string) (int, int) {
	t.Helper()
	var lo, hi int
	if n, err := fmt.Sscanf(s, "%d-%d", &lo, &hi); err == nil && n == 2 {
		return lo, hi
	}
	if n, err := fmt.Sscanf(s, "%d", &lo); err == nil && n == 1 {
		return lo, lo
	}
	t.Fatalf("range %q is neither \"lo\" nor \"lo-hi\"", s)
	return 0, 0
}

// TestGenerateAwg3DeviceTimings verifies the Architect-ported generation: all
// six fields are non-empty well-formed ranges within u16, and the WireGuard
// timer invariants hold (RejectAfterTime comfortably above
// KeepaliveTimeout+RekeyTimeout, RekeyAfterTime below RejectAfterTime,
// MaxHandshakeAttempts >= 1).
func TestGenerateAwg3DeviceTimings_FormatAndInvariants(t *testing.T) {
	for _, prof := range []ObfProfile{ObfLite, ObfStandard, ObfPro} {
		prof := prof
		t.Run(string(prof), func(t *testing.T) {
			SetRand(crand.New(crand.NewSource(42)))
			d := GenerateAwg3DeviceTimings(prof)
			fields := map[string]string{
				"ContentPaddingAddition": d.ContentPaddingAddition,
				"RekeyAfterTime":         d.RekeyAfterTime,
				"RekeyTimeout":           d.RekeyTimeout,
				"RejectAfterTime":        d.RejectAfterTime,
				"KeepaliveTimeout":       d.KeepaliveTimeout,
				"MaxHandshakeAttempts":   d.MaxHandshakeAttempts,
			}
			parsed := make(map[string][2]int, len(fields))
			for name, s := range fields {
				if s == "" {
					t.Fatalf("%s: empty (must be generated for v3)", name)
				}
				lo, hi := parseAwg3Range(t, s)
				if lo > hi {
					t.Errorf("%s=%q: lo %d > hi %d", name, s, lo, hi)
				}
				if lo < 0 || hi > 65535 {
					t.Errorf("%s=%q: out of u16 range", name, s)
				}
				parsed[name] = [2]int{lo, hi}
			}
			// Invariant: RejectAfterTime low end >= 170 and strictly above the
			// receiving-side refresh window (KeepaliveTimeout + RekeyTimeout highs).
			if parsed["RejectAfterTime"][0] < 170 {
				t.Errorf("RejectAfterTime lo=%d < 170", parsed["RejectAfterTime"][0])
			}
			if parsed["RejectAfterTime"][0] <= parsed["KeepaliveTimeout"][1]+parsed["RekeyTimeout"][1] {
				t.Errorf("RejectAfterTime lo=%d must exceed KeepaliveTimeout.hi+RekeyTimeout.hi=%d",
					parsed["RejectAfterTime"][0], parsed["KeepaliveTimeout"][1]+parsed["RekeyTimeout"][1])
			}
			// Invariant: RekeyAfterTime finishes before RejectAfterTime starts.
			if parsed["RekeyAfterTime"][1] >= parsed["RejectAfterTime"][0] {
				t.Errorf("RekeyAfterTime hi=%d must be < RejectAfterTime lo=%d",
					parsed["RekeyAfterTime"][1], parsed["RejectAfterTime"][0])
			}
			// Invariant: MaxHandshakeAttempts >= 1.
			if parsed["MaxHandshakeAttempts"][0] < 1 {
				t.Errorf("MaxHandshakeAttempts lo=%d < 1", parsed["MaxHandshakeAttempts"][0])
			}
		})
	}
}

// shrinkCPS must measure the netlink cost, not the decoded payload: both sets
// below sit under MaxIPayload, so the old PayloadSum loop left them untouched
// while `awg show` on the resulting interface fails with EMSGSIZE.
func TestShrinkCPS_ByteMetric(t *testing.T) {
	tag := func(payloadBytes int) string { return "<b 0x" + strings.Repeat("ab", payloadBytes) + ">" }
	t.Run("clears a lone oversized I1", func(t *testing.T) {
		r := shrinkCPS(CPSResult{I1: tag(1800)})
		if r.I1 != "" {
			t.Fatalf("I1 must be cleared when it alone busts the budget (%d > %d bytes)",
				CPSResult{I1: tag(1800)}.IBytes(), MaxIBytes)
		}
	})
	t.Run("drops only what the budget requires", func(t *testing.T) {
		full := CPSResult{I1: tag(355), I2: tag(355), I3: tag(355), I4: tag(355), I5: tag(355)}
		r := shrinkCPS(full)
		if r.I5 != "" {
			t.Fatalf("I5 must go: %d > %d bytes", full.IBytes(), MaxIBytes)
		}
		if r.I1 == "" || r.I4 == "" {
			t.Fatalf("dropping I5 alone brings the set to %d bytes; I1-I4 must survive, got %+v", r.IBytes(), r)
		}
	})
}
