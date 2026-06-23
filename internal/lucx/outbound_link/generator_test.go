// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package outbound_link

import (
	"encoding/json"
	"testing"
)

func TestGenerateOutbound_VLESS(t *testing.T) {
	settings := `{"clients":[{"id":"test-uuid-1234","flow":"xtls-rprx-vision","email":"client1"}],"decryption":"none","fallbacks":[]}`
	streamSettings := `{"network":"tcp","security":"reality","realitySettings":{"serverName":"yahoo.com","publicKey":"pubkey123","shortId":"abc123"}}`

	result, err := GenerateOutbound(
		"vless",
		"vless-in",
		443,
		settings,
		streamSettings,
		"5.9.1.2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Tag != "5.9.1.2-vless-out" {
		t.Errorf("expected tag '5.9.1.2-vless-out', got '%s'", result.Tag)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(result.OutboundJSON, &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if out["protocol"] != "vless" {
		t.Errorf("expected protocol 'vless', got '%v'", out["protocol"])
	}

	settingsMap := out["settings"].(map[string]interface{})
	vnext := settingsMap["vnext"].([]interface{})[0].(map[string]interface{})
	if vnext["address"] != "5.9.1.2" {
		t.Errorf("expected address '5.9.1.2', got '%v'", vnext["address"])
	}
	if int(vnext["port"].(float64)) != 443 {
		t.Errorf("expected port 443, got %v", vnext["port"])
	}
}

func TestGenerateOutbound_RejectAWG(t *testing.T) {
	_, err := GenerateOutbound("awg", "awg-in", 12345, "{}", "{}", "1.2.3.4")
	if err == nil {
		t.Fatal("expected error for AWG protocol")
	}
	if err.Error() != "protocol 'awg' does not support outbound linking" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateOutbound_RejectTelemt(t *testing.T) {
	_, err := GenerateOutbound("telemt", "mt-in", 12345, "{}", "{}", "1.2.3.4")
	if err == nil {
		t.Fatal("expected error for telemt protocol")
	}
}

func TestGenerateOutbound_NoClients(t *testing.T) {
	settings := `{"clients":[],"decryption":"none"}`
	_, err := GenerateOutbound("vless", "vless-in", 443, settings, "{}", "1.2.3.4")
	if err == nil {
		t.Fatal("expected error for no clients")
	}
}
