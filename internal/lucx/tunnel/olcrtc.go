// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// OlcrtcConfig is the operator-facing configuration of the olcRTC core
// (openlibrecommunity/olcrtc). The panel renders it into a server YAML that
// the binary takes as its sole CLI argument. Field names and validation
// mirror Bebrik2283555/Ex3-ui's extras integration and the upstream
// docs/examples/server/*.yaml schema.
type OlcrtcConfig struct {
	Remark  string `json:"remark"`
	Enabled bool   `json:"enabled"`

	// Provider is the meet auth backend: jitsi | telemost | wbstream.
	Provider string `json:"provider"`
	// RoomID is the full room URL (jitsi) or room identifier (telemost/wb).
	RoomID string `json:"roomId"`
	// CryptoKey is 64 hex chars (32 bytes), shared with the client. Empty
	// on save is auto-generated.
	CryptoKey string `json:"cryptoKey"`
	// Transport: datachannel (jitsi recommended) | vp8channel (telemost/wb).
	Transport string `json:"transport"`
	// DNS is the resolver in host:port form (olcrtc dials it directly).
	DNS string `json:"dns"`

	VP8Fps   int  `json:"vp8Fps"`
	VP8Batch int  `json:"vp8Batch"`
	Debug    bool `json:"debug"`

	// RouteThroughXray: dial egress (and transport HTTP) via SOCKS bridge in
	// Xray (native socks: YAML). Default false — Telemost/Jitsi need direct
	// path; SOCKS also cannot carry ICMP so client "ping" always fails when on.
	RouteThroughXray bool   `json:"routeThroughXray"`
	OutboundTag      string `json:"outboundTag"`
	// RouteXrayPort is backend-owned loopback SOCKS port (0 when not routed).
	RouteXrayPort int `json:"routeXrayPort"`

	MigratedToInbound bool `json:"migratedToInbound,omitempty"`
	MigratedInboundId int  `json:"migratedInboundId,omitempty"`
}

// DefaultOlcrtcConfig returns sensible defaults for a fresh olcRTC core.
func DefaultOlcrtcConfig() OlcrtcConfig {
	return OlcrtcConfig{
		Provider:  "jitsi",
		Transport: "datachannel",
		DNS:       "8.8.8.8:53",
		VP8Fps:    60,
		VP8Batch:  64,
	}
}

// Merge fills zero fields of c from the defaults.
func (c OlcrtcConfig) Merge() OlcrtcConfig {
	def := DefaultOlcrtcConfig()
	if c.Provider == "" {
		c.Provider = def.Provider
	}
	if c.Transport == "" {
		c.Transport = def.Transport
	}
	if c.DNS == "" {
		c.DNS = def.DNS
	}
	if c.VP8Fps == 0 {
		c.VP8Fps = def.VP8Fps
	}
	if c.VP8Batch == 0 {
		c.VP8Batch = def.VP8Batch
	}
	return c
}

// Validate checks the config for internal consistency against the upstream
// olcrtc binary and the olcbox Android client capabilities.
func (c OlcrtcConfig) Validate() error {
	if strings.TrimSpace(c.RoomID) == "" {
		return errors.New("olcrtc: room id is required")
	}
	switch c.Provider {
	case "jitsi", "telemost", "wbstream":
	default:
		return errors.New("olcrtc: provider must be jitsi | telemost | wbstream")
	}
	switch c.Transport {
	case "datachannel", "vp8channel":
	default:
		return errors.New("olcrtc: transport must be datachannel | vp8channel")
	}
	if c.Provider == "telemost" && c.Transport != "vp8channel" {
		return errors.New("olcrtc: Telemost supports only the vp8channel transport")
	}
	if strings.TrimSpace(c.DNS) == "" {
		return errors.New("olcrtc: dns must be set (host:port)")
	}
	if _, _, err := net.SplitHostPort(c.DNS); err != nil {
		return errors.New("olcrtc: dns must be in host:port form (e.g. 8.8.8.8:53)")
	}
	if key := strings.TrimSpace(c.CryptoKey); key != "" && !validCryptoKey(key) {
		return errors.New("olcrtc: crypto key must be 64 hex chars (openssl rand -hex 32)")
	}
	return nil
}

// EnsureCryptoKey returns c with a generated key when empty. Call after
// Validate on save so the URI is always complete.
func (c OlcrtcConfig) EnsureCryptoKey() (OlcrtcConfig, error) {
	if strings.TrimSpace(c.CryptoKey) != "" {
		return c, nil
	}
	key, err := GenerateCryptoKey()
	if err != nil {
		return c, err
	}
	c.CryptoKey = key
	return c, nil
}

// ClampVP8 bounds vp8 fps/batch to the ranges the client sanitizes.
func (c OlcrtcConfig) ClampVP8() OlcrtcConfig {
	if c.Transport != "vp8channel" {
		return c
	}
	c.VP8Fps = clampInt(c.VP8Fps, 1, 120)
	c.VP8Batch = clampInt(c.VP8Batch, 1, 64)
	return c
}

// RenderYAML produces the server YAML the olcrtc binary expects as its sole
// CLI argument. Schema mirrors upstream docs/examples/server/*.yaml and
// Ex3-ui's ServerYAML().
func (c OlcrtcConfig) RenderYAML(dataDir string) string {
	var b strings.Builder
	b.WriteString("mode: srv\n")
	b.WriteString("auth:\n")
	b.WriteString("  provider: " + yamlString(c.Provider) + "\n")
	b.WriteString("room:\n")
	b.WriteString("  id: " + yamlString(c.RoomID) + "\n")
	if key := strings.TrimSpace(c.CryptoKey); key != "" {
		b.WriteString("crypto:\n")
		b.WriteString("  key: " + yamlString(key) + "\n")
	}
	b.WriteString("net:\n")
	b.WriteString("  transport: " + yamlString(c.Transport) + "\n")
	if dns := strings.TrimSpace(c.DNS); dns != "" {
		b.WriteString("  dns: " + yamlString(dns) + "\n")
	}
	if c.Transport == "vp8channel" {
		b.WriteString("vp8:\n")
		b.WriteString("  fps: " + strconv.Itoa(c.VP8Fps) + "\n")
		b.WriteString("  batch_size: " + strconv.Itoa(c.VP8Batch) + "\n")
	}
	// Native SOCKS upstream (upstream olcrtc YAML). When routed, point at the
	// panel-injected Xray loopback SOCKS bridge (noauth).
	if c.RouteThroughXray && c.RouteXrayPort > 0 {
		b.WriteString("socks:\n")
		b.WriteString("  proxy_addr: \"127.0.0.1\"\n")
		b.WriteString("  proxy_port: " + strconv.Itoa(c.RouteXrayPort) + "\n")
		b.WriteString("  proxy_user: \"\"\n")
		b.WriteString("  proxy_pass: \"\"\n")
	}
	b.WriteString("liveness:\n")
	b.WriteString("  interval: 10s\n")
	b.WriteString("  timeout: 5s\n")
	b.WriteString("  failures: 3\n")
	if dataDir != "" {
		b.WriteString("data: " + yamlString(dataDir) + "\n")
	}
	b.WriteString("debug: " + strconv.FormatBool(c.Debug) + "\n")
	return b.String()
}

// ClientURI renders the olcrtc:// connect string imported by olcbox /
// owenclave (grammar from openlibrecommunity/olcrtc docs/uri.md and
// Ex3-ui OlcrtcURI).
func (c OlcrtcConfig) ClientURI() string {
	room := strings.TrimSpace(c.RoomID)
	key := strings.TrimSpace(c.CryptoKey)
	if room == "" || key == "" {
		return ""
	}
	transport := strings.TrimSpace(c.Transport)
	switch transport {
	case "vp8channel":
		transport = fmt.Sprintf("vp8channel<vp8-fps=%d&vp8-batch=%d>", c.VP8Fps, c.VP8Batch)
	case "":
		transport = "datachannel"
	}
	return fmt.Sprintf("olcrtc://%s?%s@%s#%s",
		strings.TrimSpace(c.Provider), transport, room, key)
}

// GenerateCryptoKey returns 32 random bytes as a 64-char hex string.
func GenerateCryptoKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func validCryptoKey(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		isDigit := r >= '0' && r <= '9'
		isLower := r >= 'a' && r <= 'f'
		isUpper := r >= 'A' && r <= 'F'
		if !isDigit && !isLower && !isUpper {
			return false
		}
	}
	return true
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// yamlString double-quotes a scalar and escapes embedded quotes/newlines so
// operator input cannot break out of its string token.
func yamlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return "\"" + s + "\""
}
