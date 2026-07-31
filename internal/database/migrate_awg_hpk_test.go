// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAwgSettings(t *testing.T) {
	for _, tc := range []struct {
		name        string
		in          string
		wantChanged bool
		wantVersion string
		wantHPK     string // "" means the key must be absent or empty; checkHPK verifies presence
		checkHPK    bool   // when true, assert headerProtectionKey == wantHPK after migration
	}{
		{
			name:        "no version + non-empty key → backfill v2 + clear hpk",
			in:          `{"privateKey":"k","jc":8,"headerProtectionKey":"aBcD...base64hpk=="}`,
			wantChanged: true,
			wantVersion: "2",
			wantHPK:     "",
			checkHPK:    true,
		},
		{
			name:        "no version + no key → backfill v2 only",
			in:          `{"privateKey":"k","jc":8}`,
			wantChanged: true,
			wantVersion: "2",
		},
		{
			name:        "v2 + non-empty key → clear hpk, keep v2",
			in:          `{"privateKey":"k","awgVersion":"2","headerProtectionKey":"aBcD...base64hpk=="}`,
			wantChanged: true,
			wantVersion: "2",
			wantHPK:     "",
			checkHPK:    true,
		},
		{
			name:        "v3 + non-empty key → preserved untouched",
			in:          `{"privateKey":"k","awgVersion":"3","headerProtectionKey":"aBcD...base64hpk=="}`,
			wantChanged: false,
			wantVersion: "3",
		},
		{
			name:        "v2 + empty key → no change",
			in:          `{"privateKey":"k","awgVersion":"2","headerProtectionKey":""}`,
			wantChanged: false,
			wantVersion: "2",
		},
		{
			name:        "v3 + empty key → no change",
			in:          `{"privateKey":"k","awgVersion":"3","headerProtectionKey":""}`,
			wantChanged: false,
			wantVersion: "3",
		},
		{
			name:        "garbage version normalized to v2",
			in:          `{"privateKey":"k","awgVersion":"banana"}`,
			wantChanged: true,
			wantVersion: "2",
		},
		{
			name:        "v1.5 preserved",
			in:          `{"privateKey":"k","awgVersion":"1.5","headerProtectionKey":""}`,
			wantChanged: false,
			wantVersion: "1.5",
		},
		{
			name:        "invalid json is a no-op",
			in:          `not json`,
			wantChanged: false,
		},
		{
			name:        "empty string is a no-op",
			in:          ``,
			wantChanged: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, _ := normalizeAwgSettings(tc.in)
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v (out=%s)", changed, tc.wantChanged, got)
			}
			if !changed {
				if got != tc.in {
					t.Fatalf("unchanged input must be returned verbatim, got %s", got)
				}
				return
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(got), &m); err != nil {
				t.Fatalf("output is not valid JSON: %v (%s)", err, got)
			}
			if tc.wantVersion != "" {
				if m["awgVersion"] != tc.wantVersion {
					t.Fatalf("awgVersion = %v, want %q", m["awgVersion"], tc.wantVersion)
				}
			}
			if tc.checkHPK {
				hpk, _ := m["headerProtectionKey"].(string)
				if hpk != tc.wantHPK {
					t.Fatalf("headerProtectionKey = %q, want %q", hpk, tc.wantHPK)
				}
			}
		})
	}
}

// The migration must not collaterally damage the rest of the settings blob:
// losing privateKey, the obfuscation numbers, or the clients array would break
// the inbound just as badly as the poisoned key did.
func TestNormalizeAwgSettings_PreservesOtherFields(t *testing.T) {
	in := `{"privateKey":"serverPriv","publicKey":"serverPub","address":"10.8.0.1/24","mtu":1320,` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"h1":"100-500","i1":"<b 0xAA>",` +
		`"headerProtectionKey":"aBcD...base64hpk==",` +
		`"clients":[{"email":"user","publicKey":"peerPub","comment":"keep me"}]}`
	got, changed, _ := normalizeAwgSettings(in)
	if !changed {
		t.Fatal("expected the settings to be normalized")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for k, want := range map[string]any{
		"privateKey": "serverPriv",
		"publicKey":  "serverPub",
		"address":    "10.8.0.1/24",
		"mtu":        float64(1320),
		"jc":         float64(8),
		"jmin":       float64(50),
		"jmax":       float64(200),
		"s1":         float64(30),
		"h1":         "100-500",
		"i1":         "<b 0xAA>",
		"awgVersion": "2",
	} {
		if m[k] != want {
			t.Errorf("%s = %v, want %v", k, m[k], want)
		}
	}
	clients, ok := m["clients"].([]any)
	if !ok || len(clients) != 1 {
		t.Fatalf("clients array lost: %v", m["clients"])
	}
	client, ok := clients[0].(map[string]any)
	if !ok {
		t.Fatalf("client entry malformed: %v", clients[0])
	}
	if client["comment"] != "keep me" {
		t.Errorf("client comment lost: %v", client["comment"])
	}
}
