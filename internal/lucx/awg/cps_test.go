// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"
)

func TestGenerateCPS_Level1_NoSignatures(t *testing.T) {
	i1, i2, i3, i4, i5 := GenerateCPS(1, CPSProfileQUIC)
	if i1 != "" || i2 != "" || i3 != "" || i4 != "" || i5 != "" {
		t.Fatal("level 1 should produce no CPS signatures")
	}
}

func TestGenerateCPS_Level2_I1Only(t *testing.T) {
	i1, i2, i3, i4, i5 := GenerateCPS(2, CPSProfileQUIC)
	if i1 == "" {
		t.Fatal("level 2 should produce I1")
	}
	if i2 != "" || i3 != "" || i4 != "" || i5 != "" {
		t.Fatal("level 2 should only produce I1")
	}
}

func TestGenerateCPS_Level3_FullChain_QUIC(t *testing.T) {
	i1, i2, i3, i4, i5 := GenerateCPS(3, CPSProfileQUIC)
	if i1 == "" || i2 == "" || i3 == "" || i4 == "" || i5 == "" {
		t.Fatal("level 3 QUIC should produce I1-I5")
	}
	// QUIC Initial should be ~2400 hex chars (1200 bytes)
	if len(i1) < 2000 {
		t.Errorf("QUIC Initial too short: %d hex chars", len(i1))
	}
}

func TestGenerateCPS_SIP(t *testing.T) {
	i1, i2, _, _, _ := GenerateCPS(2, CPSProfileSIP)
	if i1 == "" {
		t.Fatal("SIP should produce I1")
	}
	if i2 != "" {
		t.Fatal("SIP should only produce I1")
	}
}

func TestGenerateCPS_DNS(t *testing.T) {
	i1, i2, _, _, _ := GenerateCPS(3, CPSProfileDNS)
	if i1 == "" || i2 == "" {
		t.Fatal("DNS should produce I1 and I2")
	}
}
