// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func awgConfInbound(settings string) *model.Inbound {
	return &model.Inbound{
		Protocol: model.AWG,
		Port:     51820,
		Settings: settings,
	}
}

func awgConfClient(priv string) *model.Client {
	return &model.Client{
		PrivateKey: priv,
		AllowedIPs: []string{"10.200.0.2/32"},
		Enable:     true,
	}
}

func TestBuildAwgClientConf_DerivesServerPubFromPrivate(t *testing.T) {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	settings, _ := json.Marshal(map[string]any{
		"privateKey": priv,
		"publicKey":  pub,
		"mtu":        1420,
		"dns":        "1.1.1.1",
	})
	cpriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}
	conf, err := BuildAwgClientConf(awgConfInbound(string(settings)), awgConfClient(cpriv), "203.0.113.9")
	if err != nil {
		t.Fatalf("BuildAwgClientConf: %v", err)
	}
	if !strings.Contains(conf, "PublicKey = "+pub) {
		t.Errorf("conf must carry the server public key derived from privateKey, got:\n%s", conf)
	}
}

func TestBuildAwgClientConf_FallsBackToStoredPublicKey(t *testing.T) {
	_, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	settings, _ := json.Marshal(map[string]any{
		"privateKey": "not-a-valid-curve25519-key",
		"publicKey":  pub,
		"mtu":        1420,
		"dns":        "1.1.1.1",
	})
	cpriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}
	conf, err := BuildAwgClientConf(awgConfInbound(string(settings)), awgConfClient(cpriv), "203.0.113.9")
	if err != nil {
		t.Fatalf("BuildAwgClientConf must fall back to settings.publicKey when the private key cannot be parsed, got err: %v", err)
	}
	if !strings.Contains(conf, "PublicKey = "+pub) {
		t.Errorf("conf must carry the stored publicKey fallback, got:\n%s", conf)
	}
}

func TestBuildAwgClientConf_PrefersInboundPeerAddress(t *testing.T) {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	cpriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}
	settings, _ := json.Marshal(map[string]any{
		"privateKey": priv,
		"publicKey":  pub,
		"mtu":        1420,
		"dns":        "1.1.1.1",
		"clients": []map[string]any{
			{"email": "demo-user", "allowedIPs": []string{"10.8.0.3/32"}},
		},
	})
	client := &model.Client{
		Email:      "demo-user",
		PrivateKey: cpriv,
		AllowedIPs: []string{"10.201.0.2/32"},
		Enable:     true,
	}
	conf, err := BuildAwgClientConf(awgConfInbound(string(settings)), client, "203.0.113.9")
	if err != nil {
		t.Fatalf("BuildAwgClientConf: %v", err)
	}
	if !strings.Contains(conf, "Address = 10.8.0.3/32") {
		t.Errorf("must use inbound peer address, got:\n%s", conf)
	}
	if strings.Contains(conf, "10.201.0.2") {
		t.Errorf("must not use table address, got:\n%s", conf)
	}
}

func TestBuildAwgClientConf_FallsBackToClientAllowedIPs(t *testing.T) {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	cpriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}
	settings, _ := json.Marshal(map[string]any{
		"privateKey": priv,
		"publicKey":  pub,
		"mtu":        1420,
		"dns":        "1.1.1.1",
	})
	conf, err := BuildAwgClientConf(awgConfInbound(string(settings)), awgConfClient(cpriv), "203.0.113.9")
	if err != nil {
		t.Fatalf("BuildAwgClientConf: %v", err)
	}
	if !strings.Contains(conf, "Address = 10.200.0.2/32") {
		t.Errorf("must fall back to client AllowedIPs, got:\n%s", conf)
	}
}

func TestBuildAwgClientConf_ErrsWhenNoServerKeyRecoverable(t *testing.T) {
	settings, _ := json.Marshal(map[string]any{
		"privateKey": "",
		"publicKey":  "",
		"mtu":        1420,
		"dns":        "1.1.1.1",
	})
	cpriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}
	if _, err := BuildAwgClientConf(awgConfInbound(string(settings)), awgConfClient(cpriv), "203.0.113.9"); err == nil {
		t.Fatal("expected an error when neither privateKey nor publicKey is usable")
	}
}
