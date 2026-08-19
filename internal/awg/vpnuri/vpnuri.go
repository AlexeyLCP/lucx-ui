// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package vpnuri encodes AmneziaVPN vpn:// share links from a WireGuard/AWG
// client .conf. Format matches amnezia-client ExportController:
//
//	vpn:// + Base64URL( qCompress(JSON) )
//
// where qCompress is Qt's wrapper: 4-byte big-endian uncompressed length +
// raw DEFLATE (zlib) body. The JSON is the official Amnezia container
// (containers[].awg.last_config) so NekoBox+ and awg-manager parse it.
// last_config is a JSON string whose "config" field is the awg-quick .conf
// (Amnezia's processAmneziaConfig overwrites last_config if it is raw INI).
package vpnuri

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	containerName = "amnezia-awg"
	protoName     = "awg"
)

// EncodeConf builds a vpn:// URI from an awg-quick client .conf body.
func EncodeConf(conf string) (string, error) {
	conf = strings.TrimSpace(conf)
	if conf == "" {
		return "", fmt.Errorf("vpnuri: empty conf")
	}
	if !strings.Contains(conf, "[Interface]") || !strings.Contains(conf, "[Peer]") {
		return "", fmt.Errorf("vpnuri: conf missing [Interface]/[Peer]")
	}
	payload, err := json.Marshal(amneziaEnvelope(conf))
	if err != nil {
		return "", err
	}
	compressed, err := qCompress(payload)
	if err != nil {
		return "", err
	}
	return "vpn://" + base64.RawURLEncoding.EncodeToString(compressed), nil
}

func amneziaEnvelope(conf string) map[string]any {
	host, port, dns1, dns2, desc := peekConfMeta(conf)
	inner, _ := json.Marshal(map[string]any{
		"config":   conf,
		"hostName": host,
		"port":     port,
	})
	awg := map[string]any{
		"last_config":        string(inner),
		"isThirdPartyConfig": true,
		"transport_proto":    "udp",
	}
	if port > 0 {
		awg["port"] = strconv.Itoa(port)
	}
	env := map[string]any{
		"defaultContainer": containerName,
		"containers": []any{
			map[string]any{
				"container": containerName,
				protoName:   awg,
			},
		},
	}
	if host != "" {
		env["hostName"] = host
	}
	if desc != "" {
		env["description"] = desc
	}
	if dns1 != "" {
		env["dns1"] = dns1
	}
	if dns2 != "" {
		env["dns2"] = dns2
	}
	return env
}

func peekConfMeta(conf string) (host string, port int, dns1, dns2, desc string) {
	for _, raw := range strings.Split(conf, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if desc == "" {
				desc = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "endpoint":
			h, p := splitHostPort(strings.TrimSpace(val))
			if host == "" {
				host, port = h, p
			}
		case "dns":
			parts := strings.Split(val, ",")
			if dns1 == "" && len(parts) > 0 {
				dns1 = strings.TrimSpace(parts[0])
			}
			if dns2 == "" && len(parts) > 1 {
				dns2 = strings.TrimSpace(parts[1])
			}
		}
	}
	return host, port, dns1, dns2, desc
}

func splitHostPort(s string) (string, int) {
	if strings.HasPrefix(s, "[") {
		end := strings.LastIndex(s, "]")
		if end < 0 {
			return s, 0
		}
		host := s[1:end]
		rest := s[end+1:]
		if !strings.HasPrefix(rest, ":") {
			return host, 0
		}
		p, err := strconv.Atoi(rest[1:])
		if err != nil {
			return host, 0
		}
		return host, p
	}
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, 0
	}
	p, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return s, 0
	}
	return s[:i], p
}

func qCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	buf.Write(size[:])
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode returns uncompressed payload bytes (JSON envelope or legacy raw .conf).
func Decode(uri string) ([]byte, error) {
	s := strings.TrimSpace(uri)
	s = strings.TrimPrefix(s, "vpn://")
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
	}
	if len(raw) < 4 {
		return nil, fmt.Errorf("vpnuri: truncated payload")
	}
	r, err := zlib.NewReader(bytes.NewReader(raw[4:]))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ConfFromPayload extracts the awg-quick .conf from a Decode() result.
// Accepts the official JSON container and the legacy raw-conf payload.
func ConfFromPayload(payload []byte) (string, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed != "" && trimmed[0] != '{' && strings.Contains(trimmed, "[Interface]") && strings.Contains(trimmed, "[Peer]") {
		return trimmed, nil
	}
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return "", fmt.Errorf("vpnuri: not JSON and not .conf")
	}
	containers, _ := env["containers"].([]any)
	if len(containers) == 0 {
		return "", fmt.Errorf("vpnuri: no containers")
	}
	last, _ := containers[len(containers)-1].(map[string]any)
	if last == nil {
		return "", fmt.Errorf("vpnuri: bad container")
	}
	proto, _ := last["awg"].(map[string]any)
	if proto == nil {
		proto, _ = last["wireguard"].(map[string]any)
	}
	if proto == nil {
		return "", fmt.Errorf("vpnuri: no awg/wireguard")
	}
	switch lc := proto["last_config"].(type) {
	case string:
		var inner map[string]any
		if err := json.Unmarshal([]byte(lc), &inner); err == nil {
			if c, ok := inner["config"].(string); ok && strings.Contains(c, "[Interface]") {
				return strings.TrimSpace(c), nil
			}
		}
		if strings.Contains(lc, "[Interface]") {
			return strings.TrimSpace(lc), nil
		}
	case map[string]any:
		if c, ok := lc["config"].(string); ok && strings.Contains(c, "[Interface]") {
			return strings.TrimSpace(c), nil
		}
	}
	return "", fmt.Errorf("vpnuri: last_config has no .conf")
}
