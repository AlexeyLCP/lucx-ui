// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAWGConfig_ServerConfig(t *testing.T) {
	content := `# AmneziaWG Toolza — AWG 2.0 server config
# Region: ru
[Interface]
PrivateKey = sPrivKeyBase64==
Address = 10.100.0.1/24
ListenPort = 55555
MTU = 1347
Jc = 11
Jmin = 87
Jmax = 734
S1 = 45
S2 = 117
S3 = 42
S4 = 19
H1 = 113847-119305847
H2 = 536870914-715827874
H3 = 1073741826-1252658340
H4 = 1610612738-1789569700
I1 = <b 0xc0000000010bdce8ba67...>
I2 = <b 0x60d57a17a6a71a4b22...>
I3 = <b 0x483c00cb57dd5f44a2...>
I4 = <b 0x688c0325f9765207e5...>
I5 = <b 0x40ed4fd9a659c2145c...>
PostUp = /etc/amnezia/amneziawg/awg0-up.sh
PostDown = /etc/amnezia/amneziawg/awg0-down.sh

[Peer]
# client1
PublicKey = cPubKey1==
PresharedKey = psk1==
AllowedIPs = 10.100.0.2/32
`
	dir := t.TempDir()
	path := filepath.Join(dir, "awg0.conf")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := ParseAWGConfig(path)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	iface := cfg.Interface
	if iface.PrivateKey != "sPrivKeyBase64==" {
		t.Errorf("PrivateKey: got %q", iface.PrivateKey)
	}
	if iface.ListenPort != 55555 {
		t.Errorf("ListenPort: got %d", iface.ListenPort)
	}
	if iface.Jc != 11 {
		t.Errorf("Jc: got %d", iface.Jc)
	}
	if iface.Jmin != 87 {
		t.Errorf("Jmin: got %d", iface.Jmin)
	}
	if iface.Jmax != 734 {
		t.Errorf("Jmax: got %d", iface.Jmax)
	}
	if iface.S1 != 45 {
		t.Errorf("S1: got %d", iface.S1)
	}
	if iface.S2 != 117 {
		t.Errorf("S2: got %d", iface.S2)
	}
	if iface.S3 != 42 {
		t.Errorf("S3: got %d", iface.S3)
	}
	if iface.S4 != 19 {
		t.Errorf("S4: got %d", iface.S4)
	}
	if iface.H1 != "113847-119305847" {
		t.Errorf("H1: got %q", iface.H1)
	}
	if !strings.Contains(iface.I1, "0xc000") {
		t.Errorf("I1: got %q", iface.I1)
	}
	if iface.I5 != "<b 0x40ed4fd9a659c2145c...>" {
		t.Errorf("I5: got %q", iface.I5)
	}

	if len(cfg.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(cfg.Peers))
	}
	peer := cfg.Peers[0]
	if peer.PublicKey != "cPubKey1==" {
		t.Errorf("Peer PublicKey: got %q", peer.PublicKey)
	}
	if peer.PresharedKey != "psk1==" {
		t.Errorf("Peer PresharedKey: got %q", peer.PresharedKey)
	}

	if !cfg.HasObfuscation() {
		t.Error("HasObfuscation should be true")
	}
}

func TestParseAWGConfig_Minimal(t *testing.T) {
	content := `[Interface]
PrivateKey = minKey==
Address = 10.0.0.1/24
ListenPort = 12345
`
	dir := t.TempDir()
	path := filepath.Join(dir, "awg1.conf")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := ParseAWGConfig(path)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if cfg.Interface.PrivateKey != "minKey==" {
		t.Errorf("PrivateKey: got %q", cfg.Interface.PrivateKey)
	}
	if cfg.Interface.ListenPort != 12345 {
		t.Errorf("ListenPort: got %d", cfg.Interface.ListenPort)
	}
	if cfg.HasObfuscation() {
		t.Error("HasObfuscation should be false for minimal config")
	}
}

func TestParseAWGConfig_MultiplePeers(t *testing.T) {
	content := `[Interface]
PrivateKey = multi==
Address = 10.0.0.1/24
ListenPort = 55555

[Peer]
# alice
PublicKey = alicePK==
PresharedKey = alicePSK==

[Peer]
# bob
PublicKey = bobPK==
PresharedKey = bobPSK==
Endpoint = 5.9.1.2:55555
PersistentKeepalive = 25
`
	dir := t.TempDir()
	path := filepath.Join(dir, "awg2.conf")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := ParseAWGConfig(path)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(cfg.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(cfg.Peers))
	}
	if cfg.Peers[0].PublicKey != "alicePK==" {
		t.Errorf("peer0: got %q", cfg.Peers[0].PublicKey)
	}
	if cfg.Peers[1].Endpoint != "5.9.1.2:55555" {
		t.Errorf("peer1 endpoint: got %q", cfg.Peers[1].Endpoint)
	}
}

func TestAWGIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/etc/amnezia/amneziawg/awg0.conf", 0},
		{"/etc/amnezia/amneziawg/awg5.conf", 5},
		{"awg99.conf", 99},
	}
	for _, tt := range tests {
		got := AWGIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("AWGIDFromPath(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestDomainPoolByRegion(t *testing.T) {
	quicRU := DomainPoolByRegion(CPSProfileQUIC, "ru")
	if len(quicRU) == 0 {
		t.Error("QUIC RU pool should not be empty")
	}
	for _, d := range quicRU {
		if d == "" {
			t.Error("QUIC RU pool contains empty domain")
		}
	}

	sipRU := DomainPoolByRegion(CPSProfileSIP, "ru")
	if len(sipRU) == 0 {
		t.Error("SIP RU pool should not be empty")
	}

	dnsWorld := DomainPoolByRegion(CPSProfileDNS, "world")
	if len(dnsWorld) == 0 {
		t.Error("DNS WORLD pool should not be empty")
	}
}

func TestPickRandomDomain(t *testing.T) {
	d := PickRandomDomain(CPSProfileQUIC, "ru")
	if d == "" {
		t.Error("PickRandomDomain returned empty")
	}
	// Should be from the QUIC pool
	found := false
	for _, expected := range DomainPoolByRegion(CPSProfileQUIC, "ru") {
		if d == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("PickRandomDomain returned %q, not in QUIC RU pool", d)
	}
}
