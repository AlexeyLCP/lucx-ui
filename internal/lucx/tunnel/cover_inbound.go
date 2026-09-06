// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func CoverKey(id int) string { return "cover-" + strconv.Itoa(id) }

func CoverSiteDir(id int) string {
	return filepath.Join(workDir(), CoverKey(id)+"-site")
}

func RemoveCoverSite(id int) {
	_ = os.RemoveAll(CoverSiteDir(id))
}

func CoverConfigFromInbound(ib *model.Inbound) (CoverConfig, bool) {
	if ib == nil || ib.Protocol != model.Cover {
		return CoverConfig{}, false
	}
	cfg := DefaultCoverConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	return cfg.Merge(), true
}

func SettingsBehindCover(protocol model.Protocol, settings string) bool {
	if protocol != model.Naive && protocol != model.Tproxy {
		return false
	}
	var s struct {
		BehindCover bool `json:"behindCover"`
	}
	_ = json.Unmarshal([]byte(settings), &s)
	return s.BehindCover
}

func tproxyBehindCoverReady(ib *model.Inbound, panelCert, panelKey string) bool {
	insts, ok := TproxyInstancesFromInbound(ib, panelCert, panelKey)
	if !ok {
		return false
	}
	for _, inst := range insts {
		if inst.Core == Tproxy && inst.Enabled {
			return true
		}
	}
	return false
}

// CoverInstanceFromInbound builds the single Caddy process for one cover
// inbound, folding matching behindCover naive/tproxy + path routes.
func CoverInstanceFromInbound(ib *model.Inbound, others []*model.Inbound, secret []byte, panelCert, panelKey string) (Instance, bool) {
	cfg, ok := CoverConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := CoverKey(ib.Id)
	disabled := Instance{Core: Cover, Key: key, Enabled: false}
	if !cfg.Enabled {
		return disabled, true
	}
	if err := cfg.Validate(); err != nil {
		logger.Warningf("tunnel: cover-%d disabled: %v", ib.Id, err)
		return disabled, true
	}
	certFile, keyFile := cfg.ResolveCertPaths(panelCert, panelKey)
	if err := validatePEMCert("cover", certFile, keyFile, cfg.Hostname); err != nil {
		logger.Warningf("tunnel: cover-%d disabled: %v", ib.Id, err)
		return disabled, true
	}
	publicDir, publicUpstream, err := coverPublicSource(ib.Id, cfg)
	if err != nil {
		logger.Warningf("tunnel: cover-%d disabled: %v", ib.Id, err)
		return disabled, true
	}

	att := coverAttach{routes: cfg.Routes, publicDir: publicDir, publicUpstream: publicUpstream}
	for _, o := range others {
		if o == nil || !o.Enable || o.NodeID != nil {
			continue
		}
		switch o.Protocol {
		case model.Tproxy:
			tcfg, ok := TproxyConfigFromInbound(o)
			if !ok || !tcfg.BehindCover || tcfg.Hostname != cfg.Hostname {
				continue
			}
			if !tproxyBehindCoverReady(o, panelCert, panelKey) {
				continue
			}
			att.tproxyRelay = tproxyLoopback(o.Id, 2)
			att.naive = nil
			att.naiveAuth = nil
			att.routes = nil
			att.publicDir = ""
			att.publicUpstream = ""
		case model.Naive:
			if att.tproxyRelay > 0 {
				continue
			}
			ncfg, ok := ConfigFromInbound(o)
			if !ok || !naiveMatchesCover(ncfg, cfg.Hostname) {
				continue
			}
			if ncfg.UseRawConfig {
				logger.Warningf("tunnel: cover-%d skip naive-%d: raw Caddyfile", ib.Id, o.Id)
				continue
			}
			var extra []AuthPair
			if len(secret) > 0 {
				var s naiveInboundSettings
				_ = json.Unmarshal([]byte(o.Settings), &s)
				for _, c := range s.Clients {
					if !c.Enable || strings.TrimSpace(c.Email) == "" {
						continue
					}
					extra = append(extra, InboundAuthPair(secret, o, c.Email))
				}
			}
			att.naive = &ncfg
			att.naiveAuth = extra
		}
	}

	caddyfile := RenderCoverCaddyfile(cfg.Hostname, certFile, keyFile, att)
	caddyPath := configPathFor(key, Cover)
	return Instance{
		Core:             Cover,
		Key:              key,
		Enabled:          true,
		ConfigText:       caddyfile,
		Args:             []string{"run", "--config", absPath(caddyPath), "--adapter", "caddyfile"},
		FingerprintExtra: CertFileHash(certFile),
		ProbePort:        coverHTTPSPort,
	}, true
}

func coverPublicSource(id int, cfg CoverConfig) (publicDir, publicUpstream string, err error) {
	switch cfg.SiteSource {
	case "upstream":
		return "", strings.TrimSpace(cfg.SiteUpstream), nil
	case "dir":
		dir := strings.TrimSpace(cfg.SiteDir)
		if err := RequireIndexHTML(dir); err != nil {
			return "", "", err
		}
		return absPath(dir), "", nil
	default:
		dir := CoverSiteDir(id)
		if err := RequireIndexHTML(dir); err != nil {
			return "", "", err
		}
		return absPath(dir), "", nil
	}
}

func ListCoverSite(id int) []string {
	return ListSiteFiles(CoverSiteDir(id))
}

func naiveMatchesCover(n NaiveConfig, hostname string) bool {
	if !n.BehindCover || n.UseRawConfig {
		return false
	}
	h := strings.ToLower(strings.TrimSpace(hostname))
	if h == "" {
		return false
	}
	d := strings.ToLower(strings.TrimSpace(n.Domain))
	return d == "" || d == h
}

// NaiveFrontedByCover is true when a runnable cover inbound will inject this
// naive's forward_proxy. Only then should naive's own Caddy stay down.
func NaiveFrontedByCover(ib *model.Inbound, all []*model.Inbound, secret []byte, panelCert, panelKey string) bool {
	ncfg, ok := ConfigFromInbound(ib)
	if !ok || !ncfg.BehindCover {
		return false
	}
	for _, o := range all {
		inst, ok := CoverInstanceFromInbound(o, all, secret, panelCert, panelKey)
		if !ok || !inst.Enabled {
			continue
		}
		cfg, ok := CoverConfigFromInbound(o)
		if ok && naiveMatchesCover(ncfg, cfg.Hostname) && strings.Contains(inst.ConfigText, "forward_proxy") {
			return true
		}
	}
	return false
}
