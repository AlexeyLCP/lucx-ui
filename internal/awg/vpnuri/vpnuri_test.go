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
