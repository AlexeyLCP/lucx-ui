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