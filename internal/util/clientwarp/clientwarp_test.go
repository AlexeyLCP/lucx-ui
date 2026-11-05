// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package clientwarp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantNil bool
		wantErr string
	}{
		{name: "blank disables", raw: "  ", wantNil: true},
		{name: "minimal", raw: `{"domains":["geosite:youtube"],"endpoints":["162.159.192.1:4500"]}`},
		{name: "ipv6 endpoint", raw: `{"domains":["domain:a.b"],"endpoints":["[2606:4700:d0::a29f:c001]:4500"]}`},
		{name: "not json", raw: `geosite:youtube`, wantErr: "invalid JSON"},
		{name: "unknown field", raw: `{"domains":["a"],"endpoints":["1.1.1.1:1"],"endpoint":"x"}`, wantErr: "invalid JSON"},
		{name: "no domains", raw: `{"domains":[],"endpoints":["1.1.1.1:1"]}`, wantErr: "domains must list"},
		{name: "blank domain", raw: `{"domains":[" "],"endpoints":["1.1.1.1:1"]}`, wantErr: "domains[0] is empty"},
		{name: "no endpoints", raw: `{"domains":["a"]}`, wantErr: "endpoints must list"},
		{name: "endpoint without port", raw: `{"domains":["a"],"endpoints":["1.1.1.1"]}`, wantErr: "not host:port"},
		{name: "endpoint bad port", raw: `{"domains":["a"],"endpoints":["1.1.1.1:70000"]}`, wantErr: "invalid port"},
		{name: "mtu too low", raw: `{"domains":["a"],"endpoints":["1.1.1.1:1"],"mtu":1200}`, wantErr: "mtu must be"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if (cfg == nil) != tt.wantNil {
				t.Fatalf("cfg = %v, wantNil %v", cfg, tt.wantNil)
			}
		})
	}
}

func TestParseTrimsEntries(t *testing.T) {
	cfg, err := Parse(`{"domains":[" geosite:youtube "],"endpoints":[" 8.39.214.9:4177 "]}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Domains[0] != "geosite:youtube" || cfg.Endpoints[0] != "8.39.214.9:4177" {
		t.Fatalf("entries not trimmed: %q %q", cfg.Domains[0], cfg.Endpoints[0])
	}
}

// The panel switch stores DefaultConfig; it must pass the same validation.
func TestDefaultConfigRoundTrips(t *testing.T) {
	raw, _ := json.Marshal(DefaultConfig)
	if _, err := Parse(string(raw)); err != nil {
		t.Fatalf("DefaultConfig rejected: %v", err)
	}
}

func TestEffectiveMTU(t *testing.T) {
	if got := (&Config{}).EffectiveMTU(); got != DefaultMTU {
		t.Fatalf("default = %d", got)
	}
	if got := (&Config{MTU: 1420}).EffectiveMTU(); got != 1420 {
		t.Fatalf("explicit = %d", got)
	}
}

func TestStreamSettings(t *testing.T) {
	def := (&Config{}).StreamSettings()
	udp := def["finalmask"].(map[string]any)["udp"].([]any)
	noise := udp[0].(map[string]any)["settings"].(map[string]any)["noise"].([]any)
	if len(noise) != 5 {
		t.Fatalf("default noise items = %d, want 5", len(noise))
	}
	packet := noise[0].(map[string]any)["packet"].(string)
	// 1250-byte QUIC Initial: long header with fixed bit set.
	if !strings.HasPrefix(packet, "<b 0xce") || len(packet) != len("<b 0x>")+2500 {
		t.Fatalf("default packet = %.20s… (len %d)", packet, len(packet))
	}

	custom := map[string]any{"udp": []any{}}
	got := (&Config{FinalMask: custom}).StreamSettings()["finalmask"].(map[string]any)
	if _, ok := got["udp"]; !ok || len(got) != 1 {
		t.Fatalf("custom finalmask not used: %v", got)
	}
}
