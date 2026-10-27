// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	crand "math/rand"
	"strings"
	"testing"
)

const bedrockMagicHex = "00ffff00fefefefefdfdfdfd12345678"

// TestGenerateAWGParams_Minecraft guards the Bedrock (RakNet) profile: panel
// invariants hold on every draw, the v2 headers are the actual RakNet packet
// IDs, and v3 header protection uses the standard compat values 1-4 (HPK
// encrypts the header, same choice as ObfPremium).
func TestGenerateAWGParams_Minecraft(t *testing.T) {
	SetRand(crand.New(crand.NewSource(42)))
	for _, version := range []string{"1.5", "2", "3", "3.1"} {
		for i := 0; i < 200; i++ {
			p, err := GenerateAWGParams(ObfMinecraft, version)
			if err != nil {
				t.Fatalf("version %q iter %d: %v", version, i, err)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("version %q iter %d validate: %v (params %+v)", version, i, err, p)
			}
			for _, s := range []int{p.S1, p.S2, p.S3, p.S4} {
				if s < MinSForHPK {
					t.Fatalf("version %q iter %d: S value %d < MinSForHPK %d", version, i, s, MinSForHPK)
				}
			}
			if p.Jmin >= p.Jmax || p.Jmin < 32 || p.Jmax > 800 {
				t.Fatalf("version %q iter %d: junk Jmin=%d Jmax=%d outside the Bedrock band", version, i, p.Jmin, p.Jmax)
			}
			var want string
			switch version {
			case "1.5":
				want = "5,6,28,128"
			case "3", "3.1":
				want = "1,2,3,4"
			default:
				want = "5,6,28,128-141"
			}
			got := strings.Join([]string{p.H1, p.H2, p.H3, p.H4}, ",")
			if got != want {
				t.Fatalf("version %q iter %d: H1-H4 = %q, want %q", version, i, got, want)
			}
			p2, err := p.WithHeaderProtectionKey()
			if err != nil {
				t.Fatalf("version %q iter %d hpk: %v", version, i, err)
			}
			if p2.HeaderProtectionKey == "" {
				t.Fatalf("version %q iter %d: HPK not populated", version, i)
			}
		}
	}
}

// TestGenerateMinecraftCPS_Content checks the RakNet literals from generatorM:
// shared client GUID, bitwise-NOT IPv4 in I3, MTU-sized I2, frame-set I4/I5.
func TestGenerateMinecraftCPS_Content(t *testing.T) {
	SetRand(crand.New(crand.NewSource(11)))
	r, err := GenerateMinecraftCPS("127.0.0.1", 19133, false, kernelBudget())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.I1, "<b 0x01><r 8><b 0x"+bedrockMagicHex+"><b 0x") {
		t.Fatalf("I1 not a RakNet unconnected ping: %q", r.I1)
	}
	guid := strings.TrimSuffix(strings.TrimPrefix(r.I1, "<b 0x01><r 8><b 0x"+bedrockMagicHex+"><b 0x"), ">")
	if len(guid) != 16 {
		t.Fatalf("I1 GUID = %q, want 8 bytes", guid)
	}
	if !strings.HasPrefix(r.I2, "<b 0x05><b 0x"+bedrockMagicHex+"><b 0x0b><b 0x"+strings.Repeat("00", 64)) {
		t.Fatalf("I2 not an MTU-probe open-connection request 1: %q", r.I2)
	}
	// ~127.0.0.1 = 80fffffe, port 19133 = 4abd, decoy MTU 1280 = 0500.
	wantI3 := "<b 0x07><b 0x" + bedrockMagicHex + "><b 0x04><b 0x80fffffe><b 0x4abd><b 0x0500><b 0x" + guid + ">"
	if r.I3 != wantI3 {
		t.Fatalf("I3 = %q, want %q", r.I3, wantI3)
	}
	if !strings.HasPrefix(r.I4, "<b 0x84><b 0x000000><b 0x00><b 0x") || !strings.Contains(r.I4, "><r ") {
		t.Fatalf("I4 frame-set shape wrong: %q", r.I4)
	}
	if !strings.HasPrefix(r.I5, "<b 0x84><b 0x010000><b 0x00><b 0x") {
		t.Fatalf("I5 frame-set shape wrong: %q", r.I5)
	}
	if r.IBytes() > kernelBudget() {
		t.Fatalf("I1-I5 exceed the netlink budget: %d > %d", r.IBytes(), kernelBudget())
	}
	onlyI1, err := GenerateMinecraftCPS("127.0.0.1", 19132, true, kernelBudget())
	if err != nil {
		t.Fatal(err)
	}
	if onlyI1.I1 == "" || onlyI1.I2 != "" || onlyI1.I5 != "" {
		t.Fatalf("onlyI1 must render I1 only: %+v", onlyI1)
	}
	if _, err := GenerateMinecraftCPS("127.0.0.1", 19132, false, 200); err == nil {
		t.Fatal("a 200-byte budget must refuse the full set")
	}
}

// TestSanitizeBedrockEndpoint pins the placeholder fallbacks the source
// generator uses: an unusable endpoint falls back to TEST-NET-1 and the
// Bedrock default UDP port 19132, never to an empty or malformed decoy.
func TestSanitizeBedrockEndpoint(t *testing.T) {
	if ip, port := sanitizeBedrockEndpoint("203.0.113.5", 19133); ip != "203.0.113.5" || port != 19133 {
		t.Fatalf("get %s:%d, want 203.0.113.5:19133", ip, port)
	}
	if ip, port := sanitizeBedrockEndpoint("", 0); ip != "192.0.2.1" || port != 19132 {
		t.Fatalf("fallback ip/port = %s:%d, want 192.0.2.1:19132", ip, port)
	}
	if ip, _ := sanitizeBedrockEndpoint("minecraft.example.com", 0); ip != "192.0.2.1" {
		t.Fatalf("IPv6/domain must fall back to TEST-NET-1, got %s", ip)
	}
	if _, port := sanitizeBedrockEndpoint("1.2.3.4", 70000); port != 19132 {
		t.Fatalf("out-of-range port must fall back to 19132, got %d", port)
	}
}

func TestGenerateAwg3DeviceTimings_MinecraftBalanced(t *testing.T) {
	d := GenerateAwg3DeviceTimings(ObfMinecraft)
	if d.ContentPaddingAddition != "16-128" || d.RekeyAfterTime != "120-150" ||
		d.RekeyTimeout != "5-8" || d.RejectAfterTime != "190-240" ||
		d.KeepaliveTimeout != "10-20" || d.MaxHandshakeAttempts != "8-12" {
		t.Fatalf("minecraft timings = %+v, want generatorM balanced", d)
	}
}

// TestBedrockCPS_AcceptedByUserspaceEngine feeds the full RakNet set through
// amneziawg-go's IpcSet parser — a .conf the client cannot import is worthless.
func TestBedrockCPS_AcceptedByUserspaceEngine(t *testing.T) {
	SetRand(crand.New(crand.NewSource(3)))
	r, err := GenerateMinecraftCPS("203.0.113.5", 19132, false, kernelBudget())
	if err != nil {
		t.Fatal(err)
	}
	set := [5]string{r.I1, r.I2, r.I3, r.I4, r.I5}
	if err := ipcSet(t, set); err != nil {
		t.Fatalf("amneziawg-go rejected the Bedrock CPS set: %v\nset: %q", err, set)
	}
}
