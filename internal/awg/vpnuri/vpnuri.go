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
// last_config is a JSON string matching Amnezia AwgClientConfig (config plus
// client_priv_key / server_pub_key / Jc / …). processAmneziaConfig skips
// parsing when last_config is already JSON; without the structured keys the
// app imports the container but the handshake dies immediately.
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
	meta := parseConf(conf)
	innerJSON, _ := json.Marshal(meta.inner)
	awg := map[string]any{
		"last_config":        string(innerJSON),
		"isThirdPartyConfig": true,
		"transport_proto":    "udp",
	}
	if meta.port > 0 {
		awg["port"] = strconv.Itoa(meta.port)
	}
	// AmneziaVPN picks the client generation off protocol_version; without it
	// older app builds fall back to v1 obfuscation and the AWG3 handshake dies.
	if pv := protocolVersion(meta.inner); pv != "" {
		awg["protocol_version"] = pv
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
	if meta.host != "" {
		env["hostName"] = meta.host
	}
	if meta.desc != "" {
		env["description"] = meta.desc
	}
	if meta.dns1 != "" {
		env["dns1"] = meta.dns1
	}
	if meta.dns2 != "" {
		env["dns2"] = meta.dns2
	}
	return env
}

// awgLastConfigKeys are Amnezia last_config field names that match .conf keys
// 1:1 (extractWireGuardConfig copies them as-is).
var awgLastConfigKeys = map[string]string{
	"jc":                     "Jc",
	"jmin":                   "Jmin",
	"jmax":                   "Jmax",
	"s1":                     "S1",
	"s2":                     "S2",
	"s3":                     "S3",
	"s4":                     "S4",
	"h1":                     "H1",
	"h2":                     "H2",
	"h3":                     "H3",
	"h4":                     "H4",
	"i1":                     "I1",
	"i2":                     "I2",
	"i3":                     "I3",
	"i4":                     "I4",
	"i5":                     "I5",
	"headerprotectionkey":    "HeaderProtectionKey",
	"contentpaddingaddition": "ContentPaddingAddition",
	"rekeyaftertime":         "RekeyAfterTime",
	"rekeytimeout":           "RekeyTimeout",
	"rejectaftertime":        "RejectAfterTime",
	"keepalivetimeout":       "KeepaliveTimeout",
	"maxhandshakeattempts":   "MaxHandshakeAttempts",
	"randomtrailers":         "RandomTrailers",
	"disablecookies":         "DisableCookies",
}

type confMeta struct {
	host, dns1, dns2, desc string
	port                   int
	inner                  map[string]any
}

func parseConf(conf string) confMeta {
	meta := confMeta{inner: map[string]any{"config": conf}}
	for _, raw := range strings.Split(conf, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if meta.desc == "" {
				meta.desc = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			}
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "endpoint":
			if meta.host == "" {
				meta.host, meta.port = splitHostPort(val)
				if meta.host != "" {
					meta.inner["hostName"] = meta.host
				}
				if meta.port > 0 {
					meta.inner["port"] = meta.port
				}
			}
		case "dns":
			parts := strings.Split(val, ",")
			if meta.dns1 == "" && len(parts) > 0 {
				meta.dns1 = strings.TrimSpace(parts[0])
			}
			if meta.dns2 == "" && len(parts) > 1 {
				meta.dns2 = strings.TrimSpace(parts[1])
			}
		case "privatekey":
			meta.inner["client_priv_key"] = val
		case "address":
			meta.inner["client_ip"] = val
		case "publickey":
			meta.inner["server_pub_key"] = val
		case "presharedkey":
			meta.inner["psk_key"] = val
		case "mtu":
			meta.inner["mtu"] = val
		case "persistentkeepalive":
			meta.inner["persistent_keep_alive"] = val
		case "allowedips":
			var ips []string
			for _, p := range strings.Split(val, ",") {
				if ip := strings.TrimSpace(p); ip != "" {
					ips = append(ips, ip)
				}
			}
			if len(ips) > 0 {
				meta.inner["allowed_ips"] = ips
			}
		default:
			if jsonKey, ok := awgLastConfigKeys[strings.ToLower(key)]; ok {
				meta.inner[jsonKey] = val
			}
		}
	}
	return meta
}

// awg3ProtocolVersionKeys mark an AWG 3.0 config: any of them present forces
// protocol_version "3" in the envelope.
var awg3ProtocolVersionKeys = []string{
	"HeaderProtectionKey",
	"ContentPaddingAddition",
	"RekeyAfterTime",
	"RekeyTimeout",
	"RejectAfterTime",
	"KeepaliveTimeout",
	"MaxHandshakeAttempts",
}

func protocolVersion(inner map[string]any) string {
	for _, key := range awg3ProtocolVersionKeys {
		if s, ok := inner[key].(string); ok && strings.TrimSpace(s) != "" {
			return "3"
		}
	}
	if atoiPositive(inner, "S3") || atoiPositive(inner, "S4") {
		return "2"
	}
	for _, key := range []string{"H1", "H2", "H3", "H4"} {
		if s, ok := inner[key].(string); ok && strings.Contains(s, "-") {
			return "2"
		}
	}
	if s, ok := inner["I1"].(string); ok && strings.TrimSpace(s) != "" {
		return "2"
	}
	return ""
}

func atoiPositive(inner map[string]any, key string) bool {
	s, ok := inner[key].(string)
	if !ok {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil && n > 0
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
