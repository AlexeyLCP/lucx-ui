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
	"os"
	"path/filepath"
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
	// H2C is set at render time when Dest is a loopback Xray XHTTP/gRPC
	// inbound: those speak HTTP/2 cleartext and need an unbuffered stream.
	H2C bool `json:"-"`
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
	// LocalPort also serves the site as plain HTTP on 127.0.0.1: HTTP/1.1 on
	// LocalPort, HTTP/2 cleartext on LocalPort+1. Xray TCP fallbacks and the
	// panel's site upstreams point here (0 = off).
	LocalPort int `json:"localPort,omitempty"`
	// LoginPage serves the site as a sign-in page: every page is index.html
	// and any form post answers with error.html, so nothing ever signs in.
	LoginPage bool `json:"loginPage,omitempty"`
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
	if c.LocalPort != 0 && (c.LocalPort < 1024 || c.LocalPort > 65534) {
		return errors.New("cover: local site port must be 1024-65534")
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
	localPort      int
	loginPage      bool
	// listenIP pins a standalone cover's :80 and :443 to the inbound's own
	// IP, so covers on other IPs of the host don't collide.
	listenIP string
	// bind overrides the site bind directive (e.g. "l4chan/cover-1 127.0.0.1"
	// when the site is embedded into the unified gateway Caddyfile).
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
	b.WriteString("{\n\tadmin off\n\tauto_https off\n\tgrace_period 1s\n\tskip_install_trust\n")
	h1h2 := a.tproxyRelay > 0 || (a.naive != nil && !a.naive.EnableH3)
	writeCaddyServers(&b, h1h2, a.skipHTTP)
	b.WriteString(coverLocalServers(a))
	b.WriteString("}\n")
	if !a.skipHTTP {
		writeHTTPRedirectSite(&b, ":"+strconv.Itoa(coverHTTPPort), a.listenIP)
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

// coverLocalServers is the global-options part of the local site: HTTP/1.1
// only on localPort and HTTP/2 cleartext only on localPort+1, so a fallback
// keeps the protocol the client negotiated (h2 never gets an HTTP/1.1 reply).
func coverLocalServers(a coverAttach) string {
	if a.localPort <= 0 || a.tproxyRelay > 0 {
		return ""
	}
	p := strconv.Itoa(a.localPort)
	q := strconv.Itoa(a.localPort + 1)
	return "\tservers 127.0.0.1:" + p + " {\n\t\tprotocols h1\n\t}\n" +
		"\tservers 127.0.0.1:" + q + " {\n\t\tprotocols h2c\n\t}\n"
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
	switch {
	case a.bind != "":
		// Hostname carries the port: bare, it would add a :443 listener that
		// takes loopback conns away from the gateway when the cover is on 8443.
		b.WriteString(":" + strconv.Itoa(httpsPort) + ", " + caddyToken(hostname+":"+strconv.Itoa(httpsPort)) + " {\n")
	case a.naive != nil:
		b.WriteString(":" + strconv.Itoa(httpsPort) + ", " + caddyToken(hostname) + " {\n")
	default:
		b.WriteString(hostname + ":" + strconv.Itoa(httpsPort) + " {\n")
	}
	switch {
	case a.bind != "":
		b.WriteString("\tbind " + a.bind + "\n")
	case a.skipHTTP:
		b.WriteString("\tbind 127.0.0.1\n")
	case a.listenIP != "":
		b.WriteString("\tbind " + a.listenIP + "\n")
	}
	if strings.TrimSpace(cert) != "" && strings.TrimSpace(key) != "" {
		b.WriteString("\ttls " + caddyToken(cert) + " " + caddyToken(key) + "\n")
	}
	if a.tproxyRelay > 0 {
		writeSiteEncode(b, a.routes, "\t")
		writeSiteHeaders(b, "\t")
		writeHTTPPanelRoutes(b, a.routes, "\t")
		b.WriteString("\treverse_proxy 127.0.0.1:" + strconv.Itoa(a.tproxyRelay) + " {\n")
		writeReverseProxyCamouflage(b, "\t\t")
		b.WriteString("\t\ttransport http {\n\t\t\tresponse_header_timeout 40s\n\t\t}\n\t}\n}\n")
		return
	}
	writeSiteEncode(b, a.routes, "\t")
	writeSiteHeaders(b, "\t")
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
			writeCoverReverseProxy(b, r, "\t\t\t")
			b.WriteString("\t\t}\n")
		}
		if a.naive != nil {
			a.naive.appendForwardProxy(b, a.naiveAuth, "\t\t")
		}
		b.WriteString("\t}\n")
	}
	writeSiteContent(b, a, "\t")
	b.WriteString("}\n")
	writeCoverLocalSites(b, a)
}

// writeSiteContent serves the public site: the upstream, or the files with
// the site's own 404 page (and the sign-in behaviour for LoginPage).
func writeSiteContent(b *strings.Builder, a coverAttach, indent string) {
	if a.publicUpstream != "" {
		b.WriteString(indent + "reverse_proxy " + coverUpstreamHost(a.publicUpstream) + " {\n")
		writeReverseProxyCamouflage(b, indent+"\t")
		b.WriteString(indent + "}\n")
		return
	}
	if a.publicDir == "" {
		return
	}
	if a.loginPage {
		b.WriteString(indent + "@post method POST\n")
		b.WriteString(indent + "handle @post {\n" + indent + "\tmethod GET\n" + indent + "\trewrite * /error.html\n" + indent + "\tfile_server\n" + indent + "}\n")
		b.WriteString(indent + "@file file\n")
		b.WriteString(indent + "handle @file {\n" + indent + "\tfile_server\n" + indent + "}\n")
		b.WriteString(indent + "handle {\n" + indent + "\trewrite * /index.html\n" + indent + "\tfile_server\n" + indent + "}\n")
		return
	}
	b.WriteString(indent + "file_server\n")
	if _, err := os.Stat(filepath.Join(a.publicDir, "404.html")); err == nil {
		b.WriteString(indent + "handle_errors {\n" + indent + "\trewrite * /404.html\n" + indent + "\tfile_server {\n" + indent + "\t\tstatus {err.status_code}\n" + indent + "\t}\n" + indent + "}\n")
	}
}

// writeCoverLocalSites serves the same site as plain HTTP on loopback for
// Xray fallbacks and the panel's site upstreams; path routes stay off it.
func writeCoverLocalSites(b *strings.Builder, a coverAttach) {
	if a.localPort <= 0 || a.tproxyRelay > 0 {
		return
	}
	for _, port := range []int{a.localPort, a.localPort + 1} {
		b.WriteString("http://:" + strconv.Itoa(port) + " {\n\tbind 127.0.0.1\n")
		b.WriteString("\tencode zstd gzip\n")
		writeSiteHeaders(b, "\t")
		if a.publicDir != "" {
			b.WriteString("\troot * " + caddyToken(a.publicDir) + "\n")
		}
		writeSiteContent(b, a, "\t")
		b.WriteString("}\n")
	}
}

// writeSiteEncode compresses the site's pages but never a path route: those
// carry proxy streams (XHTTP, WS), and a client that advertises zstd/gzip in
// its browser-like headers would get a compressed tunnel it can't read.
func writeSiteEncode(b *strings.Builder, routes []CoverRoute, indent string) {
	var paths []string
	for _, r := range routes {
		p := strings.TrimSpace(r.Path)
		if p == "" {
			continue
		}
		if !strings.HasSuffix(p, "*") {
			p += "*"
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		b.WriteString(indent + "encode zstd gzip\n")
		return
	}
	b.WriteString(indent + "@site_pages not path " + strings.Join(paths, " ") + "\n")
	b.WriteString(indent + "encode @site_pages zstd gzip\n")
}

// writeSiteHeaders hides the server software instead of claiming one the
// site doesn't behave like (a fake "nginx" on Caddy answers is a tell).
func writeSiteHeaders(b *strings.Builder, indent string) {
	b.WriteString(indent + "header -Via\n")
	b.WriteString(indent + "header -Server\n")
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
		writeCoverReverseProxy(b, r, indent+"\t\t")
		b.WriteString(indent + "\t}\n")
	}
	b.WriteString(indent + "}\n")
}

// A route carries a proxy tunnel (VLESS-WS, XHTTP, gRPC), so responses are
// flushed at once; H2C routes also talk HTTP/2 cleartext with no timeouts.
func writeCoverReverseProxy(b *strings.Builder, r CoverRoute, indent string) {
	dest := strings.TrimSpace(r.Dest)
	if r.H2C {
		dest = "h2c://" + dest
	}
	b.WriteString(indent + "reverse_proxy " + dest + " {\n")
	b.WriteString(indent + "\theader_up Host {http.request.host}\n")
	writeClientIPHeaders(b, indent+"\t")
	b.WriteString(indent + "\tflush_interval -1\n")
	writeReverseProxyCamouflage(b, indent+"\t")
	if r.H2C {
		b.WriteString(indent + "\ttransport http {\n" + indent + "\t\tversions h2c 2\n" +
			indent + "\t\tread_timeout 0\n" + indent + "\t\twrite_timeout 0\n" + indent + "\t}\n")
	} else if strings.HasPrefix(dest, "https://") {
		b.WriteString(indent + "\ttransport http {\n" + indent + "\t\ttls_insecure_skip_verify\n" + indent + "\t}\n")
	}
	b.WriteString(indent + "}\n")
}

// writeClientIPHeaders makes the connection's address the only client IP a
// route's backend can see. Xray trusts X-Forwarded-For behind a Cover
// (TrustCoverForwardedFor), so a client must not be able to choose it:
// Caddy already replaces it unless trusted_proxies is set, and setting it
// explicitly keeps that true whatever the global options say. X-Real-IP and
// Forwarded pass through Caddy untouched, so they are dropped.
func writeClientIPHeaders(b *strings.Builder, indent string) {
	b.WriteString(indent + "header_up X-Forwarded-For {http.request.remote.host}\n")
	b.WriteString(indent + "header_up -X-Real-IP\n")
	b.WriteString(indent + "header_up -Forwarded\n")
}

// writeReverseProxyCamouflage strips Caddy's own Via and the upstream's
// Server header. Site-level "header -Via" runs before reverse_proxy adds Via,
// so ByeDPI would still see "1.1 Caddy".
func writeReverseProxyCamouflage(b *strings.Builder, indent string) {
	b.WriteString(indent + "header_down -Via\n")
	b.WriteString(indent + "header_down -Server\n")
}

func coverUpstreamHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	return u.Host
}
