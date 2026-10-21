// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func ImportConfig(text string, tproxyPort int) (*model.Inbound, error) {
	if len(text) > 1024*1024 {
		return nil, fmt.Errorf("WireGuard import exceeds 1 MiB")
	}
	settings := map[string]any{"kernel": true, "tproxyPort": tproxyPort, "clients": []map[string]any{}}
	section := ""
	port := 0
	var peers []map[string]any
	var peer map[string]any
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		if line == "[Interface]" {
			if section != "" {
				return nil, fmt.Errorf("duplicate WireGuard interface section")
			}
			section = "interface"
			continue
		}
		if line == "[Peer]" {
			if section == "" {
				return nil, fmt.Errorf("WireGuard interface section must come first")
			}
			section = "peer"
			peer = map[string]any{"email": fmt.Sprintf("wg-peer-%d", len(peers)+1), "enable": true}
			peers = append(peers, peer)
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section == "" {
			return nil, fmt.Errorf("invalid WireGuard configuration line")
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if section == "interface" {
			switch key {
			case "PrivateKey":
				settings["secretKey"] = value
			case "Address":
				settings["address"] = value
			case "ListenPort":
				n, e := strconv.Atoi(value)
				if e != nil {
					return nil, fmt.Errorf("invalid WireGuard listen port")
				}
				port = n
			case "MTU":
				n, e := strconv.Atoi(value)
				if e != nil {
					return nil, fmt.Errorf("invalid WireGuard MTU")
				}
				settings["mtu"] = n
			case "DNS":
				settings["dns"] = value
			case "Table":
				if value != "off" {
					return nil, fmt.Errorf("review and remove imported Table routing before kernel WireGuard import")
				}
			default:
				return nil, fmt.Errorf("unsupported WireGuard interface field %q; remove hooks and obfuscation before importing", key)
			}
		} else {
			switch key {
			case "PublicKey":
				peer["publicKey"] = value
				sum := sha256.Sum256([]byte(value))
				peer["email"] = fmt.Sprintf("wg-%x", sum[:8])
			case "PresharedKey":
				peer["preSharedKey"] = value
			case "AllowedIPs":
				var addresses []string
				for _, a := range strings.Split(value, ",") {
					addresses = append(addresses, strings.TrimSpace(a))
				}
				peer["allowedIPs"] = addresses
			case "PersistentKeepalive":
				peer["keepAlive"] = value
			default:
				return nil, fmt.Errorf("unsupported WireGuard peer field %q", key)
			}
		}
	}
	if scanner.Err() != nil {
		return nil, fmt.Errorf("unable to read WireGuard config")
	}
	settings["clients"] = peers
	b, _ := json.Marshal(settings)
	ib := &model.Inbound{Id: 1, Protocol: model.WireGuard, Enable: false, Port: port, Settings: string(b)}
	if _, err := Parse(ib); err != nil {
		return nil, err
	}
	ib.Id = 0
	return ib, nil
}
