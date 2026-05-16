// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telegram

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// tg://proxy link regex: server IP, port, ee+hex secret
var tgProxyRegex = regexp.MustCompile(`^tg://proxy\?server=[0-9a-zA-Z\.\-]+&port=[0-9]+&secret=ee[a-f0-9]+$`)

func TestExtractTelemtSecret_FromPassword(t *testing.T) {
	client := model.Client{
		Email:    "test@client",
		Password: "ee00000000000000000000000000000000",
	}
	inbound := model.Inbound{Port: 443, Listen: "5.9.1.2"}
	secret := extractTelemtSecret(client, inbound)
	if secret != "ee00000000000000000000000000000000" {
		t.Errorf("expected ee-secret from password, got %s", secret)
	}
}

func TestExtractTelemtSecret_FromSettings(t *testing.T) {
	client := model.Client{Email: "test@client", Password: ""}
	inbound := model.Inbound{
		Port:   443,
		Listen: "5.9.1.2",
		Settings: `{"clients":[{"email":"test@client","secret":"ee11111111111111111111111111111111"}]}`,
	}
	secret := extractTelemtSecret(client, inbound)
	if secret != "ee11111111111111111111111111111111" {
		t.Errorf("expected ee-secret from settings, got %s", secret)
	}
}

func TestTelemtLinkFormat(t *testing.T) {
	// Build the proxy link as the bot would
	serverIP := "5.9.1.2"
	port := 443
	secret := "ee00000000000000000000000000000000"
	link := fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", serverIP, port, secret)

	if !tgProxyRegex.MatchString(link) {
		t.Errorf("link does not match tg://proxy regex: %s", link)
	}

	// Verify URL encoding didn't break ? & =
	if link[0:10] != "tg://proxy" {
		t.Errorf("link scheme missing or URL-encoded: %s", link)
	}
}

func TestTelemtLinkNoDoubleEscape(t *testing.T) {
	// Simulate what happens when the link is used in Telegram inline button URL
	serverIP := "34.88.118.168"
	port := 443
	secret := "eebbaa11223344556677889900aabbccddeeff"
	link := fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", serverIP, port, secret)

	if link != "tg://proxy?server=34.88.118.168&port=443&secret=eebbaa11223344556677889900aabbccddeeff" {
		t.Errorf("unexpected link: %s", link)
	}
}

func TestGetServerIP(t *testing.T) {
	inbound := model.Inbound{Listen: "10.0.0.1", Port: 443}
	ip := GetServerIP(inbound)
	if ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", ip)
	}

	inbound2 := model.Inbound{Listen: "0.0.0.0", Port: 443}
	ip2 := GetServerIP(inbound2)
	if ip2 != "YOUR_SERVER_IP" {
		t.Errorf("expected YOUR_SERVER_IP for 0.0.0.0, got %s", ip2)
	}
}
