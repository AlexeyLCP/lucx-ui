// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
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

	// ListenPreset selects documented listen_protocols windows:
	// "stock" = TrustTunnel CONFIGURATION.md / settings.rs defaults
	// (128 KiB stream window — slow upload); "fast" = tester-tuned
	// windows that match download. Empty merges to "fast".
	ListenPreset string `json:"listenPreset"`

	// ClientRandomPrefix is the TLS Client Random filter (hex/mask, e.g.
	// "77394630/77fb7fb5"). Written to rules.toml as an allow rule and into
	// the client tt:// deep link (TLV 0x0B). Empty → generated on Merge when
	// the inbound is first saved (normalize path).
	ClientRandomPrefix string `json:"clientRandomPrefix"`
}

const (
	trustTunnelPresetStock = "stock"
	trustTunnelPresetFast  = "fast"

	stockHTTP2StreamWindow = 128 << 10
	stockHTTP2ConnWindow   = 8 << 20
	stockHTTP1UploadBuffer = 32 << 10

	fastHTTP2StreamWindow = 4 << 20
	fastHTTP2ConnWindow   = 64 << 20
	fastHTTP1UploadBuffer = 512 << 10

	defaultHTTP2MaxStreams  = 1000
	defaultHTTP2MaxFrame    = 16384
	defaultHTTP2HeaderTable = 65536
)

type trustTunnelListenTune struct {
	streamWindow, connWindow, uploadBuffer int
}

func listenTuneFor(preset string) trustTunnelListenTune {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case trustTunnelPresetStock:
		return trustTunnelListenTune{
			streamWindow: stockHTTP2StreamWindow,
			connWindow:   stockHTTP2ConnWindow,
			uploadBuffer: stockHTTP1UploadBuffer,
		}
	default:
		return trustTunnelListenTune{
			streamWindow: fastHTTP2StreamWindow,
			connWindow:   fastHTTP2ConnWindow,
			uploadBuffer: fastHTTP1UploadBuffer,
		}
	}
}

// DefaultTrustTunnelConfig returns sensible defaults for a fresh inbound.
func DefaultTrustTunnelConfig() TrustTunnelConfig {
	return TrustTunnelConfig{
		Listen:           "0.0.0.0:443",
		IPv6:             true,
		UpstreamProtocol: "http2",
		ListenPreset:     trustTunnelPresetFast,
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
	switch strings.ToLower(strings.TrimSpace(c.ListenPreset)) {
	case trustTunnelPresetStock:
		c.ListenPreset = trustTunnelPresetStock
	default:
		c.ListenPreset = trustTunnelPresetFast
	}
	if p := strings.TrimSpace(c.ClientRandomPrefix); p != "" {
		c.ClientRandomPrefix = p
	}
	return c
}

// GenerateClientRandomPrefix returns a TrustTunnel bitwise prefix/mask pair
// ("aabbccdd/ffffffff", 4 random bytes + full mask) suitable for rules.toml
// and the client deep-link TLV 0x0B.
func GenerateClientRandomPrefix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:]) + "/" + strings.Repeat("f", len(b)*2)
}

// ValidClientRandomPrefix reports whether s is a non-empty hex prefix or
// prefix/mask pair accepted by TrustTunnel rules.
func ValidClientRandomPrefix(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	prefix, mask, hasMask := strings.Cut(s, "/")
	if !isHexBytes(prefix) || len(prefix) < 2 || len(prefix)%2 != 0 {
		return false
	}
	if hasMask {
		if !isHexBytes(mask) || len(mask) == 0 || len(mask)%2 != 0 {
			return false
		}
	}
	return true
}

func isHexBytes(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// RenderRulesToml builds rules.toml. A non-empty client_random_prefix becomes
// an allow rule (first match wins; default is allow-all when no rules match).
func RenderTrustTunnelRules(clientRandomPrefix string) string {
	p := strings.TrimSpace(clientRandomPrefix)
	if p == "" || !ValidClientRandomPrefix(p) {
		return ""
	}
	var b strings.Builder
	b.WriteString("[[rule]]\n")
	b.WriteString("client_random_prefix = " + tomlString(p) + "\n")
	b.WriteString("action = \"allow\"\n")
	return b.String()
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
	switch strings.ToLower(strings.TrimSpace(c.ListenPreset)) {
	case "", trustTunnelPresetStock, trustTunnelPresetFast:
	default:
		return fmt.Errorf("trusttunnel: listen preset must be stock | fast")
	}
	if p := strings.TrimSpace(c.ClientRandomPrefix); p != "" && !ValidClientRandomPrefix(p) {
		return fmt.Errorf("trusttunnel: client random prefix must be hex or hex/mask")
	}
	return nil
}

// EnsureClientRandomPrefix fills an empty prefix with a freshly generated one.
func (c *TrustTunnelConfig) EnsureClientRandomPrefix() {
	if c == nil {
		return
	}
	if strings.TrimSpace(c.ClientRandomPrefix) == "" {
		c.ClientRandomPrefix = GenerateClientRandomPrefix()
	}
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
	return validatePEMCert("trusttunnel", certFile, keyFile, hostname)
}

func validatePEMCert(label, certFile, keyFile, hostname string) error {
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("%s: no TLS certificate — issue a domain certificate (x-ui console menu → SSL) or provide cert/key paths", label)
	}
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("%s: read certificate %s: %w", label, certFile, err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		return fmt.Errorf("%s: read key %s: %w", label, keyFile, err)
	}
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		return fmt.Errorf("%s: certificate/key pair invalid: %w", label, err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("%s: certificate %s has no PEM block", label, certFile)
	}
	x, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("%s: parse certificate: %w", label, err)
	}
	now := time.Now()
	if now.After(x.NotAfter) {
		return fmt.Errorf("%s: certificate expired %s — renew it (x-ui console menu → SSL)", label, x.NotAfter.Format(time.RFC3339))
	}
	if now.Before(x.NotBefore) {
		return fmt.Errorf("%s: certificate not valid until %s", label, x.NotBefore.Format(time.RFC3339))
	}
	host := strings.TrimSpace(hostname)
	if !certCoversHost(x, host) {
		return fmt.Errorf("%s: certificate does not cover domain %q (SAN: %s)", label, host, strings.Join(x.DNSNames, ", "))
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
	tune := listenTuneFor(c.ListenPreset)
	b.WriteString("\n[listen_protocols.http1]\n")
	b.WriteString("upload_buffer_size = " + strconv.Itoa(tune.uploadBuffer) + "\n")
	b.WriteString("\n[listen_protocols.http2]\n")
	b.WriteString("initial_stream_window_size = " + strconv.Itoa(tune.streamWindow) + "\n")
	b.WriteString("initial_connection_window_size = " + strconv.Itoa(tune.connWindow) + "\n")
	b.WriteString("max_concurrent_streams = " + strconv.Itoa(defaultHTTP2MaxStreams) + "\n")
	b.WriteString("max_frame_size = " + strconv.Itoa(defaultHTTP2MaxFrame) + "\n")
	b.WriteString("header_table_size = " + strconv.Itoa(defaultHTTP2HeaderTable) + "\n")
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
	if p := strings.TrimSpace(c.ClientRandomPrefix); ValidClientRandomPrefix(p) {
		payload = append(payload, ttTLV(0x0B, []byte(p))...)
	}
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

// ClientURI builds the Throne-compatible tt://user:pass@host:port share
// link. Official TrustTunnel apps parse ClientDeepLink (TLV); Throne does
// not and needs this URI form (tester repro lucx.139). A configured
// client_random_prefix is appended as a query param (lucx.142). The value
// is hex or hex/mask — leave `/` unescaped (lucx.145): NekoBox+ does not
// URL-decode `%2F` and drops the prefix.
func (c TrustTunnelConfig) ClientURI(address string, pair AuthPair, remark string) string {
	address = strings.TrimSpace(address)
	user := strings.TrimSpace(pair.User)
	if address == "" || user == "" {
		return ""
	}
	alpn := "h2"
	if strings.EqualFold(strings.TrimSpace(c.UpstreamProtocol), "http3") {
		alpn = "h3"
	}
	sni := strings.TrimSpace(c.Hostname)
	var q []string
	q = append(q, "security=tls")
	if sni != "" {
		q = append(q, "sni="+url.QueryEscape(sni))
	}
	q = append(q, "alpn="+alpn)
	if p := strings.TrimSpace(c.ClientRandomPrefix); ValidClientRandomPrefix(p) {
		q = append(q, "client_random_prefix="+p)
	}
	u := url.URL{
		Scheme:   "tt",
		User:     url.UserPassword(user, pair.Pass),
		Host:     address,
		RawQuery: strings.Join(q, "&"),
		Fragment: strings.TrimSpace(remark),
	}
	return u.String()
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
