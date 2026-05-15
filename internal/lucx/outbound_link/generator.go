// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package outbound_link

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AllowedProtocols lists Xray protocols that support outbound linking.
var AllowedProtocols = map[string]bool{
	"vless":       true,
	"vmess":       true,
	"trojan":      true,
	"hysteria2":   true,
	"shadowsocks": true,
	"wireguard":   true,
	"socks":       true,
	"http":        true,
}

// OutboundResult holds the generated outbound config.
type OutboundResult struct {
	Tag          string          `json:"tag"`
	OutboundJSON json.RawMessage `json:"outboundJson"`
}

// GenerateOutbound creates an outbound config from an inbound's settings.
// It extracts the first client's credentials and creates a vnext/outbound
// pointing to the remote node.
func GenerateOutbound(
	protocol string,
	originalTag string,
	port int,
	settingsJSON string,
	streamSettingsJSON string,
	remoteAddress string,
) (*OutboundResult, error) {
	if !AllowedProtocols[protocol] {
		return nil, fmt.Errorf("protocol '%s' does not support outbound linking", protocol)
	}

	var inboundSettings map[string]interface{}
	if err := json.Unmarshal([]byte(settingsJSON), &inboundSettings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	clientsRaw, ok := inboundSettings["clients"]
	if !ok {
		return nil, fmt.Errorf("inbound has no clients")
	}
	clients, ok := clientsRaw.([]interface{})
	if !ok || len(clients) == 0 {
		return nil, fmt.Errorf("inbound has no clients")
	}

	firstClient := clients[0].(map[string]interface{})

	outboundSettings := buildOutboundSettings(protocol, remoteAddress, port, firstClient)

	sanitizedAddr := strings.ReplaceAll(remoteAddress, ":", "-")
	tag := fmt.Sprintf("%s-%s-out", sanitizedAddr, protocol)

	outbound := map[string]interface{}{
		"protocol": protocol,
		"tag":      tag,
		"settings": outboundSettings,
	}

	if streamSettingsJSON != "" && streamSettingsJSON != "{}" {
		var streamSettings interface{}
		if err := json.Unmarshal([]byte(streamSettingsJSON), &streamSettings); err == nil {
			outbound["streamSettings"] = streamSettings
		}
	}

	outboundBytes, err := json.Marshal(outbound)
	if err != nil {
		return nil, fmt.Errorf("marshal outbound: %w", err)
	}

	return &OutboundResult{
		Tag:          tag,
		OutboundJSON: outboundBytes,
	}, nil
}

func buildOutboundSettings(protocol, address string, port int, client map[string]interface{}) map[string]interface{} {
	user := map[string]interface{}{}
	if id, ok := client["id"]; ok {
		user["id"] = id
	}
	if flow, ok := client["flow"]; ok && flow != "" {
		user["flow"] = flow
	}
	if security, ok := client["security"]; ok {
		user["security"] = security
	}
	if password, ok := client["password"]; ok {
		user["password"] = password
	}
	user["encryption"] = "none"

	switch protocol {
	case "vless", "trojan", "vmess":
		return map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": address,
					"port":    port,
					"users":   []interface{}{user},
				},
			},
		}
	case "shadowsocks":
		return map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"address":  address,
					"port":     port,
					"method":   client["method"],
					"password": client["password"],
				},
			},
		}
	default:
		return map[string]interface{}{
			"address": address,
			"port":    port,
			"users":   []interface{}{user},
		}
	}
}
