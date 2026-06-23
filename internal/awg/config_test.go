// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGenerateConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		params   *AWGParams
		checks   []string
		notExist []string
	}{
		{
			name:   "default_obfuscation_keys_present",
			params: mustGen(1, "quic", "ru"),
			checks: []string{
				"[Interface]",
				"PrivateKey = ",
				"MTU = 1320",
				"Jc = ", "Jmin = ", "Jmax = ",
				"S1 = ", "S2 = ", "S3 = ", "S4 = ",
				"H1 = ", "H2 = ", "H3 = ", "H4 = ",
				"PostUp = ", "PostDown = ",
			},
			notExist: []string{"<nil>", "(null)", "undefined", "%!s"},
		},
		{
			name:   "jumbo_random_all_headers",
			params: mustGen(3, "quic", "ru"),
			checks: []string{
				"Jc = ", "Jmin = ", "Jmax = ",
				"H1 = ", "H2 = ", "H3 = ", "H4 = ",
			},
		},
		{
			name:   "sip_profile_server_config",
			params: mustGen(2, "sip", "ru"),
			checks: []string{
				"[Interface]",
				"PrivateKey = ",
				"PostUp = ", "PostDown = ",
			},
		},
		{
			name:   "dns_profile_padding_params",
			params: mustGen(1, "dns", "world"),
			checks: []string{
				"MTU = 1320",
				"S1 = ", "S2 = ", "S3 = ", "S4 = ",
			},
		},
		{
			name:   "exact_custom_values",
			params: &AWGParams{
				PrivateKey:    "priv123",
				PublicKey:     "pub123",
				PresharedKey:  "psk123",
				MTU:           1420,
				Jc:            5, Jmin: 75, Jmax: 200,
				S1: 30, S2: 60, S3: 20, S4: 10,
				H1: "100000-500000", H2: "600000-900000",
				H3: "1000000-1500000", H4: "1600000-2000000",
				ObfLevel: 1, MimicryProfile: "quic", Region: "ru",
			},
			checks: []string{
				"[Interface]",
				"PrivateKey = priv123",
				"MTU = 1420",
				"Jc = 5", "Jmin = 75", "Jmax = 200",
				"S1 = 30", "S2 = 60", "S3 = 20", "S4 = 10",
				"H1 = 100000-500000", "H2 = 600000-900000",
				"H3 = 1000000-1500000", "H4 = 1600000-2000000",
				"ListenPort = 0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildAWGConfigRaw(tt.params, TemplateData{
				AWGServerIP: "10.0.0.1",
				AWGPort:     0,
			})
			for _, check := range tt.checks {
				if !strings.Contains(config, check) {
					t.Errorf("[%s] missing: %s\nConfig:\n%s", tt.name, check, config)
				}
			}
			for _, ne := range tt.notExist {
				if strings.Contains(config, ne) {
					t.Errorf("[%s] should not contain: %s", tt.name, ne)
				}
			}
		})
	}
}

func TestBuildAWGConfig_HexHeaders(t *testing.T) {
	params, _ := GenerateAWGParams(3, "quic", "ru")
	if !strings.Contains(params.H1, "-") {
		t.Error("H1 must contain dash separator")
	}
	if !strings.Contains(params.H4, "-") {
		t.Error("H4 must contain dash separator")
	}
}

func mustGen(level int, profile, region string) *AWGParams {
	p, err := GenerateAWGParams(level, profile, region)
	if err != nil {
		panic(err)
	}
	return p
}

func buildAWGConfigRaw(params *AWGParams, data TemplateData) string {
	awg := &model.Inbound{Port: data.AWGPort}
	if err := MergeParamsToSettings(awg, params, "", "", "", "", ""); err != nil {
		panic(err)
	}
	return BuildServerConfig(awg, "/tmp/up.sh", "/tmp/down.sh")
}
