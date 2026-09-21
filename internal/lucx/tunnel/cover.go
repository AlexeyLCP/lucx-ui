// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const Cover Name = "cover"

const (
	coverHTTPSPort = 443
	coverHTTPPort  = 80
)

// CoverRoute is one path → loopback reverse_proxy (VLESS-WS etc.).
type CoverRoute struct {
	Path string `json:"path"`
	Dest string `json:"dest"`
}

// CoverConfig is a camouflage site on :80/:443. Other HTTP sidecars attach
// via behindCover (naive forward_proxy, tproxy reverse_proxy) or Routes.
type CoverConfig struct {
	Remark       string       `json:"remark"`
	Enabled      bool         `json:"enabled"`
	Hostname     string       `json:"hostname"`
	SiteSource   string       `json:"siteSource"` // zip | dir | upstream
	SiteDir      string       `json:"siteDir"`
	SiteUpstream string       `json:"siteUpstream"`
	CertFile     string       `json:"certFile"`
	KeyFile      string       `json:"keyFile"`
	Routes       []CoverRoute `json:"routes"`
}

func DefaultCoverConfig() CoverConfig {
	return CoverConfig{SiteSource: "zip"}
}

func (c CoverConfig) Merge() CoverConfig {
	if strings.TrimSpace(c.SiteSource) == "" {
		c.SiteSource = "zip"
	}
	c.Hostname = strings.ToLower(strings.TrimSpace(c.Hostname))
	var routes []CoverRoute
	for _, r := range c.Routes {
		if strings.TrimSpace(r.Path) == "" {
			continue
		}
		routes = append(routes, r)
	}
	if routes == nil {
		routes = []CoverRoute{}
	}
	c.Routes = routes
	return c
}

func (c CoverConfig) Validate() error {
	if err := validateTproxyHostname(c.Hostname); err != nil {
		return fmt.Errorf("cover: %s", strings.TrimPrefix(err.Error(), "tproxy: "))
	}
	switch c.SiteSource {
	case "zip", "dir", "upstream":
	default:
		return errors.New("cover: siteSource must be zip, dir, or upstream")
	}
	if c.SiteSource == "upstream" {
		if err := validateTproxyUpstream(c.SiteUpstream); err != nil {
			return fmt.Errorf("cover: %s", strings.TrimPrefix(err.Error(), "tproxy: "))
		}
	}
	for _, r := range c.Routes {
		if err := validateCoverRoute(r); err != nil {
			return err
		}
	}
	return nil
}

func validateCoverRoute(r CoverRoute) error {
	path := strings.TrimSpace(r.Path)
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t\n\r{}") {
		return errors.New("cover: path must start with /")
	}
	dest := strings.TrimSpace(r.Dest)
	host, port, err := net.SplitHostPort(dest)
	if err != nil || port == "" {
		return errors.New("cover: dest must be 127.0.0.1:port")
	}
	if host != "127.0.0.1" && host != "::1" {
		return errors.New("cover: dest must be loopback")
	}
	return nil
}

func (c CoverConfig) ResolveCertPaths(panelCert, panelKey string) (cert, key string) {
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

func (c CoverConfig) ValidateCert(panelCert, panelKey string) error {
	cert, key := c.ResolveCertPaths(panelCert, panelKey)
	return validatePEMCert("cover", cert, key, strings.TrimSpace(c.Hostname))
}

type coverAttach struct {
	tproxyRelay    int
	naive          *NaiveConfig
	naiveAuth      []AuthPair
	routes         []CoverRoute
	publicDir      string
	publicUpstream string
	httpsPort      int
	skipHTTP       bool
	// bind overrides the site bind directive (e.g. "l4chan/cover-1" when the
	// site is embedded into the unified gateway Caddyfile).
	bind string
}

func writeCaddyServers(b *strings.Builder, h1h2, proxyProtocol bool) {
	if proxyProtocol {
		h1h2 = true
	}
	if !h1h2 && !proxyProtocol {
		return
	}
	b.WriteString("\tservers {\n")
	if h1h2 {
		b.WriteString("\t\tprotocols h1 h2\n")
	}
	if proxyProtocol {
		b.WriteString("\t\tlistener_wrappers {\n")
		b.WriteString("\t\t\tproxy_protocol {\n\t\t\t\ttimeout 5s\n\t\t\t\tallow 127.0.0.1/32 ::1/128\n\t\t\t}\n")
		b.WriteString("\t\t\ttls\n")
		b.WriteString("\t\t}\n")
	}
	b.WriteString("\t}\n")
}

func RenderCoverCaddyfile(hostname, cert, key string, a coverAttach) string {
	var b strings.Builder
	b.WriteString("{\n\tadmin off\n\tauto_https off\n\tskip_install_trust\n")
	h1h2 := a.tproxyRelay > 0 || (a.naive != nil && !a.naive.EnableH3)
	writeCaddyServers(&b, h1h2, a.skipHTTP)
	b.WriteString("}\n")
	if !a.skipHTTP {
		b.WriteString(":" + strconv.Itoa(coverHTTPPort) + " {\n\tredir https://{host}{uri} permanent\n}\n")
	}
	writeCoverSite(&b, hostname, cert, key, a)
	return b.String()
}

// RenderCoverSite emits only the cover site block for the unified gateway
// Caddyfile — a.bind carries the l4chan listener name.
func RenderCoverSite(hostname, cert, key string, a coverAttach) string {
	var b strings.Builder
	writeCoverSite(&b, hostname, cert, key, a)
	return b.String()
}

func writeCoverSite(b *strings.Builder, hostname, cert, key string, a coverAttach) {
	httpsPort := a.httpsPort
	if httpsPort <= 0 {
		httpsPort = coverHTTPSPort
	}
	// Naive padding dies on host:443 (None). :443, "host" is Variant1 even
	// with file_server/encode in the same site (stand 2026-09-06). Embedded
	// (a.bind set) is the same: the site is the gateway's unknown-SNI
	// fallback and must answer any Host.
	if a.naive != nil || a.bind != "" {
		b.WriteString(":" + strconv.Itoa(httpsPort) + ", " + caddyToken(hostname) + " {\n")
	} else {
		b.WriteString(hostname + ":" + strconv.Itoa(httpsPort) + " {\n")
	}
	switch {
	case a.bind != "":
		b.WriteString("\tbind " + a.bind + "\n")
	case a.skipHTTP:
		b.WriteString("\tbind 127.0.0.1\n")
	}
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		b.WriteString("\ttls " + caddyToken(cert) + " " + caddyToken(key) + "\n")
	}
	if a.tproxyRelay > 0 {
		b.WriteString("\tencode zstd gzip\n")
		b.WriteString("\theader -Via\n")
		writeHTTPPanelRoutes(b, a.routes, "\t")
		b.WriteString("\treverse_proxy 127.0.0.1:" + strconv.Itoa(a.tproxyRelay) +
			" {\n\t\ttransport http {\n\t\t\tresponse_header_timeout 40s\n\t\t}\n\t}\n}\n")
		return
	}
	b.WriteString("\tencode zstd gzip\n")
	if a.publicDir != "" {
		b.WriteString("\troot * " + caddyToken(a.publicDir) + "\n")
	}
	needRoute := a.naive != nil || len(a.routes) > 0
	if needRoute {
		b.WriteString("\troute {\n")
		for _, r := range a.routes {
			path := strings.TrimSpace(r.Path)
			if !strings.HasSuffix(path, "*") {
				path += "*"
			}
			b.WriteString("\t\thandle " + path + " {\n")
			writeCoverReverseProxy(b, r.Dest, "\t\t\t")
			b.WriteString("\t\t}\n")
		}
		if a.naive != nil {
			a.naive.appendForwardProxy(b, a.naiveAuth, "\t\t")
		}
		b.WriteString("\t}\n")
	}
	if a.publicUpstream != "" {
		b.WriteString("\treverse_proxy " + coverUpstreamHost(a.publicUpstream) + "\n")
	} else if a.publicDir != "" {
		b.WriteString("\tfile_server\n")
	}
	b.WriteString("}\n")
}

func writeHTTPPanelRoutes(b *strings.Builder, routes []CoverRoute, indent string) {
	if len(routes) == 0 {
		return
	}
	b.WriteString(indent + "route {\n")
	for _, r := range routes {
		path := strings.TrimSpace(r.Path)
		if path == "" {
			continue
		}
		if !strings.HasSuffix(path, "*") {
			path += "*"
		}
		b.WriteString(indent + "\thandle " + path + " {\n")
		writeCoverReverseProxy(b, r.Dest, indent+"\t\t")
		b.WriteString(indent + "\t}\n")
	}
	b.WriteString(indent + "}\n")
}

func writeCoverReverseProxy(b *strings.Builder, dest, indent string) {
	dest = strings.TrimSpace(dest)
	b.WriteString(indent + "reverse_proxy " + dest + " {\n")
	b.WriteString(indent + "\theader_up Host {http.request.host}\n")
	if strings.HasPrefix(dest, "https://") {
		b.WriteString(indent + "\ttransport http {\n" + indent + "\t\ttls_insecure_skip_verify\n" + indent + "\t}\n")
	}
	b.WriteString(indent + "}\n")
}

func coverUpstreamHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	return u.Host
}
