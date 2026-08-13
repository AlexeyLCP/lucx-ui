// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// TrustTunnelConfig is the operator-facing configuration of one TrustTunnel
// inbound, rendered into the endpoint TOML files (vpn/hosts/credentials/rules).
// Client credentials are not stored: they are HMAC-derived per panel client at
// render time (naive/mieru model).
type TrustTunnelConfig struct {
	Remark    string `json:"remark"`
	Enabled   bool   `json:"enabled"`
	Hostname  string `json:"hostname"`
	Listen    string `json:"listen"`
	IPv6      bool   `json:"ipv6"`
	CertFile  string `json:"certFile"`
	KeyFile   string `json:"keyFile"`
	ClientDNS string `json:"clientDns"`

	// UpstreamProtocol is advertised in the client deep link: http2 | http3.
	UpstreamProtocol string `json:"upstreamProtocol"`

	// RouteThroughXray makes the endpoint forward tunneled traffic through a
	// hidden loopback SOCKS bridge the panel injects into Xray
	// ([forward_protocol.socks5]). Default false — direct forwarding.
	RouteThroughXray bool   `json:"routeThroughXray"`
	OutboundTag      string `json:"outboundTag"`
	// RouteXrayPort is the backend-owned loopback SOCKS port (0 when not routed).
	RouteXrayPort int `json:"routeXrayPort"`
	// MetricsPort is the backend-owned loopback Prometheus port for traffic
	// accounting (0 = metrics disabled).
	MetricsPort int `json:"metricsPort"`
}

// DefaultTrustTunnelConfig returns sensible defaults for a fresh inbound.
func DefaultTrustTunnelConfig() TrustTunnelConfig {
	return TrustTunnelConfig{
		Listen:           "0.0.0.0:443",
		IPv6:             true,
		UpstreamProtocol: "http2",
	}
}

// Merge fills zero fields of c from the defaults.
func (c TrustTunnelConfig) Merge() TrustTunnelConfig {
	def := DefaultTrustTunnelConfig()
	if strings.TrimSpace(c.Listen) == "" {
		c.Listen = def.Listen
	}
	if strings.TrimSpace(c.UpstreamProtocol) == "" {
		c.UpstreamProtocol = def.UpstreamProtocol
	}
	return c
}

// Validate checks the config for internal consistency. certOK reports whether
// the certificate files were resolved and verified for the hostname — the
// endpoint cannot start without a trusted cert, so an invalid cert keeps the
// instance down (and the save-time hook rejects it with a message).
func (c TrustTunnelConfig) Validate() error {
	if strings.TrimSpace(c.Hostname) == "" {
		return fmt.Errorf("trusttunnel: hostname (domain) is required")
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return fmt.Errorf("trusttunnel: listen must be host:port")
	}
	switch strings.ToLower(strings.TrimSpace(c.UpstreamProtocol)) {
	case "http2", "http3":
	default:
		return fmt.Errorf("trusttunnel: upstream protocol must be http2 | http3")
	}
	return nil
}

// ResolveCertPaths returns the effective cert/key paths: explicit config
// values win over the panel ACME cert pair.
func (c TrustTunnelConfig) ResolveCertPaths(panelCert, panelKey string) (cert, key string) {
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

// ValidateCertFiles verifies the cert/key pair exists, parses, covers the
// hostname in its SANs and is not expired.
func ValidateCertFiles(certFile, keyFile, hostname string) error {
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("trusttunnel: no TLS certificate — issue a domain certificate (x-ui console menu → SSL) or provide cert/key paths")
	}
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("trusttunnel: read certificate %s: %w", certFile, err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		return fmt.Errorf("trusttunnel: read key %s: %w", keyFile, err)
	}
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		return fmt.Errorf("trusttunnel: certificate/key pair invalid: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("trusttunnel: certificate %s has no PEM block", certFile)
	}
	x, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("trusttunnel: parse certificate: %w", err)
	}
	now := time.Now()
	if now.After(x.NotAfter) {
		return fmt.Errorf("trusttunnel: certificate expired %s — renew it (x-ui console menu → SSL)", x.NotAfter.Format(time.RFC3339))
	}
	if now.Before(x.NotBefore) {
		return fmt.Errorf("trusttunnel: certificate not valid until %s", x.NotBefore.Format(time.RFC3339))
	}
	host := strings.TrimSpace(hostname)
	if !certCoversHost(x, host) {
		return fmt.Errorf("trusttunnel: certificate does not cover domain %q (SAN: %s)", host, strings.Join(x.DNSNames, ", "))
	}
	return nil
}

// CertFileHash returns sha256 of the cert file content (fingerprint material
// so ACME renewal restarts the endpoint), or "" when unreadable.
func CertFileHash(certFile string) string {
	b, err := os.ReadFile(certFile)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func certCoversHost(x *x509.Certificate, host string) bool {
	if x.VerifyHostname(host) == nil {
		return true
	}
	for _, n := range x.DNSNames {
		if strings.EqualFold(n, host) {
			return true
		}
	}
	return false
}

func tomlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return "\"" + s + "\""
}

// RenderVpnToml produces the main endpoint settings file. credsPath/rulesPath
// point at the companion ExtraFiles; socksAddr (127.0.0.1:port) switches
// forwarding through the panel bridge; metricsAddr enables the Prometheus
// listener used by the traffic scraper.
func (c TrustTunnelConfig) RenderVpnToml(credsPath, rulesPath, socksAddr, metricsAddr string) string {
	var b strings.Builder
	b.WriteString("listen_address = " + tomlString(c.Listen) + "\n")
	b.WriteString("ipv6_available = " + strconv.FormatBool(c.IPv6) + "\n")
	b.WriteString("allow_private_network_connections = false\n")
	b.WriteString("credentials_file = " + tomlString(credsPath) + "\n")
	b.WriteString("rules_file = " + tomlString(rulesPath) + "\n")
	b.WriteString("\n[listen_protocols.http1]\n")
	b.WriteString("\n[listen_protocols.http2]\n")
	b.WriteString("\n[listen_protocols.quic]\n")
	b.WriteString("\n[forward_protocol]\n")
	if socksAddr != "" {
		b.WriteString("[forward_protocol.socks5]\n")
		b.WriteString("address = " + tomlString(socksAddr) + "\n")
	} else {
		b.WriteString("direct = {}\n")
	}
	if metricsAddr != "" {
		b.WriteString("\n[metrics]\n")
		b.WriteString("address = " + tomlString(metricsAddr) + "\n")
		b.WriteString("request_timeout_secs = 3\n")
	}
	return b.String()
}

// RenderHostsToml produces the TLS hosts file (one main host).
func (c TrustTunnelConfig) RenderHostsToml(certPath, keyPath string) string {
	var b strings.Builder
	b.WriteString("[[main_hosts]]\n")
	b.WriteString("hostname = " + tomlString(strings.TrimSpace(c.Hostname)) + "\n")
	b.WriteString("cert_chain_path = " + tomlString(certPath) + "\n")
	b.WriteString("private_key_path = " + tomlString(keyPath) + "\n")
	return b.String()
}

// RenderCredentialsToml produces the client credentials file.
func RenderTrustTunnelCredentials(users []AuthPair) string {
	var b strings.Builder
	for _, u := range users {
		if strings.TrimSpace(u.User) == "" {
			continue
		}
		b.WriteString("[[client]]\n")
		b.WriteString("username = " + tomlString(u.User) + "\n")
		b.WriteString("password = " + tomlString(u.Pass) + "\n")
	}
	return b.String()
}

// --- tt:// deep link encoder (TrustTunnel DEEP_LINK.md v1) -----------------

// tlsVarint encodes v as an RFC 9000 §16 variable-length integer.
func tlsVarint(v uint64) []byte {
	switch {
	case v < 1<<6:
		return []byte{byte(v)}
	case v < 1<<14:
		return []byte{byte(v>>8) | 0x40, byte(v)}
	case v < 1<<30:
		return []byte{byte(v>>24) | 0x80, byte(v >> 16), byte(v >> 8), byte(v)}
	default:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, v)
		b[0] |= 0xc0
		return b
	}
}

func ttTLV(tag byte, value []byte) []byte {
	out := make([]byte, 0, 1+2+len(value))
	out = append(out, tag)
	out = append(out, tlsVarint(uint64(len(value)))...)
	return append(out, value...)
}

// ClientDeepLink builds the tt://?base64url(TLV) deep link for one client.
// address is the endpoint host[:port] the client dials; the LE cert is
// omitted (publicly trusted). Fields at their default value are skipped.
func (c TrustTunnelConfig) ClientDeepLink(address string, pair AuthPair, remark string) string {
	address = strings.TrimSpace(address)
	if address == "" || strings.TrimSpace(pair.User) == "" {
		return ""
	}
	var payload []byte
	payload = append(payload, ttTLV(0x00, tlsVarint(1))...) // version 1
	payload = append(payload, ttTLV(0x01, []byte(strings.TrimSpace(c.Hostname)))...)
	payload = append(payload, ttTLV(0x02, []byte(address))...)
	if !c.IPv6 { // default true — omit when true
		payload = append(payload, ttTLV(0x04, []byte{0x00})...)
	}
	payload = append(payload, ttTLV(0x05, []byte(pair.User))...)
	payload = append(payload, ttTLV(0x06, []byte(pair.Pass))...)
	if strings.EqualFold(strings.TrimSpace(c.UpstreamProtocol), "http3") { // default http2 — omit
		payload = append(payload, ttTLV(0x09, tlsVarint(2))...)
	}
	if r := strings.TrimSpace(remark); r != "" {
		payload = append(payload, ttTLV(0x0C, []byte(r))...)
	}
	var dns []string
	for _, d := range strings.Split(c.ClientDNS, ",") {
		if d = strings.TrimSpace(d); d != "" {
			dns = append(dns, d)
		}
	}
	if len(dns) > 0 {
		var list []byte
		for _, d := range dns {
			list = append(list, tlsVarint(uint64(len(d)))...)
			list = append(list, []byte(d)...)
		}
		payload = append(payload, ttTLV(0x0D, list)...)
	}
	return "tt://?" + base64.RawURLEncoding.EncodeToString(payload)
}

// ListenPort extracts the port from the listen address (0 on parse failure).
func (c TrustTunnelConfig) ListenPort() int {
	if _, p, err := net.SplitHostPort(strings.TrimSpace(c.Listen)); err == nil {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	return 0
}
