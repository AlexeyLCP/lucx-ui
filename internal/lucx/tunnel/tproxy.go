// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	Tproxy      Name = "tproxy"
	Mtproxy     Name = "mtproxy"
	TproxyCaddy Name = "tproxycaddy"
)

const (
	tproxyDefaultPort       = 443
	tproxySocksRedirectPort = 23990
)

func TproxySocksRedirectPort() int { return tproxySocksRedirectPort }

// TproxySitePrompt is the operator-facing generator prompt from
// telegramdesktop/tproxy-server PUBLIC_SITE.md. Copied to the clipboard so
// each install gets a unique site; LucX does not ship a shared stub.
const TproxySitePrompt = `Create a distinctive, self-contained static website with three to five HTML pages, shared external CSS, an SVG favicon, a custom 404 page, and no remote resources. Use plain HTML/CSS and optional same-origin external JavaScript. Do not use inline script, inline CSS, forms, analytics, service workers, frames, or a client-side router. Output only deployable files, with index.html at the root and ordinary .html files for clean extensionless links. Do not mention Telegram or a proxy.`

// TproxyConfig is one Telegram WEB proxy inbound (tproxy-server + official
// MTProxy + Caddy TLS reverse_proxy). Share link: t.me/webproxy?server=&secret=.
type TproxyConfig struct {
	Remark           string `json:"remark"`
	Enabled          bool   `json:"enabled"`
	Port             int    `json:"port"`
	Hostname         string `json:"hostname"`
	Secret           string `json:"secret"`
	SiteSource       string `json:"siteSource"` // zip | dir | upstream
	SiteDir          string `json:"siteDir"`
	SiteUpstream     string `json:"siteUpstream"`
	CarrierMode      string `json:"carrierMode"`
	CertFile         string `json:"certFile"`
	KeyFile          string `json:"keyFile"`
	RouteThroughXray bool   `json:"routeThroughXray"`
	RouteXrayPort    int    `json:"routeXrayPort"`
	OutboundTag      string `json:"outboundTag"`
	ExternalTLS      bool   `json:"externalTLS"`
	BehindCover      bool   `json:"behindCover"`
}

func DefaultTproxyConfig() TproxyConfig {
	return TproxyConfig{Port: tproxyDefaultPort, SiteSource: "zip", CarrierMode: "https"}
}

func (c TproxyConfig) Merge() TproxyConfig {
	if c.Port <= 0 {
		c.Port = tproxyDefaultPort
	}
	if strings.TrimSpace(c.SiteSource) == "" {
		c.SiteSource = "zip"
	}
	if strings.TrimSpace(c.CarrierMode) == "" {
		c.CarrierMode = "https"
	}
	c.Hostname = strings.ToLower(strings.TrimSpace(c.Hostname))
	c.Secret = strings.ToLower(strings.TrimSpace(c.Secret))
	c.CarrierMode = strings.ToLower(strings.TrimSpace(c.CarrierMode))
	return c
}

func (c TproxyConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("tproxy: port must be in 1..65535")
	}
	if err := validateTproxyHostname(c.Hostname); err != nil {
		return err
	}
	if err := validateTproxySecret(c.Secret); err != nil {
		return err
	}
	switch c.CarrierMode {
	case "https", "https-lanes", "websocket", "websocket-lanes":
	default:
		return fmt.Errorf("tproxy: unknown carrier_mode %q", c.CarrierMode)
	}
	switch c.SiteSource {
	case "zip", "dir", "upstream":
	default:
		return fmt.Errorf("tproxy: siteSource must be zip, dir, or upstream")
	}
	if c.SiteSource == "upstream" {
		if err := validateTproxyUpstream(c.SiteUpstream); err != nil {
			return err
		}
	}
	return nil
}

func validateTproxyHostname(h string) error {
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return errors.New("tproxy: hostname is required")
	}
	if strings.ContainsAny(h, "/:?#@ ") || strings.Contains(h, "://") {
		return errors.New("tproxy: hostname must be a bare lowercase DNS name")
	}
	return nil
}

func validateTproxySecret(secret string) error {
	secret = strings.ToLower(strings.TrimSpace(secret))
	if len(secret) != 32 {
		return errors.New("tproxy: secret must be 32 hex characters (16 bytes)")
	}
	for _, r := range secret {
		if r < '0' || (r > '9' && r < 'a') || r > 'f' {
			return errors.New("tproxy: secret must be 32 hex characters (16 bytes)")
		}
	}
	return nil
}

func validateTproxyUpstream(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" || u.Path != "" || u.RawQuery != "" {
		return errors.New("tproxy: siteUpstream must be http://127.0.0.1:port")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || port == "" {
		return errors.New("tproxy: siteUpstream must be http://127.0.0.1:port")
	}
	if host != "127.0.0.1" && host != "::1" {
		return errors.New("tproxy: siteUpstream must be loopback")
	}
	return nil
}

func (c TproxyConfig) EnsureSecret() (TproxyConfig, error) {
	if strings.TrimSpace(c.Secret) != "" {
		return c, nil
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return c, err
	}
	c.Secret = hex.EncodeToString(b[:])
	return c, nil
}

func (c TproxyConfig) ResolveCertPaths(panelCert, panelKey string) (cert, key string) {
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

func (c TproxyConfig) ValidateCert(panelCert, panelKey string) error {
	cert, key := c.ResolveCertPaths(panelCert, panelKey)
	return validatePEMCert("tproxy", cert, key, strings.TrimSpace(c.Hostname))
}

func (c TproxyConfig) ClientLink() string {
	host := strings.TrimSpace(c.Hostname)
	secret := strings.TrimSpace(c.Secret)
	if host == "" || secret == "" {
		return ""
	}
	return "https://t.me/webproxy?server=" + url.QueryEscape(host) + "&secret=" + url.QueryEscape(secret)
}

func tproxyLoopback(id, offset int) int {
	return 24000 + id*4 + offset
}

func tproxyXrayComment() string { return "lucx-tproxy-xray" }

// mtproxyRedirectUIDOK rejects root: a NAT OUTPUT REDIRECT on uid 0
// hijacks every locally generated TCP (Xray, Caddy, the panel).
func mtproxyRedirectUIDOK(uid string) bool {
	return uid != "" && uid != "0"
}

func mtproxyXrayRedirectArgs(uid string, port int) []string {
	return []string{
		"-t", "nat", "-I", "OUTPUT",
		"-m", "owner", "--uid-owner", uid,
		"-p", "tcp", "!", "-d", "127.0.0.0/8",
		"-m", "comment", "--comment", tproxyXrayComment(),
		"-j", "REDIRECT", "--to-ports", strconv.Itoa(port),
	}
}

func RenderTproxyCaddyfile(hostname string, port int, cert, key string, relayPort int) string {
	hostPort := hostname + ":" + strconv.Itoa(port)
	var b strings.Builder
	b.WriteString("{\n\tadmin off\n\tauto_https off\n}\n")
	b.WriteString(hostPort)
	b.WriteString(" {\n")
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		b.WriteString("\ttls ")
		b.WriteString(caddyToken(cert))
		b.WriteString(" ")
		b.WriteString(caddyToken(key))
		b.WriteString("\n")
	}
	b.WriteString("\tencode zstd gzip\n")
	b.WriteString("\treverse_proxy 127.0.0.1:")
	b.WriteString(strconv.Itoa(relayPort))
	b.WriteString(" {\n\t\ttransport http {\n\t\t\tresponse_header_timeout 40s\n\t\t}\n\t}\n}\n")
	return b.String()
}

func RenderTproxyConfigJSON(hostname, listen, admin, publicDir, publicUpstream, profilesFile string) (string, error) {
	cfg := map[string]any{
		"public_hostname": hostname,
		"listen":          listen,
		"admin_listen":    admin,
		"public_dir":      publicDir,
		"public_upstream": publicUpstream,
		"profiles_file":   profilesFile,
		"enable_pprof":    false,
		"limits": map[string]any{
			"max_header_bytes":              16384,
			"max_body_bytes":                2097152,
			"max_frame_payload":             1048576,
			"carrier_batch_bytes":           2097152,
			"max_streams_per_session":       128,
			"max_closed_stream_ids":         4096,
			"max_pending_per_session":       33554432,
			"max_pending_global":            536870912,
			"max_pending_items_per_session": 16384,
			"max_pending_items_global":      262144,
			"max_sessions_per_ip":           0,
			"max_sessions_global":           128,
			"max_streams_global":            4096,
			"max_backend_dials_in_flight":   256,
			"new_sessions_per_minute":       600,
			"new_sessions_burst":            128,
			"new_streams_per_minute":        6000,
			"new_streams_burst":             512,
			"max_bootstraps_per_ip":         0,
			"max_bootstraps_global":         512,
			"new_bootstraps_per_minute":     1200,
			"new_bootstraps_burst":          256,
			"max_profiles":                  32,
		},
		"timeouts": map[string]string{
			"backend_dial":       "5s",
			"long_poll":          "25s",
			"reconnect_grace":    "2m",
			"bootstrap_lifetime": "2m",
			"read_header":        "10s",
			"idle":               "75s",
			"shutdown":           "15s",
		},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}

func RenderTproxyProfilesJSON(name, secret, backend, carrier string) (string, error) {
	doc := map[string]any{
		"profiles": []map[string]string{{
			"name":         name,
			"secret":       secret,
			"backend":      backend,
			"carrier_mode": carrier,
		}},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}
