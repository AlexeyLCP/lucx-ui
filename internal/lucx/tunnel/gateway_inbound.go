// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func GatewayKey(id int) string { return "gateway-" + strconv.Itoa(id) }

func GatewayConfigFromInbound(ib *model.Inbound) (GatewayConfig, bool) {
	if ib == nil || ib.Protocol != model.Gateway {
		return GatewayConfig{}, false
	}
	cfg := DefaultGatewayConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	return cfg.Merge(), true
}

func GatewayInstanceFromInbound(ib *model.Inbound, others []*model.Inbound, secret []byte, panelCert, panelKey string) (Instance, bool) {
	cfg, ok := GatewayConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := GatewayKey(ib.Id)
	disabled := Instance{Core: Gateway, Key: key, Enabled: false}
	if !cfg.Enabled || !cfg.Applied() {
		return disabled, true
	}
	port := ib.Port
	if port <= 0 {
		port = gatewayDefaultPort
	}
	bindIP := strings.TrimSpace(cfg.BindIP)
	rows := BuildPreview(port, cfg.PublicHost, others, bindIP)
	selected := map[int]bool{}
	for _, s := range cfg.Snapshot {
		selected[s.InboundID] = true
	}
	routes := cfg.Routes
	if len(routes) == 0 {
		routes = RoutesFromPreview(rows, selected)
	}
	if len(routes) == 0 {
		logger.Warningf("tunnel: gateway-%d disabled: no SNI routes", ib.Id)
		return disabled, true
	}
	confPath := configPathFor(key, Gateway)
	probe := port
	if bindIP != "" {
		probe = 0
	}
	conf := ""
	fpExtra := ""
	if cfg.Unified && GatewaySupportsChan() {
		var sites []string
		var certs []string
		byID := map[int]*model.Inbound{}
		for _, o := range others {
			if o != nil {
				byID[o.Id] = o
			}
		}
		fallbackChan := ""
		tproxyChan := ""
		for _, s := range cfg.Snapshot {
			o := byID[s.InboundID]
			if o == nil {
				continue
			}
			site, certFile := gatewaySiteBlock(o, others, secret, panelCert, panelKey)
			if site != "" {
				sites = append(sites, site)
			}
			if certFile != "" {
				certs = append(certs, certFile)
			}
			switch o.Protocol {
			case model.Cover:
				fallbackChan = CoverKey(o.Id)
			case model.Tproxy:
				if tproxyChan == "" {
					tproxyChan = TproxyCaddyKey(o.Id)
				}
			}
		}
		if fallbackChan == "" {
			fallbackChan = tproxyChan
		}
		for _, c := range certs {
			fpExtra += CertFileHash(c)
		}
		conf = RenderUnifiedGatewayCaddyfile(port, routes, fallbackChan, bindIP, sites)
	} else {
		fallback := cfg.Fallback
		if fallback == "" {
			fallback = CoverFallback(rows, selected)
		}
		conf = RenderGatewayCaddyfile(port, routes, fallback, bindIP)
	}
	return Instance{
		Core:             Gateway,
		Key:              key,
		Enabled:          true,
		ConfigText:       conf,
		Args:             []string{"run", "--config", absPath(confPath), "--adapter", "caddyfile"},
		FingerprintExtra: fpExtra,
		ProbePort:        probe,
	}, true
}

// gatewaySiteBlock renders the site block one absorbed caddy-class inbound
// contributes to the unified gateway Caddyfile. Returns the cert path so the
// instance fingerprint tracks renewals. Empty site = the inbound stays down.
func gatewaySiteBlock(o *model.Inbound, others []*model.Inbound, secret []byte, panelCert, panelKey string) (site, certFile string) {
	switch o.Protocol {
	case model.Naive:
		ncfg, ok := ConfigFromInbound(o)
		if !ok || ncfg.UseRawConfig || ncfg.BehindCover {
			return "", ""
		}
		if ncfg.UseAcme || strings.TrimSpace(ncfg.CertFile) == "" || strings.TrimSpace(ncfg.KeyFile) == "" {
			ncfg.UseAcme = false
			ncfg.CertFile = panelCert
			ncfg.KeyFile = panelKey
		}
		extra := naiveClientAuth(secret, o)
		if err := ncfg.ValidateInbound(len(extra) > 0); err != nil {
			logger.Warningf("tunnel: gateway naive-%d site skipped: %v", o.Id, err)
			return "", ""
		}
		return ncfg.RenderSite(NaiveKey(o.Id), extra, AccessLogPath(NaiveKey(o.Id))), ncfg.CertFile
	case model.Cover:
		ccfg, ok := CoverConfigFromInbound(o)
		if !ok {
			return "", ""
		}
		att, cf, kf, ok := coverAttachFor(o, ccfg, others, secret, panelCert, panelKey)
		if !ok {
			return "", ""
		}
		att.bind = "l4chan/" + CoverKey(o.Id)
		if o.Port > 0 {
			att.httpsPort = o.Port
		}
		return RenderCoverSite(ccfg.Hostname, cf, kf, att), cf
	case model.Tproxy:
		tcfg, ok := TproxyConfigFromInbound(o)
		if !ok || tcfg.BehindCover {
			return "", ""
		}
		cf, kf := tcfg.ResolveCertPaths(panelCert, panelKey)
		port := o.Port
		if port <= 0 {
			port = 443
		}
		return RenderTproxySite(tcfg.Hostname, port, cf, kf, tproxyLoopback(o.Id, 2), TproxyCaddyKey(o.Id), gatewayPanelRoutes(others)), cf
	}
	return "", ""
}

// GatewayAbsorbed reports whether an applied unified gateway already serves
// this inbound inside its own process — its standalone sidecar stays off.
func GatewayAbsorbed(ib *model.Inbound, all []*model.Inbound) bool {
	if ib == nil || !IsLoopbackListen(ib.Listen) {
		return false
	}
	if ClassifyInbound(ib).Class != ClassCaddy {
		return false
	}
	if !GatewaySupportsChan() {
		return false
	}
	for _, o := range all {
		cfg, ok := GatewayConfigFromInbound(o)
		if !ok || !cfg.Enabled || !cfg.Applied() || !cfg.Unified {
			continue
		}
		for _, s := range cfg.Snapshot {
			if s.InboundID == ib.Id {
				return true
			}
		}
	}
	return false
}

// gatewayChanCap caches the l4http capability probe per binary mtime so a
// reconcile tick does not fork caddy for every masked inbound.
var gatewayChanCap struct {
	mu    sync.Mutex
	mtime time.Time
	ok    bool
}

// GatewaySupportsChan reports whether the installed caddy-layer4 binary
// carries the caddylucx l4http handler. A pre-merge binary makes the unified
// config unloadable — the gateway then renders the legacy proxy layout and
// standalone sidecars keep running instead of dying with it. A var so tests
// can stub the binary probe.
var GatewaySupportsChan = probeGatewayChan

func probeGatewayChan() bool {
	bin := Gateway.BinaryPath()
	st, err := os.Stat(bin)
	if err != nil {
		return false
	}
	gatewayChanCap.mu.Lock()
	defer gatewayChanCap.mu.Unlock()
	if st.ModTime().Equal(gatewayChanCap.mtime) {
		return gatewayChanCap.ok
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "list-modules").Output()
	gatewayChanCap.ok = err == nil && bytes.Contains(out, []byte("layer4.handlers.l4http"))
	gatewayChanCap.mtime = st.ModTime()
	if !gatewayChanCap.ok {
		logger.Warningf("tunnel: %s lacks layer4.handlers.l4http — unified gateway falls back to proxy layout", bin)
	}
	return gatewayChanCap.ok
}
