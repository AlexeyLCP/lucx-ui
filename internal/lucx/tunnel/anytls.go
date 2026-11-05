// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AnytlsConfig is one anytls-server instance (anytls/anytls-go reference
// implementation plus LucX -cert/-key overlay): a TLS proxy over the AnyTLS
// protocol (splits the outer TLS handshake to dodge TLS-in-TLS fingerprints)
// with a shared password per server port, plus (overlay -users-file) one
// derived password per attached client so traffic, online state and client
// IPs are per user. Clients: sing-box, mihomo,
// Shadowrocket, Stash, Loon. URI scheme: anytls://password@host:port/?sni=.
type AnytlsConfig struct {
	Remark   string `json:"remark"`
	Enabled  bool   `json:"enabled"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	SNI      string `json:"sni"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
	// Fallback is a loopback host:port (e.g. a Cover's local site) that gets
	// every connection failing AnyTLS authentication — browsers and probes see
	// that site instead of a dropped TLS connection. Needs the LucX overlay.
	Fallback string `json:"fallback,omitempty"`
	Bind     string `json:"-"`
	// usersFile / statsFile are the overlay's -users-file and -stats paths,
	// set by AnytlsInstanceFromInbound when the binary supports them.
	usersFile string
	statsFile string

	// RouteThroughXray redirects the sidecar's own outbound TCP (uid-scoped,
	// lucx-mtproxy pattern, lucx.273) into a hidden Xray SOCKS inbound so
	// Xray routing rules apply. anytls-go has no SOCKS option, so the bridge
	// is iptables REDIRECT (tproxy pattern), not a dialer setting.
	RouteThroughXray bool   `json:"routeThroughXray"`
	RouteXrayPort    int    `json:"routeXrayPort"`
	OutboundTag      string `json:"outboundTag"`
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
	if fb := strings.TrimSpace(c.Fallback); fb != "" {
		host, port, err := net.SplitHostPort(fb)
		if err != nil || port == "" || (host != "127.0.0.1" && host != "::1") {
			return errors.New("anytls: fallback must be 127.0.0.1:port")
		}
	}
	return nil
}

// ListenAddr is the anytls-server -l value.
func (c AnytlsConfig) ListenAddr() string {
	host := strings.TrimSpace(c.Bind)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, strconv.Itoa(c.Port))
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
var (
	anytlsPWFileOnce sync.Once
	anytlsPWFileOK   bool
	anytlsFallbackOK bool
	anytlsUsersOK    bool
	anytlsProxyOK    bool
)

func anytlsProbeHelp() {
	anytlsPWFileOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		b, _ := exec.CommandContext(ctx, Anytls.BinaryPath(), "-h").CombinedOutput()
		out := string(b)
		anytlsPWFileOK = strings.Contains(out, "password-file")
		anytlsFallbackOK = strings.Contains(out, "-fallback")
		anytlsUsersOK = strings.Contains(out, "-users-file") && strings.Contains(out, "-stats")
		anytlsProxyOK = strings.Contains(out, "-proxy-protocol")
	})
}

func anytlsSupportsPasswordFile() bool {
	anytlsProbeHelp()
	return anytlsPWFileOK
}

// anytlsSupportsFallback: the binary is the LucX overlay with -fallback.
func anytlsSupportsFallback() bool {
	anytlsProbeHelp()
	return anytlsFallbackOK
}

// AnytlsSupportsUsers: the binary is the LucX overlay with -users-file and
// -stats, so every client gets its own password.
func AnytlsSupportsUsers() bool {
	anytlsProbeHelp()
	return anytlsUsersOK
}

// AnytlsSupportsProxyProtocol: the binary reads the SNI gateway's PROXY v1
// header (from loopback peers only), so it sees the real client IP.
func AnytlsSupportsProxyProtocol() bool {
	anytlsProbeHelp()
	return anytlsProxyOK
}

// SetAnytlsCapsForTest pretends the AnyTLS binary does (or doesn't) support
// per-user passwords and PROXY v1; restore undoes it. Tests only.
func SetAnytlsCapsForTest(users, proxy bool) (restore func()) {
	anytlsProbeHelp()
	u, p := anytlsUsersOK, anytlsProxyOK
	anytlsUsersOK, anytlsProxyOK = users, proxy
	return func() { anytlsUsersOK, anytlsProxyOK = u, p }
}

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
	if fb := strings.TrimSpace(c.Fallback); fb != "" && anytlsSupportsFallback() {
		args = append(args, "-fallback", fb)
	}
	if c.usersFile != "" && c.statsFile != "" {
		args = append(args, "-users-file", c.usersFile, "-stats", c.statsFile)
	}
	if AnytlsSupportsProxyProtocol() {
		args = append(args, "-proxy-protocol")
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

// redirectOwnerUID resolves the uid the routed anytls child shares with the
// REDIRECT egress bridge (lucx-mtproxy user, lucx.273); zero = no bridge user
// on this host — egress stays direct. The Linux build wires the real lookup.
var redirectOwnerUID = func() int { return 0 }

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
