// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"
)

func TestGenerateAWGParams_Basic(t *testing.T) {
	params, err := GenerateAWGParams(1, "quic", "ru")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.PrivateKey == "" || params.PublicKey == "" || params.PresharedKey == "" {
		t.Fatal("keys must not be empty")
	}
	if params.PrivateKey == params.PublicKey {
		t.Fatal("private and public keys must differ")
	}
}

func TestGenerateAWGParams_ObfuscationRanges(t *testing.T) {
	for i := 0; i < 20; i++ {
		params, err := GenerateAWGParams(3, "quic", "ru")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if params.Jc < 4 || params.Jc > 16 {
			t.Errorf("Jc out of range: %d", params.Jc)
		}
		if params.Jmin < 50 || params.Jmin > 256 {
			t.Errorf("Jmin out of range: %d", params.Jmin)
		}
		if params.Jmax < 300 || params.Jmax > 1000 {
			t.Errorf("Jmax out of range: %d", params.Jmax)
		}
		if params.Jmin >= params.Jmax {
			t.Errorf("Jmin (%d) must be < Jmax (%d)", params.Jmin, params.Jmax)
		}
		if params.S1 < 15 || params.S1 > 150 {
			t.Errorf("S1 out of range: %d", params.S1)
		}
		if params.S2 < 15 || params.S2 > 150 {
			t.Errorf("S2 out of range: %d", params.S2)
		}
		if params.S1+56 == params.S2 {
			t.Errorf("S1+56 must not equal S2 (DPI detection risk)")
		}
		if params.S3 < 8 || params.S3 > 64 {
			t.Errorf("S3 out of range: %d", params.S3)
		}
		if params.S4 < 6 || params.S4 > 31 {
			t.Errorf("S4 out of range: %d", params.S4)
		}
	}
}

func TestGenerateAWGParams_QuadrantHeaders(t *testing.T) {
	params, _ := GenerateAWGParams(3, "quic", "ru")
	if !strings.Contains(params.H1, "-") || !strings.Contains(params.H2, "-") ||
	   !strings.Contains(params.H3, "-") || !strings.Contains(params.H4, "-") {
		t.Fatal("H1-H4 must be range strings like '427819-925639'")
	}
}

func TestGenerateAWGParams_InvalidObfLevel(t *testing.T) {
	_, err := GenerateAWGParams(0, "quic", "ru")
	if err == nil {
		t.Fatal("expected error for obfLevel 0")
	}
	_, err = GenerateAWGParams(4, "quic", "ru")
	if err == nil {
		t.Fatal("expected error for obfLevel 4")
	}
}

func TestGenerateAWGParams_InvalidProfile(t *testing.T) {
	_, err := GenerateAWGParams(1, "invalid", "ru")
	if err == nil {
		t.Fatal("expected error for invalid profile")
	}
}
