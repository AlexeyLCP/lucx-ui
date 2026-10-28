// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

// MasterDNSVPN sidecar (issue #127): masterking32/MasterDnsVPN Go server.
// The core is an authoritative-DNS tunnel server: the tunnel domain's NS
// record must delegate to this host, DNS queries carry the encrypted tunnel,
// and the decoded egress goes through a loopback SOCKS bridge into Xray
// (routeThroughXray) or straight out of the host in direct mode. Operator
// supplies the delegated domain; the panel renders the server TOML, the
// client TOML and the resolver list for the MasterDnsVPN client app.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	masterdnsMinSecretChars = 32
	// masterdnsDirectClientPort is the local SOCKS port the panel suggests in
	// the client TOML (upstream sample default).
	masterdnsDirectClientPort = 18000
)

// MasterdnsConfig is the operator-facing configuration of one MasterDNSVPN
// inbound. Port/UDP bind = inbound.Port.
type MasterdnsConfig struct {
	Remark  string `json:"remark"`
	Enabled bool   `json:"enabled"`
	// Domains is the delegated tunnel domain list (comma/space separated).
	Domains string `json:"domains"`
	// Secret is the shared ENCRYPTION_KEY (hex). Must equal the client key.
	Secret string `json:"secret"`
	// Encryption is DATA_ENCRYPTION_METHOD: 0 None, 1 XOR, 2 ChaCha20,
	// 3 AES-128-GCM, 4 AES-192-GCM, 5 AES-256-GCM. Default 2.
	Encryption int `json:"encryption"`

	// RouteThroughXray sends the decoded egress into Xray's router via the
	// loopback SOCKS bridge; when off the core dials destinations directly.
	RouteThroughXray bool   `json:"routeThroughXray"`
	OutboundTag      string `json:"outboundTag"`
	ConfigDir        string `json:"configDir"`

	// Resolvers is the resolver list shipped in the client package (one
	// host:port per line, comma/space separated). Empty → public defaults.
	Resolvers string `json:"resolvers"`

	// RouteXrayPort is backend-owned loopback SOCKS port (0 when not routed).
	RouteXrayPort int `json:"routeXrayPort"`
}

func DefaultMasterdnsConfig() MasterdnsConfig {
	return MasterdnsConfig{Encryption: 2}
}

func (c MasterdnsConfig) Merge() MasterdnsConfig {
	c.Domains = strings.TrimSpace(c.Domains)
	c.Secret = strings.TrimSpace(c.Secret)
	if c.Encryption < 0 || c.Encryption > 5 {
		c.Encryption = 2
	}
	c.Resolvers = strings.TrimSpace(c.Resolvers)
	return c
}

func (c MasterdnsConfig) Validate(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("masterdns: port must be 1-65535")
	}
	if err := validateMasterdnsDomains(c.Domains); err != nil {
		return err
	}
	if c.Encryption > 0 && openfluxSecretChars(c.Secret) < masterdnsMinSecretChars {
		return fmt.Errorf("masterdns: secret must be at least %d hex chars", masterdnsMinSecretChars)
	}
	return nil
}

func validateMasterdnsDomains(raw string) error {
	list := masterdnsDomainList(raw)
	if len(list) == 0 {
		return errors.New("masterdns: at least one delegated tunnel domain is required")
	}
	for _, d := range list {
		if len(d) > 253 {
			return errors.New("masterdns: domain too long")
		}
		if !masterdnsDomainOK(d) {
			return errors.New("masterdns: tunnel domain must be a DNS name (e.g. v.example.com)")
		}
	}
	return nil
}

// masterdnsDomainOK checks one hostname label-by-label (RFC-ish, no wildcards).
func masterdnsDomainOK(domain string) bool {
	last := byte('.')
	domain = strings.TrimSuffix(domain, ".")
	for i := 0; i < len(domain); i++ {
		ch := domain[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9', ch == '-':
			if ch == '-' && last == '.' {
				return false
			}
		case ch == '.':
			if last == '.' || last == '-' {
				return false
			}
		case ch >= 'A' && ch <= 'Z':
			// tolerated, normalized downstream
		default:
			return false
		}
		last = ch
	}
	// non-empty, no leading/trailing dot/hyphen, has a TLD part
	return len(domain) > 0 && domain[0] != '.' && domain[0] != '-' &&
		domain[len(domain)-1] != '-' && strings.Count(domain, ".") >= 1
}

func masterdnsDomainSep(r rune) bool {
	return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t'
}

// masterdnsDomainList trims per-domain pieces, lowercases and dedups.
func masterdnsDomainList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, p := range strings.FieldsFunc(raw, masterdnsDomainSep) {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (c MasterdnsConfig) EnsureSecret() (MasterdnsConfig, error) {
	if strings.TrimSpace(c.Secret) != "" {
		return c, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return c, err
	}
	c.Secret = hex.EncodeToString(buf)
	return c, nil
}

// ResolveConfigDir mirrors csqtt: empty → the core data dir; a supplied path
// must stay inside the panels work dir.
func (c MasterdnsConfig) ResolveConfigDir() string {
	fallback := dataDirFor(MasterdnsKey, Masterdns)
	d := strings.TrimSpace(c.ConfigDir)
	if d == "" {
		return fallback
	}
	abs, err := filepath.Abs(d)
	if err != nil {
		return fallback
	}
	root, err := filepath.Abs(workDir())
	if err != nil {
		return fallback
	}
	sep := string(filepath.Separator)
	if abs == root || strings.HasPrefix(abs, root+sep) {
		return abs
	}
	return fallback
}

func (c MasterdnsConfig) secretPath(key string) string {
	return absPath(filepath.Join(workDir(), key+"-secret.txt"))
}

func (c MasterdnsConfig) confPath(key string) string {
	return absPath(filepath.Join(workDir(), key+".toml"))
}

// RenderServerConf is the exit-side TOML. Egress is a SOCKS5 server that
// either routes everything through the upstream SOCKS (the Xray loopback
// bridge) or dials destinations directly.
func (c MasterdnsConfig) RenderServerConf(key string, port int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# rendered by LucX-UI for %s\n", key)
	fmt.Fprintf(&b, "DOMAIN = [")
	for i, d := range masterdnsDomainList(c.Domains) {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", d)
	}
	b.WriteString("]\n")
	fmt.Fprintf(&b, "PROTOCOL_TYPE = \"SOCKS5\"\n")
	fmt.Fprintf(&b, "DATA_ENCRYPTION_METHOD = %d\n", c.Encryption)
	fmt.Fprintf(&b, "ENCRYPTION_KEY_FILE = %q\n", c.secretPath(key))
	fmt.Fprintf(&b, "UDP_PORT = %d\n", port)
	if c.RouteThroughXray && c.RouteXrayPort > 0 {
		fmt.Fprintf(&b, "USE_EXTERNAL_SOCKS5 = true\n")
		fmt.Fprintf(&b, "FORWARD_IP = \"127.0.0.1\"\n")
		fmt.Fprintf(&b, "FORWARD_PORT = %d\n", c.RouteXrayPort)
	} else {
		fmt.Fprintf(&b, "USE_EXTERNAL_SOCKS5 = false\n")
	}
	fmt.Fprintf(&b, "SOCKS_CONNECT_TIMEOUT = 60.0\n")
	fmt.Fprintf(&b, "LOG_LEVEL = \"INFO\"\n")
	return b.String()
}

// RenderClientTOML is the client_config.toml the MasterDnsVPN app imports.
func (c MasterdnsConfig) RenderClientTOML() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# MasterDnsVPN client config — rendered by LucX-UI\n")
	fmt.Fprintf(&b, "DOMAINS = [")
	for i, d := range masterdnsDomainList(c.Domains) {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", d)
	}
	b.WriteString("]\n")
	fmt.Fprintf(&b, "DATA_ENCRYPTION_METHOD = %d\n", c.Encryption)
	fmt.Fprintf(&b, "ENCRYPTION_KEY = %q\n", strings.TrimSpace(c.Secret))
	fmt.Fprintf(&b, "PROTOCOL_TYPE = \"SOCKS5\"\n")
	fmt.Fprintf(&b, "LISTEN_IP = \"127.0.0.1\"\n")
	fmt.Fprintf(&b, "LISTEN_PORT = %d\n", masterdnsDirectClientPort)
	fmt.Fprintf(&b, "RESOLVERS_FILE_PATH = \"client_resolvers.txt\"\n")
	return b.String()
}

// RenderClientResolvers is the resolver list for the client package. The
// operator may prepend ISP resolvers for lower latency; empty → public
// defaults.
func (c MasterdnsConfig) RenderClientResolvers() string {
	if strings.TrimSpace(c.Resolvers) != "" {
		var b strings.Builder
		for _, r := range strings.FieldsFunc(c.Resolvers, masterdnsDomainSep) {
			b.WriteString(strings.TrimSpace(r))
			b.WriteString("\n")
		}
		return b.String()
	}
	return "8.8.8.8:53\n1.1.1.1:53\n9.9.9.9:53\n"
}

// syncMasterdnsFiles writes the server TOML and the secret key file when
// either changed (stamp compare, like syncCsqttPasswordStamp). The work dir
// is created first — on a fresh panel it does not exist yet.
func syncMasterdnsFiles(cfg MasterdnsConfig, key string, port int) error {
	if err := os.MkdirAll(workDir(), 0o755); err != nil {
		return err
	}
	conf := cfg.confPath(key)
	if err := os.WriteFile(conf, []byte(cfg.RenderServerConf(key, port)), 0o600); err != nil {
		return err
	}
	secret := cfg.secretPath(key)
	prev, err := os.ReadFile(secret)
	if err == nil && strings.TrimSpace(string(prev)) == strings.TrimSpace(cfg.Secret) {
		return nil
	}
	if err := os.WriteFile(secret, []byte(strings.TrimSpace(cfg.Secret)), 0o600); err != nil {
		return err
	}
	return nil
}

func (c MasterdnsConfig) BuildArgs(key string) []string {
	return []string{"-config", c.confPath(key)}
}

func (c MasterdnsConfig) String() string {
	return fmt.Sprintf("masterdns domain=%s", c.Domains)
}
