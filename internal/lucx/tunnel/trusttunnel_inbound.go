// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TrustTunnelKey returns the manager key for a TrustTunnel inbound id.
func TrustTunnelKey(inboundId int) string {
	return fmt.Sprintf("trusttunnel-%d", inboundId)
}

// trustTunnelInboundSettings is the JSON shape of inbound.settings for
// protocol=trusttunnel: TrustTunnelConfig plus multi-client clients[].
type trustTunnelInboundSettings struct {
	Remark             string `json:"remark"`
	Hostname           string `json:"hostname"`
	Listen             string `json:"listen"`
	IPv6               bool   `json:"ipv6"`
	CertFile           string `json:"certFile"`
	KeyFile            string `json:"keyFile"`
	ClientDNS          string `json:"clientDns"`
	UpstreamProtocol   string `json:"upstreamProtocol"`
	RouteThroughXray   bool   `json:"routeThroughXray"`
	RouteXrayPort      int    `json:"routeXrayPort"`
	OutboundTag        string `json:"outboundTag"`
	MetricsPort        int    `json:"metricsPort"`
	ListenPreset       string `json:"listenPreset"`
	ClientRandomPrefix string `json:"clientRandomPrefix"`
	Clients            []struct {
		Email  string `json:"email"`
		Enable bool   `json:"enable"`
	} `json:"clients"`
}

// TrustTunnelConfigFromInbound maps an inbound row to TrustTunnelConfig.
func TrustTunnelConfigFromInbound(ib *model.Inbound) (TrustTunnelConfig, bool) {
	if ib == nil || ib.Protocol != model.TrustTunnel {
		return TrustTunnelConfig{}, false
	}
	var s trustTunnelInboundSettings
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &s)
	}
	cfg := TrustTunnelConfig{
		Remark:             firstNonEmpty(s.Remark, ib.Remark),
		Enabled:            ib.Enable,
		Hostname:           s.Hostname,
		Listen:             s.Listen,
		IPv6:               s.IPv6,
		CertFile:           s.CertFile,
		KeyFile:            s.KeyFile,
		ClientDNS:          s.ClientDNS,
		UpstreamProtocol:   s.UpstreamProtocol,
		RouteThroughXray:   s.RouteThroughXray,
		RouteXrayPort:      s.RouteXrayPort,
		OutboundTag:        s.OutboundTag,
		MetricsPort:        s.MetricsPort,
		ListenPreset:       s.ListenPreset,
		ClientRandomPrefix: s.ClientRandomPrefix,
	}.Merge()
	return cfg, true
}

// TrustTunnelInstanceFromInbound builds a supervised Instance for a
// TrustTunnel inbound. secret derives per-client credentials; panelCert /
// panelKey are the panel ACME cert paths used when the inbound carries no
// explicit paths. Invalid cert / no clients / bad config → Enabled:false
// instance (reconcile converges it down; the save hook rejects with a
// human-readable reason).
func TrustTunnelInstanceFromInbound(ib *model.Inbound, secret []byte, panelCert, panelKey string) (Instance, bool) {
	cfg, ok := TrustTunnelConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := TrustTunnelKey(ib.Id)
	if !ib.Enable {
		return Instance{Core: TrustTunnel, Key: key, Enabled: false}, true
	}
	if err := cfg.Validate(); err != nil {
		return Instance{Core: TrustTunnel, Key: key, Enabled: false}, true
	}

	certFile, keyFile := cfg.ResolveCertPaths(panelCert, panelKey)
	if err := ValidateCertFiles(certFile, keyFile, cfg.Hostname); err != nil {
		return Instance{Core: TrustTunnel, Key: key, Enabled: false}, true
	}

	var users []AuthPair
	if len(secret) > 0 {
		var s trustTunnelInboundSettings
		_ = json.Unmarshal([]byte(ib.Settings), &s)
		for _, c := range s.Clients {
			if !c.Enable || strings.TrimSpace(c.Email) == "" {
				continue
			}
			users = append(users, InboundAuthPair(secret, ib, c.Email))
		}
	}
	if len(users) == 0 {
		return Instance{Core: TrustTunnel, Key: key, Enabled: false}, true
	}

	credsName := key + "-credentials.toml"
	rulesName := key + "-rules.toml"
	hostsName := trustTunnelHostsFileName(key)

	socksAddr := ""
	if cfg.RouteThroughXray && cfg.RouteXrayPort > 0 {
		socksAddr = fmt.Sprintf("127.0.0.1:%d", cfg.RouteXrayPort)
	}
	metricsAddr := ""
	if cfg.MetricsPort > 0 {
		metricsAddr = fmt.Sprintf("127.0.0.1:%d", cfg.MetricsPort)
	}

	return Instance{
		Core:    TrustTunnel,
		Key:     key,
		Enabled: true,
		ConfigText: cfg.RenderVpnToml(
			filepath.Join(workDir(), credsName),
			filepath.Join(workDir(), rulesName),
			socksAddr, metricsAddr,
		),
		ExtraFiles: map[string]string{
			hostsName: cfg.RenderHostsToml(certFile, keyFile),
			credsName: RenderTrustTunnelCredentials(users),
			rulesName: RenderTrustTunnelRules(cfg.ClientRandomPrefix),
		},
		FingerprintExtra: CertFileHash(certFile),
		ProbePort:        cfg.ListenPort(),
	}, true
}
