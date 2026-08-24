// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestParseServerConf_ToolzaPeers(t *testing.T) {
	const conf = `[Interface]
PrivateKey = serverpriv
Address = 10.8.1.1/24
ListenPort = 40677
Jc = 4
Jmin = 10
Jmax = 50
S1 = 40
S2 = 60
H1 = 1
H2 = 2
H3 = 3
H4 = 4

[Peer]
# alice
# expires=1700000000
PublicKey = pubA
PresharedKey = pskA
AllowedIPs = 10.8.1.2/32

[Peer]
# bob
PublicKey = pubB
AllowedIPs = 127.0.0.2/32
# orig_ips=10.8.1.3/32
`
	s := ParseServerConf(conf)
	if s.ListenPort != 40677 || s.Address != "10.8.1.1/24" || s.Jc != 4 {
		t.Fatalf("interface: %+v", s)
	}
	if s.AwgVersion != "1.5" {
		t.Fatalf("version = %s", s.AwgVersion)
	}
	if len(s.Peers) != 2 {
		t.Fatalf("peers = %d", len(s.Peers))
	}
	if s.Peers[0].Name != "alice" || s.Peers[0].PSK != "pskA" || s.Peers[0].Expiry != 1700000000 {
		t.Fatalf("alice: %+v", s.Peers[0])
	}
	if !s.Peers[1].Suspended || s.Peers[1].OrigIPs != "10.8.1.3/32" {
		t.Fatalf("bob suspend: %+v", s.Peers[1])
	}
}

func TestParseServerConf_SeventyPeers(t *testing.T) {
	var b strings.Builder
	b.WriteString("[Interface]\nPrivateKey = k\nAddress = 10.9.0.1/24\nListenPort = 1\n")
	for i := 2; i <= 71; i++ {
		b.WriteString("\n[Peer]\n# user")
		b.WriteString(strings.Repeat("x", 0))
		b.WriteString("\nPublicKey = pub")
		b.WriteString(strings.Repeat("A", 8))
		b.WriteByte(byte('A' + (i % 26)))
		b.WriteString("\nAllowedIPs = 10.9.0.")
		b.WriteString(string(rune('0')))
		b.WriteString("/32\n")
	}
	// unique pubkeys
	b.Reset()
	b.WriteString("[Interface]\nPrivateKey = k\nAddress = 10.9.0.1/24\nListenPort = 1\n")
	for i := 0; i < 70; i++ {
		b.WriteString("\n[Peer]\n# u")
		b.WriteString(itoa(i))
		b.WriteString("\nPublicKey = ")
		b.WriteString(itoa(i))
		b.WriteString("pub\nAllowedIPs = 10.9.0.")
		b.WriteString(itoa(i + 2))
		b.WriteString("/32\n")
	}
	s := ParseServerConf(b.String())
	if len(s.Peers) != 70 {
		t.Fatalf("peers = %d, want 70", len(s.Peers))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestDiscover_EmptyIsJSONArray(t *testing.T) {
	got := Discover(DiscoverPaths{AmneziaDir: t.TempDir()})
	if got == nil {
		t.Fatal("Discover must return a non-nil empty slice")
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Fatalf("empty Discover JSON = %s, want []", raw)
	}
}

func TestDiscover_SkipsManagedAndKeepsForeign(t *testing.T) {
	dir := t.TempDir()
	foreign := `[Interface]
PrivateKey = foreign
Address = 10.8.0.1/24
ListenPort = 51820

[Peer]
# phone
PublicKey = pubX
AllowedIPs = 10.8.0.2/32
`
	if err := os.WriteFile(filepath.Join(dir, "awg0.conf"), []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	managed := xuiManagedMarker + "\n[Interface]\nPrivateKey = ours\nAddress = 10.9.0.1/24\nListenPort = 1\n"
	if err := os.WriteFile(filepath.Join(dir, "awg5.conf"), []byte(managed), 0o600); err != nil {
		t.Fatal(err)
	}
	got := Discover(DiscoverPaths{AmneziaDir: dir})
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(got), got)
	}
	if got[0].Ifname != "awg0" || got[0].PeerCount != 1 || got[0].NamedPeers != 1 {
		t.Fatalf("got %+v", got[0])
	}
}

func TestBuildInbound_PreservesKeys(t *testing.T) {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	c := ImportCandidate{
		Ifname: "awg0",
		Port:   40677,
		Conf: ServerConf{
			PrivateKey: "server-priv",
			Address:    "10.8.1.1/24",
			ListenPort: 40677,
			Jc:         5,
			AwgVersion: "2",
			Peers: []ServerPeer{{
				Name:       "alice",
				PublicKey:  pub,
				PSK:        "psk-keep",
				AllowedIPs: "10.8.1.2/32",
			}},
		},
		Keys:  map[string]ClientKeyFile{pub: {PrivateKey: priv}},
		Peers: []ImportPeer{{Email: "alice", PublicKey: pub, HasKey: true}},
	}
	built, err := BuildInbound(c, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if built.MissingKeys != 0 {
		t.Fatalf("missing = %d", built.MissingKeys)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(built.Inbound.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["privateKey"] != "server-priv" || settings["routeThroughXray"] != false {
		t.Fatalf("settings %+v", settings)
	}
	clients := settings["clients"].([]any)
	row := clients[0].(map[string]any)
	if row["privateKey"] != priv || row["publicKey"] != pub || row["preSharedKey"] != "psk-keep" {
		t.Fatalf("client %+v", row)
	}
	if row["allowedIPs"].([]any)[0] != "10.8.1.2/32" {
		t.Fatalf("ips %+v", row["allowedIPs"])
	}
}

func TestSanitizeEmailUnique(t *testing.T) {
	used := map[string]struct{}{}
	a := sanitizeEmail("Alice Phone", "10.8.0.2/32", "AAAA", used)
	b := sanitizeEmail("Alice Phone", "10.8.0.3/32", "BBBB", used)
	if a != "alice-phone" {
		t.Fatalf("first email = %q, want alice-phone", a)
	}
	if b != "alice-phone-2" {
		t.Fatalf("second email = %q, want alice-phone-2", b)
	}
}

func TestBuildInbound_ReservedEmailSuffix(t *testing.T) {
	c := ImportCandidate{
		Ifname: "awg0",
		Port:   1,
		Conf: ServerConf{
			PrivateKey: "k",
			Address:    "10.8.0.1/24",
			ListenPort: 1,
			Peers:      []ServerPeer{{Name: "alice", PublicKey: "P", AllowedIPs: "10.8.0.2/32"}},
		},
		Peers: []ImportPeer{{Email: "alice", PublicKey: "P"}},
	}
	built, err := BuildInbound(c, 1, map[string]struct{}{"alice": {}})
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(built.Inbound.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	email := settings["clients"].([]any)[0].(map[string]any)["email"]
	if email == "alice" {
		t.Fatalf("reserved email was not suffixed: %v", email)
	}
}

func TestBackupImportSources_CopiesServerAndClients(t *testing.T) {
	root := withTempConfigDir(t)
	srcDir := t.TempDir()
	server := filepath.Join(srcDir, "awg0.conf")
	client := filepath.Join(srcDir, "alice_awg2.conf")
	if err := os.WriteFile(server, []byte("[Interface]\nPrivateKey = srv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(client, []byte("[Interface]\nPrivateKey = cli\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := ImportCandidate{
		ID:       "awg-multi:awg0",
		ConfPath: server,
		Keys:     map[string]ClientKeyFile{"pub": {Path: client}},
	}
	dir, err := BackupImportSources(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, filepath.Join(root, "x-ui-backup")) {
		t.Fatalf("backup dir %s not under %s", dir, root)
	}
	if _, err := os.Stat(filepath.Join(dir, "awg0.conf")); err != nil {
		t.Fatalf("server copy missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "alice_awg2.conf")); err != nil {
		t.Fatalf("client copy missing: %v", err)
	}
	if _, err := os.Stat(server); err != nil {
		t.Fatal("original server conf must stay until adopt")
	}
}

func TestApplyClientCPS_OnlyMatchingPeer(t *testing.T) {
	conf := ServerConf{
		AwgVersion: "1.5",
		Peers:      []ServerPeer{{PublicKey: "peerA"}},
	}
	keys := map[string]ClientKeyFile{
		"other": {I1: "<b>stolen</b>"},
		"peerA": {I1: "<b>ours</b>"},
	}
	applyClientCPS(&conf, keys)
	if conf.I1 != "<b>ours</b>" {
		t.Fatalf("I1 = %q, want matching peer", conf.I1)
	}
}
