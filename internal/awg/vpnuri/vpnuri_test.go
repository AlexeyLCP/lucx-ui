// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package vpnuri

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleConf = `[Interface]
PrivateKey = CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=
Address = 10.200.0.2/32
DNS = 1.1.1.1, 1.0.0.1
MTU = 1320
Jc = 4

[Peer]
PublicKey = DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 1.2.3.4:51820
`

func TestEncodeDecodeRoundTrip(t *testing.T) {
	uri, err := EncodeConf(sampleConf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uri, "vpn://") {
		t.Fatalf("prefix: %s", uri[:20])
	}
	if strings.Contains(uri, "=") {
		t.Fatalf("unexpected padding in %s", uri)
	}
	got, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	conf, err := ConfFromPayload(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(conf) != strings.TrimSpace(sampleConf) {
		t.Fatalf("round-trip mismatch:\n got %q\nwant %q", conf, sampleConf)
	}
}

func TestEncodeConf_OfficialContainer(t *testing.T) {
	uri, err := EncodeConf("# alice\n" + sampleConf)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("payload must be JSON: %v\n%s", err, raw)
	}
	if env["defaultContainer"] != containerName {
		t.Fatalf("defaultContainer = %v", env["defaultContainer"])
	}
	if env["hostName"] != "1.2.3.4" {
		t.Fatalf("hostName = %v", env["hostName"])
	}
	if env["dns1"] != "1.1.1.1" || env["dns2"] != "1.0.0.1" {
		t.Fatalf("dns = %v / %v", env["dns1"], env["dns2"])
	}
	if env["description"] != "alice" {
		t.Fatalf("description = %v", env["description"])
	}
	containers, _ := env["containers"].([]any)
	if len(containers) != 1 {
		t.Fatalf("containers len = %d", len(containers))
	}
	c0, _ := containers[0].(map[string]any)
	if c0["container"] != containerName {
		t.Fatalf("container = %v", c0["container"])
	}
	awg, _ := c0["awg"].(map[string]any)
	if awg == nil {
		t.Fatal("missing awg key (NekoBox+ requires it)")
	}
	lc, _ := awg["last_config"].(string)
	if lc == "" {
		t.Fatal("last_config must be a JSON string")
	}
	var inner map[string]any
	if err := json.Unmarshal([]byte(lc), &inner); err != nil {
		t.Fatalf("last_config not JSON: %v", err)
	}
	cfg, _ := inner["config"].(string)
	if !strings.Contains(cfg, "[Interface]") || !strings.Contains(cfg, "Endpoint = 1.2.3.4:51820") {
		t.Fatalf("inner config missing .conf:\n%s", cfg)
	}
	if inner["client_priv_key"] != "CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=" {
		t.Fatalf("client_priv_key = %v", inner["client_priv_key"])
	}
	if inner["client_ip"] != "10.200.0.2/32" {
		t.Fatalf("client_ip = %v", inner["client_ip"])
	}
	if inner["server_pub_key"] != "DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=" {
		t.Fatalf("server_pub_key = %v", inner["server_pub_key"])
	}
	if inner["Jc"] != "4" {
		t.Fatalf("Jc = %v", inner["Jc"])
	}
	if inner["mtu"] != "1320" {
		t.Fatalf("mtu = %v", inner["mtu"])
	}
	port, _ := inner["port"].(float64)
	if port != 51820 {
		t.Fatalf("port = %v", inner["port"])
	}
	ips, _ := inner["allowed_ips"].([]any)
	if len(ips) != 2 || ips[0] != "0.0.0.0/0" || ips[1] != "::/0" {
		t.Fatalf("allowed_ips = %v", inner["allowed_ips"])
	}
}

func TestEncodeConf_DomainEndpoint(t *testing.T) {
	conf := strings.Replace(sampleConf, "1.2.3.4:51820", "vpn.example.com:51820", 1)
	uri, err := EncodeConf(conf)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	if env["hostName"] != "vpn.example.com" {
		t.Fatalf("envelope hostName = %v", env["hostName"])
	}
	containers, _ := env["containers"].([]any)
	c0, _ := containers[0].(map[string]any)
	awg, _ := c0["awg"].(map[string]any)
	var inner map[string]any
	if err := json.Unmarshal([]byte(awg["last_config"].(string)), &inner); err != nil {
		t.Fatal(err)
	}
	if inner["hostName"] != "vpn.example.com" {
		t.Fatalf("last_config hostName = %v", inner["hostName"])
	}
}

func TestEncodeConf_OmitsEmptyAwgKeys(t *testing.T) {
	uri, err := EncodeConf(sampleConf)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	containers, _ := env["containers"].([]any)
	c0, _ := containers[0].(map[string]any)
	awg, _ := c0["awg"].(map[string]any)
	var inner map[string]any
	if err := json.Unmarshal([]byte(awg["last_config"].(string)), &inner); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"S1", "H1", "I1", "HeaderProtectionKey", "psk_key", "persistent_keep_alive"} {
		if _, ok := inner[k]; ok {
			t.Fatalf("empty key %s must be omitted, got %v", k, inner[k])
		}
	}
}

func TestConfFromPayload_LegacyRawConf(t *testing.T) {
	got, err := ConfFromPayload([]byte(sampleConf))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "[Peer]") {
		t.Fatalf("got %q", got)
	}
}

func TestEncodeConf_RejectsEmpty(t *testing.T) {
	if _, err := EncodeConf(""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := EncodeConf("not a conf"); err == nil {
		t.Fatal("expected error for missing sections")
	}
}

func envelopeAwg(t *testing.T, conf string) map[string]any {
	t.Helper()
	uri, err := EncodeConf(conf)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	containers, _ := env["containers"].([]any)
	c0, _ := containers[0].(map[string]any)
	awg, _ := c0["awg"].(map[string]any)
	if awg == nil {
		t.Fatal("missing awg key")
	}
	return awg
}

func TestEncodeConf_ProtocolVersion(t *testing.T) {
	if pv, ok := envelopeAwg(t, sampleConf)["protocol_version"]; ok {
		t.Fatalf("v1 config must omit protocol_version, got %v", pv)
	}

	v2 := strings.Replace(sampleConf, "Jc = 4", "Jc = 4\nS3 = 10\nS4 = 5", 1)
	if got := envelopeAwg(t, v2)["protocol_version"]; got != "2" {
		t.Fatalf("S3/S4 config must carry protocol_version 2, got %v", got)
	}

	v3 := strings.Replace(sampleConf, "Jc = 4",
		"Jc = 4\nHeaderProtectionKey = MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=", 1)
	if got := envelopeAwg(t, v3)["protocol_version"]; got != "3" {
		t.Fatalf("HPK config must carry protocol_version 3, got %v", got)
	}
}
