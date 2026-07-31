// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import "testing"

func TestParseConf_Client(t *testing.T) {
	conf := `[Interface]
PrivateKey = abcDEF
Address = 10.9.0.5/32
MTU = 1320
Table = off
Jc = 3
Jmin = 50
Jmax = 150

[Peer]
PublicKey = upstreamPub
Endpoint = up.example.com:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`
	s, err := ParseConf(conf)
	if err != nil {
		t.Fatalf("ParseConf: %v", err)
	}
	if s.PrivateKey != "abcDEF" {
		t.Errorf("PrivateKey = %q", s.PrivateKey)
	}
	if s.Address != "10.9.0.5/32" {
		t.Errorf("Address = %q", s.Address)
	}
	if s.MTU != 1320 {
		t.Errorf("MTU = %d", s.MTU)
	}
	if s.PublicKey != "upstreamPub" {
		t.Errorf("PublicKey = %q", s.PublicKey)
	}
	if s.Endpoint != "up.example.com:51820" {
		t.Errorf("Endpoint = %q", s.Endpoint)
	}
	if s.Keepalive != 25 {
		t.Errorf("Keepalive = %d", s.Keepalive)
	}
	if s.Jc != 3 {
		t.Errorf("Jc = %d", s.Jc)
	}
	// No S3/S4, I1-I5, or HeaderProtectionKey → auto-detected as legacy "1.5".
	if s.AwgVersion != "1.5" {
		t.Errorf("AwgVersion = %q, want \"1.5\" (legacy field set)", s.AwgVersion)
	}
}

func TestParseConf_Empty(t *testing.T) {
	s, err := ParseConf("")
	if err != nil {
		t.Fatalf("ParseConf empty: %v", err)
	}
	if s.PrivateKey != "" || s.Address != "" {
		t.Errorf("expected zero-value ClientSettings, got %+v", s)
	}
}

func TestParseConf_CommentsAndWhitespace(t *testing.T) {
	conf := `# my comment
[Interface]

PrivateKey = k
; another comment
Address = 10.9.0.5/32
`
	s, _ := ParseConf(conf)
	if s.PrivateKey != "k" {
		t.Errorf("PrivateKey = %q, want k", s.PrivateKey)
	}
	if s.Address != "10.9.0.5/32" {
		t.Errorf("Address = %q", s.Address)
	}
}

// TestParseConf_AwgVersions verifies ParseConf eats a .conf of any AWG version
// and auto-detects the protocol version from the field set, so a pasted v3
// config keeps its HeaderProtectionKey and renders as version "3".
func TestParseConf_AwgVersions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		conf    string
		wantVer string
		wantHPK string
	}{
		{
			name: "v3 with HeaderProtectionKey → version 3, HPK kept",
			conf: `[Interface]
PrivateKey = k
Address = 10.9.0.5/32
Jc = 5
Jmin = 50
Jmax = 200
S1 = 30
S2 = 60
S3 = 20
S4 = 25
H1 = 100000-500000
HeaderProtectionKey = aBcD...base64hpk==

[Peer]
PublicKey = pub
Endpoint = up:51820
`,
			wantVer: "3",
			wantHPK: "aBcD...base64hpk==",
		},
		{
			name: "v2 with S3/S4 and I1-I5, no HPK → version 2",
			conf: `[Interface]
PrivateKey = k
Address = 10.9.0.5/32
Jc = 5
S1 = 30
S2 = 60
S3 = 20
S4 = 25
I1 = <b 0xaa>

[Peer]
PublicKey = pub
Endpoint = up:51820
`,
			wantVer: "2",
			wantHPK: "",
		},
		{
			name: "legacy with Jc/S1/S2 only → version 1.5",
			conf: `[Interface]
PrivateKey = k
Address = 10.9.0.5/32
Jc = 3
S1 = 30
S2 = 60

[Peer]
PublicKey = pub
Endpoint = up:51820
`,
			wantVer: "1.5",
			wantHPK: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ParseConf(tc.conf)
			if err != nil {
				t.Fatalf("ParseConf: %v", err)
			}
			if s.AwgVersion != tc.wantVer {
				t.Errorf("AwgVersion = %q, want %q", s.AwgVersion, tc.wantVer)
			}
			if s.HeaderProtectionKey != tc.wantHPK {
				t.Errorf("HeaderProtectionKey = %q, want %q", s.HeaderProtectionKey, tc.wantHPK)
			}
		})
	}
}
