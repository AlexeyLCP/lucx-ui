// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// AuthPair is one basic_auth credential pair for the forward_proxy block.
// The panel renders its service-level pair plus one pair per enabled panel
// client (per-client subscription links).
type AuthPair struct {
	User string
	Pass string
}

// ClientAuth deterministically derives a client's NaiveProxy credentials from
// the panel secret and the client email (HMAC-SHA256). Nothing is stored: the
// Caddyfile renderer and the subscription link builder re-derive the same
// pair, so disabling a client drops their basic_auth line on the next
// reconcile without a migration. The username is opaque (no email leakage in
// logs/URLs); the password is 162 bits of HMAC output.
func ClientAuth(secret []byte, email string) AuthPair {
	userMac := hmac.New(sha256.New, secret)
	userMac.Write([]byte("lucx-naive-user:" + email))
	passMac := hmac.New(sha256.New, secret)
	passMac.Write([]byte("lucx-naive-pass:" + email))
	return AuthPair{
		User: "nx" + hex.EncodeToString(userMac.Sum(nil))[:10],
		Pass: base64.RawURLEncoding.EncodeToString(passMac.Sum(nil))[:27],
	}
}

// NaiveConfig is the operator-facing configuration of the NaiveProxy core. It
// is persisted as JSON by the web layer and rendered into a Caddyfile by
// RenderCaddyfile. Field semantics follow the upstream Caddy + forward_proxy
// documentation; the rendering rules encode the pitfalls collected by the
// elector1337/3x-ui-naive project (admin off, bare :port bind normalization,
// isolated ACME storage).
type NaiveConfig struct {
	Remark    string `json:"remark"`
	Enabled   bool   `json:"enabled"`
	Listen    string `json:"listen"`
	Port      int    `json:"port"`
	Domain    string `json:"domain"`
	UseAcme   bool   `json:"useAcme"`
	AcmeEmail string `json:"acmeEmail"`
	CertFile  string `json:"certFile"`
	KeyFile   string `json:"keyFile"`
	AuthUser  string `json:"authUser"`
	AuthPass  string `json:"authPass"`

	EnableH3        bool   `json:"enableH3"`
	ProbeResistance bool   `json:"probeResistance"`
	LogLevel        string `json:"logLevel"`
	ExtraArgs       string `json:"extraArgs"`

	// RouteThroughXray makes caddy dial destinations through a hidden
	// loopback SOCKS bridge the panel injects into Xray (tag
	// NaiveEgressTag). forward_proxy's native `upstream socks5://…`
	// directive does the dial — no patched binary required. Backend-
	// owned RouteXrayPort is allocated on first enable and kept stable
	// across saves; OutboundTag optionally force-routes the bridge.
	// Incompatible with raw Caddyfile mode (operator owns the file).
	RouteThroughXray bool   `json:"routeThroughXray"`
	RouteXrayPort    int    `json:"routeXrayPort"`
	OutboundTag      string `json:"outboundTag"`

	UseRawConfig bool   `json:"useRawConfig"`
	RawConfig    string `json:"rawConfig"`
}

// NaiveEgressTag is the stable Xray inbound tag of the hidden SOCKS
// bridge a routed NaiveProxy core dials. Operators can match it in
// routing rules the same way they match an mtproto inbound tag.
const NaiveEgressTag = "lucx-tunnel-naive"

// DefaultNaiveConfig returns sensible defaults for a fresh NaiveProxy core.
func DefaultNaiveConfig() NaiveConfig {
	return NaiveConfig{
		Port:            443,
		EnableH3:        true,
		ProbeResistance: true,
		LogLevel:        "WARN",
	}
}

// Merge fills zero fields of c from the defaults so a partial JSON document
// stored by an older panel version never wipes a field it does not know.
func (c NaiveConfig) Merge() NaiveConfig {
	def := DefaultNaiveConfig()
	if c.Port == 0 {
		c.Port = def.Port
	}
	if c.LogLevel == "" {
		c.LogLevel = def.LogLevel
	}
	return c
}

// Validate checks the config for internal consistency. Raw mode only
// requires a non-empty Caddyfile — everything else is the operator's
// responsibility there (the panel cannot parse arbitrary Caddyfiles).
func (c NaiveConfig) Validate() error {
	if c.UseRawConfig {
		if strings.TrimSpace(c.RawConfig) == "" {
			return errors.New("naive: raw Caddyfile is empty")
		}
		if c.RouteThroughXray {
			return errors.New("naive: Route through Xray is unavailable in raw Caddyfile mode")
		}
		return nil
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("naive: port must be in 1..65535")
	}
	if strings.TrimSpace(c.AuthUser) == "" || strings.TrimSpace(c.AuthPass) == "" {
		return errors.New("naive: auth user and password are required")
	}
	if c.UseAcme {
		if strings.TrimSpace(c.Domain) == "" {
			return errors.New("naive: Auto TLS requires a domain")
		}
		if c.Port != 443 {
			return errors.New("naive: Auto TLS (Let's Encrypt HTTP-01) requires port 443")
		}
	} else {
		if strings.TrimSpace(c.CertFile) == "" || strings.TrimSpace(c.KeyFile) == "" {
			return errors.New("naive: certificate and key file paths are required (or enable Auto TLS)")
		}
	}
	switch strings.ToUpper(strings.TrimSpace(c.LogLevel)) {
	case "", "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return errors.New("naive: log level must be one of DEBUG | INFO | WARN | ERROR")
	}
	return nil
}

// caddyToken quotes a value for the Caddyfile lexer, escaping embedded
// backslashes, quotes and newlines so operator input cannot break out of the
// token or inject directives.
func caddyToken(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return "\"" + s + "\""
}

// RenderCaddyfile produces the Caddyfile served to the caddy binary.
// extraAuth carries the per-client credential pairs (one basic_auth line
// each) rendered after the service-level pair; nil for raw mode and
// previews. accessLogPath, when non-empty, enables a JSON site access_log
// used by CollectNaiveTraffic for online/traffic (best-effort); empty for
// previews and raw mode.
//
// Rendering rules (hard-won upstream lessons):
//   - admin off: several cores/panels on one host must not fight over the
//     Caddy admin port :2019.
//   - a wildcard listen address renders as a bare ":port" site address —
//     Caddy treats an explicit "0.0.0.0:port" as a host matcher, not a bind;
//     a concrete listen IP becomes a "bind" directive instead.
//   - Auto TLS puts the domain in the site address (Caddy then manages the
//     certificate itself); manual TLS lists cert/key paths in the tls
//     directive and adds the domain as a second site address for SNI.
//   - the HTTP/2 padding protocol has NO Caddyfile subdirective in the naive
//     forward_proxy fork: the server engages it per connection whenever the
//     client sends a Padding header (the naive client always does). E2E
//     caught a rendered `padding` line failing `caddy adapt` (lucx.91).
func (c NaiveConfig) RenderCaddyfile(extraAuth []AuthPair, accessLogPath string) string {
	if c.UseRawConfig {
		return strings.TrimRight(c.RawConfig, "\n") + "\n"
	}

	level := strings.ToUpper(strings.TrimSpace(c.LogLevel))
	if level == "" {
		level = "WARN"
	}

	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("\tadmin off\n")
	// Caddy tries to install its internal CA root into the system trust
	// store whenever a localhost/internal cert appears — pointless for a
	// headless sidecar and noisy in the log.
	b.WriteString("\tskip_install_trust\n")
	if !c.UseAcme {
		// Manual TLS: disable automatic HTTPS entirely, otherwise Caddy
		// starts the ACME challenge/redirect listener on :80 for the
		// hostname site address and dies on "bind: permission denied"
		// when the panel runs unprivileged (caught by E2E, lucx.91).
		// ACME mode keeps auto HTTPS — that is the mechanism Let's
		// Encrypt HTTP-01 uses.
		b.WriteString("\tauto_https off\n")
	}
	b.WriteString("\tlog {\n\t\tlevel " + level + "\n\t}\n")
	if !c.EnableH3 {
		b.WriteString("\tservers {\n\t\tprotocols h1 h2\n\t}\n")
	}
	b.WriteString("}\n\n")

	listen := strings.TrimSpace(c.Listen)
	wildcard := listen == "" || listen == "0.0.0.0" || listen == "::"
	domain := strings.TrimSpace(c.Domain)

	var addrs []string
	if c.UseAcme {
		addrs = append(addrs, domain)
	} else {
		addrs = append(addrs, ":"+strconv.Itoa(c.Port))
		if domain != "" {
			// The domain address must carry the port too: a bare hostname
			// defaults to :443 (automatic HTTPS) and Caddy opens an extra
			// listener there — E2E caught ":443 bind permission denied" on
			// a non-root panel with port 18443 (lucx.91).
			if c.Port == 443 {
				addrs = append(addrs, domain)
			} else {
				addrs = append(addrs, net.JoinHostPort(domain, strconv.Itoa(c.Port)))
			}
		}
	}
	b.WriteString(strings.Join(addrs, ", ") + " {\n")
	if !wildcard {
		b.WriteString("\tbind " + listen + "\n")
	}
	if c.UseAcme {
		if email := strings.TrimSpace(c.AcmeEmail); email != "" {
			b.WriteString("\ttls " + email + "\n")
		}
	} else {
		b.WriteString("\ttls " + caddyToken(strings.TrimSpace(c.CertFile)) + " " +
			caddyToken(strings.TrimSpace(c.KeyFile)) + "\n")
	}
	// JSON access log → CollectNaiveTraffic (online last-seen + best-effort
	// per-client bytes). Path is per-instance under dataDir so multi-inbound
	// hosts never share a log. Omitted for previews (empty path).
	if p := strings.TrimSpace(accessLogPath); p != "" {
		b.WriteString("\tlog {\n")
		b.WriteString("\t\toutput file " + caddyToken(p) + "\n")
		b.WriteString("\t\tformat json\n")
		b.WriteString("\t}\n")
	}
	b.WriteString("\troute {\n")
	b.WriteString("\t\tforward_proxy {\n")
	// Service-level pair is optional for inbound mode (clients may be the
	// only basic_auth lines). Skip when AuthUser is empty.
	if u := strings.TrimSpace(c.AuthUser); u != "" {
		b.WriteString("\t\t\tbasic_auth " +
			caddyToken(u) + " " +
			caddyToken(strings.TrimSpace(c.AuthPass)) + "\n")
	}
	for _, pair := range extraAuth {
		if strings.TrimSpace(pair.User) == "" {
			continue
		}
		b.WriteString("\t\t\tbasic_auth " +
			caddyToken(pair.User) + " " + caddyToken(pair.Pass) + "\n")
	}
	b.WriteString("\t\t\thide_ip\n")
	b.WriteString("\t\t\thide_via\n")
	if c.ProbeResistance {
		b.WriteString("\t\t\tprobe_resistance\n")
	}
	// klzgrad/forwardproxy supports `upstream socks5://…` to localhost
	// natively (no binary patch). The panel injects the matching SOCKS
	// inbound at RouteXrayPort via injectTunnelEgress.
	if c.RouteThroughXray && c.RouteXrayPort > 0 {
		b.WriteString("\t\t\tupstream socks5://127.0.0.1:" + strconv.Itoa(c.RouteXrayPort) + "\n")
	}
	b.WriteString("\t\t}\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

// ClientURL renders the share link consumed by naive-compatible clients
// (NekoBox, husi, Exclave, the official naive client):
//
//	naive+https://USER:PASS@DOMAIN[:PORT]
//
// The port is omitted for 443. Empty domain or user yields "".
func (c NaiveConfig) ClientURL() string {
	domain := strings.TrimSpace(c.Domain)
	user := strings.TrimSpace(c.AuthUser)
	if domain == "" || user == "" {
		return ""
	}
	host := domain
	if c.Port != 443 {
		host = net.JoinHostPort(domain, strconv.Itoa(c.Port))
	}
	u := url.URL{
		Scheme: "https",
		User:   url.UserPassword(user, strings.TrimSpace(c.AuthPass)),
		Host:   host,
	}
	return "naive+" + u.String()
}
