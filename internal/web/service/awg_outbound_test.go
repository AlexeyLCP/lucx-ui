// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

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

// TestAwgSettingsClientIPs covers the helper that feeds the outbound
// subnet-conflict guard: it must surface every single-host client address
// (bare or /32) and skip network entries like 0.0.0.0/0.
func TestAwgSettingsClientIPs(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		want     []string
	}{
		{"single client /32", `{"clients":[{"allowedIPs":["10.8.0.5/32"]}]}`, []string{"10.8.0.5"}},
		{"bare address", `{"clients":[{"allowedIPs":["10.8.0.7"]}]}`, []string{"10.8.0.7"}},
		{"network entry skipped", `{"clients":[{"allowedIPs":["0.0.0.0/0","10.8.0.9/32"]}]}`, []string{"10.8.0.9"}},
		{"multiple clients", `{"clients":[{"allowedIPs":["10.8.0.2/32"]},{"allowedIPs":["10.8.0.4/32"]}]}`, []string{"10.8.0.2", "10.8.0.4"}},
		{"no clients", `{"address":"10.8.1.1/24"}`, nil},
		{"malformed", `{not json`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := awgSettingsClientIPs(tt.settings)
			if len(got) != len(tt.want) {
				t.Fatalf("awgSettingsClientIPs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("awgSettingsClientIPs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestAwgOutboundSubnetClash covers the outbound-side subnet guard. The key
// regression (lucx.69): a provider conf landing in 10.8.0.0/24 must clash with
// an inbound whose CLIENTS sit in 10.8.0.0/24 even though that inbound's own
// settings.address is a different /24 (legacy wrong-subnet) — so the guard has
// to look at client addresses, not just server subnets.
func TestAwgOutboundSubnetClash(t *testing.T) {
	wrongSubnetInbound := &model.Inbound{
		Remark:   "awg2",
		Settings: `{"address":"10.8.1.1/24","clients":[{"allowedIPs":["10.8.0.2/32","10.8.0.5/32"]}]}`,
	}
	cleanInbound := &model.Inbound{
		Remark:   "awg13",
		Settings: `{"address":"11.85.5.1/24","clients":[{"allowedIPs":["11.85.5.2/32"]}]}`,
	}
	tests := []struct {
		name      string
		addr      string
		inbounds  []*model.Inbound
		wantClash bool
	}{
		{"outbound /24 over wrong-subnet clients", "10.8.0.3/24", []*model.Inbound{wrongSubnetInbound}, true},
		{"outbound on inbound server subnet", "10.8.1.9/24", []*model.Inbound{wrongSubnetInbound}, true},
		{"disjoint subnet", "12.80.1.2/24", []*model.Inbound{wrongSubnetInbound, cleanInbound}, false},
		{"single-host /32 exempt", "10.205.0.1/32", []*model.Inbound{wrongSubnetInbound}, false},
		{"empty address", "", []*model.Inbound{wrongSubnetInbound}, false},
		{"no inbounds", "10.8.0.3/24", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := awgOutboundSubnetClash(tt.addr, tt.inbounds)
			if (err != nil) != tt.wantClash {
				t.Errorf("awgOutboundSubnetClash(%q) err = %v, wantClash %v", tt.addr, err, tt.wantClash)
			}
		})
	}
}
