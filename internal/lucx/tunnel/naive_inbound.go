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

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// NaiveKey returns the manager key for a Naive inbound id (naive-12).
func NaiveKey(inboundId int) string {
	return fmt.Sprintf("naive-%d", inboundId)
}

// ClientAuthForInbound derives per-client Naive credentials scoped to one
// inbound so the same email on two Naive inbounds gets distinct basic_auth
// pairs (and so a client detached from inbound A loses access to A only).
func ClientAuthForInbound(secret []byte, inboundId int, email string) AuthPair {
	return ClientAuth(secret, fmt.Sprintf("%d:%s", inboundId, strings.TrimSpace(email)))
}

// naiveInboundSettings is the JSON shape of inbound.settings for protocol=naive.
// Mirrors NaiveConfig plus multi-client clients[] (mtproto pattern).
type naiveInboundSettings struct {
	Remark           string `json:"remark"`
	Listen           string `json:"listen"`
	Domain           string `json:"domain"`
	UseAcme          bool   `json:"useAcme"`
	AcmeEmail        string `json:"acmeEmail"`
	CertFile         string `json:"certFile"`
	KeyFile          string `json:"keyFile"`
	AuthUser         string `json:"authUser"`
	AuthPass         string `json:"authPass"`
	EnableH3         bool   `json:"enableH3"`
	ProbeResistance  bool   `json:"probeResistance"`
	LogLevel         string `json:"logLevel"`
	ExtraArgs        string `json:"extraArgs"`
	RouteThroughXray bool   `json:"routeThroughXray"`
	RouteXrayPort    int    `json:"routeXrayPort"`
	OutboundTag      string `json:"outboundTag"`
	UseRawConfig     bool   `json:"useRawConfig"`
	RawConfig        string `json:"rawConfig"`
	BehindCover      bool   `json:"behindCover"`
	Clients          []struct {
		Email  string `json:"email"`
		Enable bool   `json:"enable"`
	} `json:"clients"`
}

// ConfigFromInbound maps an inbound row to NaiveConfig (port/listen from the
// inbound envelope; the rest from settings JSON).
func ConfigFromInbound(ib *model.Inbound) (NaiveConfig, bool) {
	if ib == nil || ib.Protocol != model.Naive {
		return NaiveConfig{}, false
	}
	var s naiveInboundSettings
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &s)
	}
	cfg := NaiveConfig{
		Remark:           firstNonEmpty(s.Remark, ib.Remark),
		Enabled:          ib.Enable,
		Listen:           firstNonEmpty(s.Listen, ib.Listen),
		Port:             ib.Port,
		Domain:           s.Domain,
		UseAcme:          s.UseAcme,
		AcmeEmail:        s.AcmeEmail,
		CertFile:         s.CertFile,
		KeyFile:          s.KeyFile,
		AuthUser:         s.AuthUser,
		AuthPass:         s.AuthPass,
		EnableH3:         s.EnableH3,
		ProbeResistance:  s.ProbeResistance,
		LogLevel:         s.LogLevel,
		ExtraArgs:        s.ExtraArgs,
		RouteThroughXray: s.RouteThroughXray,
		RouteXrayPort:    s.RouteXrayPort,
		OutboundTag:      s.OutboundTag,
		UseRawConfig:     s.UseRawConfig,
		RawConfig:        s.RawConfig,
		BehindCover:      s.BehindCover,
	}.Merge()
	return cfg, true
}

// InstanceFromInbound builds a supervised Instance for a Naive inbound.
// secret is the panel secret used to derive per-client basic_auth pairs.
// ok is false when the inbound is not Naive or has nothing runnable
// (disabled / invalid / no auth at all).
func InstanceFromInbound(ib *model.Inbound, secret []byte) (Instance, bool) {
	cfg, ok := ConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	if !ib.Enable || cfg.BehindCover {
		return Instance{
			Core:    Naive,
			Key:     NaiveKey(ib.Id),
			Enabled: false,
		}, true
	}

	var extra []AuthPair
	if !cfg.UseRawConfig && len(secret) > 0 {
		var s naiveInboundSettings
		_ = json.Unmarshal([]byte(ib.Settings), &s)
		for _, c := range s.Clients {
			if !c.Enable || strings.TrimSpace(c.Email) == "" {
				continue
			}
			extra = append(extra, InboundAuthPair(secret, ib, c.Email))
		}
	}

	// Inbound mode: service auth optional when at least one client pair exists.
	if err := cfg.ValidateInbound(len(extra) > 0); err != nil {
		return Instance{
			Core:    Naive,
			Key:     NaiveKey(ib.Id),
			Enabled: false,
		}, true
	}

	key := NaiveKey(ib.Id)
	return Instance{
		Core:       Naive,
		Key:        key,
		Enabled:    true,
		ConfigText: cfg.RenderCaddyfile(extra, AccessLogPath(key)),
		ExtraArgs:  cfg.ExtraArgs,
		ProbePort:  cfg.Port,
	}, true
}

// ValidateInbound is like Validate but allows empty service auth when
// hasClientAuth is true (clients on the inbound supply basic_auth lines).
func (c NaiveConfig) ValidateInbound(hasClientAuth bool) error {
	if c.UseRawConfig {
		return c.Validate()
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("naive: port must be in 1..65535")
	}
	svcOK := strings.TrimSpace(c.AuthUser) != "" && strings.TrimSpace(c.AuthPass) != ""
	if !svcOK && !hasClientAuth {
		return fmt.Errorf("naive: auth user/password or at least one enabled client is required")
	}
	if c.UseAcme {
		if strings.TrimSpace(c.Domain) == "" {
			return fmt.Errorf("naive: Auto TLS requires a domain")
		}
		if c.Port != 443 {
			return fmt.Errorf("naive: Auto TLS (Let's Encrypt HTTP-01) requires port 443")
		}
	} else {
		if strings.TrimSpace(c.CertFile) == "" || strings.TrimSpace(c.KeyFile) == "" {
			return fmt.Errorf("naive: certificate and key file paths are required (or enable Auto TLS)")
		}
	}
	switch strings.ToUpper(strings.TrimSpace(c.LogLevel)) {
	case "", "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return fmt.Errorf("naive: log level must be one of DEBUG | INFO | WARN | ERROR")
	}
	if c.RouteThroughXray && c.UseRawConfig {
		return fmt.Errorf("naive: Route through Xray is unavailable in raw Caddyfile mode")
	}
	if err := c.checkCaddyFields(); err != nil {
		return err
	}
	return nil
}

// ClientURLFor builds a naive+https share link for one auth pair.
func (c NaiveConfig) ClientURLFor(pair AuthPair, remark string) string {
	domain := strings.TrimSpace(c.Domain)
	user := strings.TrimSpace(pair.User)
	if domain == "" || user == "" {
		return ""
	}
	port := c.Port
	if port <= 0 {
		port = 443
	}
	host := net.JoinHostPort(domain, strconv.Itoa(port))
	u := url.URL{
		Scheme:   "https",
		User:     url.UserPassword(user, strings.TrimSpace(pair.Pass)),
		Host:     host,
		Fragment: strings.TrimSpace(remark),
	}
	return "naive+" + u.String()
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
