// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// MieruPortBinding is one mita listen binding: a single port or an inclusive
// range, over TCP or UDP (enfein/mieru server config schema).
type MieruPortBinding struct {
	Port      int    `json:"port,omitempty"`
	PortRange string `json:"portRange,omitempty"`
	Protocol  string `json:"protocol"`
}

// MieruConfig is the operator-facing configuration of one mieru inbound,
// rendered into the mita server JSON (MITA_CONFIG_JSON_FILE). Users are not
// part of the stored config: they are HMAC-derived per panel client at
// render time (naive model).
type MieruConfig struct {
	Remark       string             `json:"remark"`
	Enabled      bool               `json:"enabled"`
	PortBindings []MieruPortBinding `json:"portBindings"`
	MTU          int                `json:"mtu"`
	LoggingLevel string             `json:"loggingLevel"`

	// RouteThroughXray dials egress through the panel-injected loopback SOCKS
	// bridge (mita native egress.proxies SOCKS5). Default false — direct
	// egress is the camouflage baseline; SOCKS also loses UDP associate.
	RouteThroughXray bool   `json:"routeThroughXray"`
	OutboundTag      string `json:"outboundTag"`
	// RouteXrayPort is the backend-owned loopback SOCKS port (0 when not routed).
	RouteXrayPort int `json:"routeXrayPort"`
}

// DefaultMieruConfig returns sensible defaults for a fresh mieru inbound.
func DefaultMieruConfig() MieruConfig {
	return MieruConfig{
		PortBindings: []MieruPortBinding{{Port: 20100, Protocol: "TCP"}},
		MTU:          1400,
		LoggingLevel: "INFO",
	}
}

// Merge fills zero fields of c from the defaults.
func (c MieruConfig) Merge() MieruConfig {
	def := DefaultMieruConfig()
	if len(c.PortBindings) == 0 {
		c.PortBindings = def.PortBindings
	}
	if c.MTU == 0 {
		c.MTU = def.MTU
	}
	if strings.TrimSpace(c.LoggingLevel) == "" {
		c.LoggingLevel = def.LoggingLevel
	}
	return c
}

// Validate checks the config against the mita server schema.
func (c MieruConfig) Validate() error {
	if len(c.PortBindings) == 0 {
		return fmt.Errorf("mieru: at least one port binding is required")
	}
	for _, b := range c.PortBindings {
		switch strings.ToUpper(strings.TrimSpace(b.Protocol)) {
		case "TCP", "UDP":
		default:
			return fmt.Errorf("mieru: binding protocol must be TCP | UDP")
		}
		if b.PortRange != "" {
			lo, hi, ok := parsePortRange(b.PortRange)
			if !ok {
				return fmt.Errorf("mieru: invalid port range %q (want \"10000-10010\")", b.PortRange)
			}
			if lo < 1025 || hi > 65535 {
				return fmt.Errorf("mieru: port range must be within 1025..65535")
			}
			continue
		}
		if b.Port < 1025 || b.Port > 65535 {
			return fmt.Errorf("mieru: port must be within 1025..65535")
		}
	}
	if c.MTU != 0 && (c.MTU < 1280 || c.MTU > 1500) {
		return fmt.Errorf("mieru: mtu must be within 1280..1500")
	}
	switch strings.ToUpper(strings.TrimSpace(c.LoggingLevel)) {
	case "", "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return fmt.Errorf("mieru: logging level must be one of DEBUG | INFO | WARN | ERROR")
	}
	return nil
}

// ValidateInbound is Validate plus the inbound-mode requirement that at least
// one panel client supplies credentials (mita without users is inert).
func (c MieruConfig) ValidateInbound(hasClientAuth bool) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if !hasClientAuth {
		return fmt.Errorf("mieru: at least one enabled client is required")
	}
	return nil
}

func parsePortRange(s string) (lo, hi int, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(s), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	hi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || lo > hi {
		return 0, 0, false
	}
	return lo, hi, true
}

// MieruPortRangeBounds is the exported parsePortRange for the web layer
// (port-conflict checks).
func MieruPortRangeBounds(s string) (lo, hi int, ok bool) {
	return parsePortRange(s)
}

// MieruPrimaryPort returns the representative listen port of the config: the
// first single-port binding, or the low end of the first range. Used for the
// inbound.Port bookkeeping and the TCP probe.
func MieruPrimaryPort(c MieruConfig) int {
	for _, b := range c.PortBindings {
		if b.Port > 0 {
			return b.Port
		}
		if lo, _, ok := parsePortRange(b.PortRange); ok {
			return lo
		}
	}
	return 0
}

// mitaServerConfig mirrors the mita JSON config fields the panel renders.
type mitaServerConfig struct {
	PortBindings []mitaPortBinding `json:"portBindings"`
	Users        []mitaUser        `json:"users"`
	MTU          int               `json:"mtu,omitempty"`
	LoggingLevel string            `json:"loggingLevel,omitempty"`
	Egress       *mitaEgress       `json:"egress,omitempty"`
}

type mitaPortBinding struct {
	Port      int    `json:"port,omitempty"`
	PortRange string `json:"portRange,omitempty"`
	Protocol  string `json:"protocol"`
}

type mitaUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type mitaEgress struct {
	Proxies []mitaEgressProxy `json:"proxies"`
	Rules   []mitaEgressRule  `json:"rules"`
}

type mitaEgressProxy struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

type mitaEgressRule struct {
	IPRanges    []string `json:"ipRanges"`
	DomainNames []string `json:"domainNames"`
	Action      string   `json:"action"`
	ProxyNames  []string `json:"proxyNames,omitempty"`
}

const mieruEgressProxyName = "lucx-xray"

// RenderJSON produces the mita server config. users are the per-panel-client
// HMAC pairs; routeThroughXray appends the native SOCKS5 egress toward the
// panel's loopback bridge.
func (c MieruConfig) RenderJSON(users []AuthPair) string {
	cfg := mitaServerConfig{
		MTU:          c.MTU,
		LoggingLevel: strings.ToUpper(strings.TrimSpace(c.LoggingLevel)),
	}
	for _, b := range c.PortBindings {
		cfg.PortBindings = append(cfg.PortBindings, mitaPortBinding{
			Port:      b.Port,
			PortRange: strings.TrimSpace(b.PortRange),
			Protocol:  strings.ToUpper(strings.TrimSpace(b.Protocol)),
		})
	}
	for _, u := range users {
		if strings.TrimSpace(u.User) == "" {
			continue
		}
		cfg.Users = append(cfg.Users, mitaUser{Name: u.User, Password: u.Pass})
	}
	if c.RouteThroughXray && c.RouteXrayPort > 0 {
		cfg.Egress = &mitaEgress{
			Proxies: []mitaEgressProxy{{
				Name:     mieruEgressProxyName,
				Protocol: "SOCKS5_PROXY_PROTOCOL",
				Host:     "127.0.0.1",
				Port:     c.RouteXrayPort,
			}},
			Rules: []mitaEgressRule{{
				IPRanges:    []string{"*"},
				DomainNames: []string{"*"},
				Action:      "PROXY",
				ProxyNames:  []string{mieruEgressProxyName},
			}},
		}
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// ClientLink builds the mierus:// simple share link for one client
// (enfein/mieru "Sharing Client Settings" format: profile once, mtu at most
// once, port/protocol paired and repeatable — the query is assembled
// manually to keep each port next to its protocol). host is the bare server
// address (IP or domain, no port).
func (c MieruConfig) ClientLink(host string, pair AuthPair, remark string) string {
	host = strings.TrimSpace(host)
	if host == "" || strings.TrimSpace(pair.User) == "" {
		return ""
	}
	var q []string
	q = append(q, "profile=default")
	if c.MTU > 0 {
		q = append(q, "mtu="+strconv.Itoa(c.MTU))
	}
	for _, b := range c.PortBindings {
		if b.PortRange != "" {
			q = append(q, "port="+url.QueryEscape(strings.TrimSpace(b.PortRange)))
		} else {
			q = append(q, "port="+strconv.Itoa(b.Port))
		}
		q = append(q, "protocol="+strings.ToUpper(strings.TrimSpace(b.Protocol)))
	}
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		host = "[" + host + "]"
	}
	u := url.URL{
		Scheme:   "mierus",
		User:     url.UserPassword(pair.User, pair.Pass),
		Host:     host,
		RawQuery: strings.Join(q, "&"),
		Fragment: strings.TrimSpace(remark),
	}
	return u.String()
}
