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

func TestStripHeaderProtectionKey(t *testing.T) {
	for _, tc := range []struct {
		name        string
		in          string
		wantChanged bool
	}{
		{
			name:        "non-empty key is cleared",
			in:          `{"privateKey":"k","jc":8,"headerProtectionKey":"aBcD...base64hpk=="}`,
			wantChanged: true,
		},
		{
			name:        "already empty key is left alone",
			in:          `{"privateKey":"k","headerProtectionKey":""}`,
			wantChanged: false,
		},
		{
			name:        "missing key is a no-op",
			in:          `{"privateKey":"k","jc":8}`,
			wantChanged: false,
		},
		{
			name:        "invalid json is returned verbatim",
			in:          `not json`,
			wantChanged: false,
		},
		{
			name:        "empty string is a no-op",
			in:          ``,
			wantChanged: false,
		},
		{
			name:        "non-string key value is still cleared",
			in:          `{"headerProtectionKey":123}`,
			wantChanged: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := stripHeaderProtectionKey(tc.in)
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
			hpk, ok := m["headerProtectionKey"]
			if !ok {
				t.Fatalf("key must be kept (blanked, not deleted) so the Zod shape holds, got %s", got)
			}
			if hpk != "" {
				t.Fatalf("key must be blanked, got %v", hpk)
			}
		})
	}
}

// The migration must not collaterally damage the rest of the settings blob:
// losing privateKey or the obfuscation numbers would break the inbound just as
// badly as the poisoned key did.
func TestStripHeaderProtectionKey_PreservesOtherFields(t *testing.T) {
	in := `{"privateKey":"serverPriv","publicKey":"serverPub","address":"10.8.0.1/24","mtu":1320,` +
		`"jc":8,"jmin":50,"jmax":200,"s1":30,"h1":"100-500","i1":"<b 0xAA>",` +
		`"headerProtectionKey":"aBcD...base64hpk==",` +
		`"clients":[{"email":"user","publicKey":"peerPub","comment":"keep me"}]}`
	got, changed := stripHeaderProtectionKey(in)
	if !changed {
		t.Fatal("expected the key to be cleared")
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
