// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// AnytlsConfig is one anytls-server instance (anytls/anytls-go reference
// implementation): a TLS proxy over the AnyTLS protocol (splits the outer
// TLS handshake to dodge TLS-in-TLS fingerprints) with a single shared
// password per server port. Clients: sing-box, mihomo, Shadowrocket,
// Stash, Loon. URI scheme: anytls://password@host:port.
type AnytlsConfig struct {
	Remark   string `json:"remark"`
	Enabled  bool   `json:"enabled"`
	Port     int    `json:"port"`
	Password string `json:"password"`
}

// DefaultAnytlsConfig returns factory defaults (port 8443, as in the
// anytls-go examples).
func DefaultAnytlsConfig() AnytlsConfig {
	return AnytlsConfig{Port: 8443}
}

// Merge fills zero fields with defaults.
func (c AnytlsConfig) Merge() AnytlsConfig {
	if c.Port <= 0 {
		c.Port = 8443
	}
	return c
}

// Validate checks the runnable config (password enforced: anytls-server
// refuses to start without -p).
func (c AnytlsConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("anytls: port must be in 1..65535")
	}
	if strings.TrimSpace(c.Password) == "" {
		return errors.New("anytls: password is required")
	}
	return nil
}

// ListenAddr is the anytls-server -l value.
func (c AnytlsConfig) ListenAddr() string {
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(c.Port))
}

// BuildArgs renders the argv for anytls-server (flags only).
func (c AnytlsConfig) BuildArgs() []string {
	return []string{"-l", c.ListenAddr(), "-p", strings.TrimSpace(c.Password)}
}

// EnsurePassword returns c with a generated shared password when empty.
func (c AnytlsConfig) EnsurePassword() (AnytlsConfig, error) {
	if strings.TrimSpace(c.Password) != "" {
		return c, nil
	}
	pass, err := generateQwdttPassword(24)
	if err != nil {
		return c, err
	}
	c.Password = pass
	return c, nil
}

// ClientLink renders the anytls:// share URI (anytls-go URI scheme).
// anytls-server ships a self-signed cert, so insecure=1 is required or
// sing-box/mihomo/Shadowrocket refuse the handshake. Password and remark
// are percent-encoded per the scheme.
func (c AnytlsConfig) ClientLink(host, remark string) string {
	host = strings.TrimSpace(host)
	pass := strings.TrimSpace(c.Password)
	if host == "" || pass == "" {
		return ""
	}
	u := url.URL{
		Scheme:   "anytls",
		User:     url.User(pass),
		Host:     net.JoinHostPort(host, strconv.Itoa(c.Port)),
		Path:     "/",
		RawQuery: "insecure=1",
	}
	if r := strings.TrimSpace(remark); r != "" {
		u.Fragment = r
	}
	return u.String()
}
