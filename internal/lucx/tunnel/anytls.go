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
// implementation plus LucX -cert/-key overlay): a TLS proxy over the AnyTLS
// protocol (splits the outer TLS handshake to dodge TLS-in-TLS fingerprints)
// with a single shared password per server port. Clients: sing-box, mihomo,
// Shadowrocket, Stash, Loon. URI scheme: anytls://password@host:port/?sni=.
type AnytlsConfig struct {
	Remark   string `json:"remark"`
	Enabled  bool   `json:"enabled"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	SNI      string `json:"sni"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
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

// Validate checks the runnable config (password and SNI enforced: the
// overlay anytls-server needs -p, and a real cert must cover SNI).
func (c AnytlsConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("anytls: port must be in 1..65535")
	}
	if strings.TrimSpace(c.Password) == "" {
		return errors.New("anytls: password is required")
	}
	if strings.TrimSpace(c.SNI) == "" {
		return errors.New("anytls: sni/domain is required")
	}
	return nil
}

// ListenAddr is the anytls-server -l value.
func (c AnytlsConfig) ListenAddr() string {
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(c.Port))
}

// ResolveCertPaths returns explicit config paths, else the panel ACME pair.
func (c AnytlsConfig) ResolveCertPaths(panelCert, panelKey string) (cert, key string) {
	cert = strings.TrimSpace(c.CertFile)
	if cert == "" {
		cert = strings.TrimSpace(panelCert)
	}
	key = strings.TrimSpace(c.KeyFile)
	if key == "" {
		key = strings.TrimSpace(panelKey)
	}
	return cert, key
}

// ValidateCert checks the resolved certificate covers SNI.
func (c AnytlsConfig) ValidateCert(panelCert, panelKey string) error {
	cert, key := c.ResolveCertPaths(panelCert, panelKey)
	return validatePEMCert("anytls", cert, key, strings.TrimSpace(c.SNI))
}

// BuildArgs renders the argv for the LucX-overlay anytls-server.
func (c AnytlsConfig) BuildArgs(cert, key, passwordFile string) []string {
	args := []string{"-l", c.ListenAddr()}
	if strings.TrimSpace(passwordFile) != "" {
		args = append(args, "-password-file", passwordFile)
	} else {
		args = append(args, "-p", strings.TrimSpace(c.Password))
	}
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		args = append(args, "-cert", cert, "-key", key)
	}
	return args
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
// A trusted cert is required, so the query is sni= (no insecure=1).
func (c AnytlsConfig) ClientLink(host, remark string) string {
	host = strings.TrimSpace(host)
	pass := strings.TrimSpace(c.Password)
	sni := strings.TrimSpace(c.SNI)
	if host == "" || pass == "" || sni == "" {
		return ""
	}
	u := url.URL{
		Scheme:   "anytls",
		User:     url.User(pass),
		Host:     net.JoinHostPort(host, strconv.Itoa(c.Port)),
		Path:     "/",
		RawQuery: "sni=" + url.QueryEscape(sni),
	}
	if r := strings.TrimSpace(remark); r != "" {
		u.Fragment = r
	}
	return u.String()
}
