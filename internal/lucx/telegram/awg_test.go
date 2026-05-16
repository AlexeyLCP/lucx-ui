// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telegram

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestBuildAWGConfigText_RequiredSections(t *testing.T) {
	client := model.Client{
		Email:    "test@awg",
		ID:       "pubkey12345",
		Password: "psk12345",
	}
	inbound := model.Inbound{
		Port:   55555,
		Listen: "5.9.1.2",
		Settings: `{
			"mtu": 1320,
			"jc": 10,
			"jmin": 100,
			"jmax": 500,
			"s1": 50,
			"s2": 80,
			"s3": 30,
			"s4": 15,
			"h1": "88830977-466888999",
			"h2": "577571549-1039919960",
			"h3": "1167874883-1558472606",
			"h4": "1739740840-2061202155"
		}`,
	}

	config := buildAWGConfigText(client, inbound, "5.9.1.2")

	checks := []string{
		"[Interface]",
		"[Peer]",
		"PrivateKey = ",
		"PublicKey = pubkey12345",
		"PresharedKey = psk12345",
		"Endpoint = 5.9.1.2:55555",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
		// Obfuscation keys
		"Jc = 10",
		"Jmin = 100",
		"Jmax = 500",
		"S1 = 50",
		"S2 = 80",
		"S3 = 30",
		"S4 = 15",
		"H1 = 88830977-466888999",
		"H2 = 577571549-1039919960",
		"H3 = 1167874883-1558472606",
		"H4 = 1739740840-2061202155",
	}

	for _, check := range checks {
		if !strings.Contains(config, check) {
			t.Errorf("config missing: %s", check)
		}
	}
}

func TestBuildAWGConfigText_Defaults(t *testing.T) {
	client := model.Client{Email: "test@awg", ID: "pk", Password: "psk"}
	inbound := model.Inbound{Port: 12345, Listen: "1.2.3.4", Settings: "{}"}

	config := buildAWGConfigText(client, inbound, "1.2.3.4")

	// Default values should be used
	defaults := []string{
		"MTU = 1320",
		"Jc = 8",
		"Jmin = 50",
		"Jmax = 500",
		"S1 = 50",
		"S2 = 80",
		"S3 = 30",
		"S4 = 15",
		"H1 = 88830977-466888999",
	}
	for _, d := range defaults {
		if !strings.Contains(config, d) {
			t.Errorf("default missing: %s\nGot:\n%s", d, config)
		}
	}
}

func TestBuildAWGConfigText_CPS(t *testing.T) {
	client := model.Client{Email: "cps@awg", ID: "pk", Password: "psk"}
	inbound := model.Inbound{
		Port:   443,
		Listen: "5.5.5.5",
		Settings: `{
			"i1": "abc123",
			"i2": "def456",
			"i3": "ghi789",
			"i4": "jkl012",
			"i5": "mno345"
		}`,
	}

	config := buildAWGConfigText(client, inbound, "5.5.5.5")

	for _, sig := range []string{"I1 = <b 0xabc123>", "I2", "I3", "I4", "I5"} {
		if !strings.Contains(config, sig) {
			t.Errorf("CPS signature missing: %s", sig)
		}
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := map[string]string{
		"test@client":   "test_client",
		"user@domain":   "user_domain",
		"simple":        "simple",
		"a@b.c":         "a_b.c",
		"name with spaces": "name_with_spaces",
	}
	for input, expected := range tests {
		result := sanitizeFileName(input)
		if result != expected {
			t.Errorf("sanitize(%s) = %s, want %s", input, result, expected)
		}
	}
}

// TestAWGConfig_SpecCompliance verifies the full .conf file matches spec.
func TestAWGConfig_SpecCompliance(t *testing.T) {
	client := model.Client{
		Email:      "client@awg",
		ID:         "cHVibGljS2V5QmFzZTY0U3RyaW5n",
		Password:   "cFNLQmFzZTY0U3RyaW5n",
		PrivateKey: "cHJpdmF0ZUtleUJhc2U2NFN0cmluZw==",
	}
	inbound := model.Inbound{
		Port:   55555,
		Listen: "5.9.1.2",
		Settings: `{
			"mtu": 1320,
			"jc": 8, "jmin": 50, "jmax": 500,
			"s1": 108, "s2": 27, "s3": 35, "s4": 16,
			"h1": "88830977-466888999",
			"h2": "577571549-1039919960",
			"h3": "1167874883-1558472606",
			"h4": "1739740840-2061202155",
			"clients": []
		}`,
	}

	expected := []string{
		"# client@awg — LucX-UI AWG Client",
		"[Interface]",
		"PrivateKey = cHJpdmF0ZUtleUJhc2U2NFN0cmluZw==",
		"Address = 10.100.0.2/32",
		"DNS = 1.1.1.1, 1.0.0.1",
		"MTU = 1320",
		"Jc = 8",
		"Jmin = 50",
		"Jmax = 500",
		"S1 = 108",
		"S2 = 27",
		"S3 = 35",
		"S4 = 16",
		"H1 = 88830977-466888999",
		"H2 = 577571549-1039919960",
		"H3 = 1167874883-1558472606",
		"H4 = 1739740840-2061202155",
		"[Peer]",
		"PublicKey = cHVibGljS2V5QmFzZTY0U3RyaW5n",
		"PresharedKey = cFNLQmFzZTY0U3RyaW5n",
		"Endpoint = 5.9.1.2:55555",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
	}

	config := buildAWGConfigText(client, inbound, "5.9.1.2")

	t.Logf("ACTUAL config:\n%s", config)

	for _, exp := range expected {
		if !strings.Contains(config, exp) {
			t.Errorf("MISSING: %s", exp)
		}
	}

	// Verify no placeholders remain
	placeholders := []string{"<CLIENT_PRIVATE_KEY>", "<SERVER_PUBKEY>", "<PSK>", "<GENERATE"}
	for _, ph := range placeholders {
		if strings.Contains(config, ph) {
			t.Errorf("PLACEHOLDER STILL PRESENT: %s", ph)
		}
	}
}
