// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAivpnDefaultSubnetStable(t *testing.T) {
	for id := 1; id <= 260; id++ {
		subnet := AivpnDefaultSubnet(id)
		again := AivpnDefaultSubnet(id)
		if subnet != again {
			t.Fatalf("subnet for id %d is not stable", id)
		}
		// Must remain a valid IPv4 /24 away from the other panel TUN ranges.
		for _, reserved := range []string{"10.254.", "10.68.", "10.66."} {
			if strings.Contains(subnet, reserved) {
				t.Fatalf("id %d collides with the reserved range %s: %s", id, reserved, subnet)
			}
		}
		check := AivpnConfig{Subnet: subnet}
		if _, _, err := check.serverVPNSuffix(); err != nil {
			t.Fatalf("derived subnet invalid: %v", err)
		}
	}
}

func TestAivpnRenderConfig(t *testing.T) {
	cfg := AivpnConfig{
		ListenHost: "0.0.0.0",
		Subnet:     "10.21.0.1/24",
		ServerIP:   "203.0.113.7",
	}
	raw := cfg.RenderConfig(AivpnKey, 45001, "10.21.0.1/24")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if parsed["listen_addr"] != "0.0.0.0:45001" {
		t.Fatalf("listen_addr = %v", parsed["listen_addr"])
	}
	if parsed["server_ip"] != "203.0.113.7" {
		t.Fatalf("server_ip = %v", parsed["server_ip"])
	}
	network, ok := parsed["network_config"].(map[string]any)
	if !ok {
		t.Fatalf("network_config missing")
	}
	if network["server_vpn_ip"] != "10.21.0.1" {
		t.Fatalf("server_vpn_ip = %v", network["server_vpn_ip"])
	}
	if network["prefix_len"] != float64(24) {
		t.Fatalf("prefix_len = %v", network["prefix_len"])
	}
	paths := []string{"key_file", "clients_db", "audit_log", "management_socket"}
	for _, p := range paths {
		if s, ok := parsed[p].(string); !ok || s == "" {
			t.Fatalf("%s missing/empty: %v", p, parsed[p])
		}
	}
	// IPv6 disabled by default: no ipv6_prefix key, ipv6_enabled false.
	if network["ipv6_enabled"] != false {
		t.Fatalf("ipv6_enabled default = %v", network["ipv6_enabled"])
	}
	if _, ok := network["ipv6_prefix"]; ok {
		t.Fatalf("ipv6_prefix must be omitted when IPv6 is off")
	}
}

func TestAivpnKeyFileWrittenOnce(t *testing.T) {
	dir := t.TempDir()
	prev := tunnelDir
	tunnelDir = func() string { return dir }
	defer func() { tunnelDir = prev }()

	if err := EnsureAivpnKeyFile(AivpnKey); err != nil {
		t.Fatalf("mint: %v", err)
	}
	keyPath := filepath.Join(dir, "aivpn-data", "server.key")
	first, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if len(first) != 32 {
		t.Fatalf("server key must be 32 bytes, got %d", len(first))
	}
	// Second call must keep the same key (Rule 0: never rotate silently).
	if err := EnsureAivpnKeyFile(AivpnKey); err != nil {
		t.Fatalf("re-ensure: %v", err)
	}
	second, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("re-read key: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("server key was rewritten by EnsureKeyFile")
	}
}

func TestAivpnInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Remark:   "aivpn test",
		Port:     45001,
		Protocol: model.Aivpn,
		Enable:   false,
	}
	inst, ok := AivpnInstanceFromInbound(ib)
	if !ok {
		t.Fatalf("expected an instance")
	}
	if inst.Enabled {
		t.Fatalf("disabled inbound must not spawn")
	}
	ib.Enable = true
	inst, ok = AivpnInstanceFromInbound(ib)
	if !ok || !inst.Enabled {
		t.Fatalf("expected an enabled instance")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(inst.ConfigText), &cfg); err != nil {
		t.Fatalf("instance config not JSON: %v", err)
	}
	if cfg["listen_addr"] != "0.0.0.0:45001" {
		t.Fatalf("listen_addr = %v", cfg["listen_addr"])
	}
	// Args must point the core at the rendered config file.
	if len(inst.Args) != 2 || inst.Args[0] != "--config" {
		t.Fatalf("args = %v", inst.Args)
	}
	if _, ok := AivpnConfigFromInbound(ib); !ok {
		t.Fatalf("config parse failed")
	}
	// Fingerprint must differ between two different ports (restart on change).
	other := *ib
	other.Port = 45002
	otherInst, _ := AivpnInstanceFromInbound(&other)
	if otherInst.Fingerprint() == inst.Fingerprint() {
		t.Fatalf("fingerprint must change with the port")
	}
}
