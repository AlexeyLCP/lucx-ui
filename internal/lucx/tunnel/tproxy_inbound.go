// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TproxyKey(id int) string      { return "tproxy-" + strconv.Itoa(id) }
func MtproxyKey(id int) string     { return "mtproxy-" + strconv.Itoa(id) }
func TproxyCaddyKey(id int) string { return "tproxycaddy-" + strconv.Itoa(id) }

func TproxySiteDir(id int) string {
	return filepath.Join(dataDirFor(TproxyKey(id), Tproxy), "site")
}

func TproxyConfigFromInbound(ib *model.Inbound) (TproxyConfig, bool) {
	if ib == nil || ib.Protocol != model.Tproxy {
		return TproxyConfig{}, false
	}
	cfg := DefaultTproxyConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	cfg = cfg.Merge()
	if ib.Port > 0 {
		cfg.Port = ib.Port
	}
	return cfg, true
}

func TproxyPrimaryPort(cfg TproxyConfig) int {
	if cfg.Port > 0 {
		return cfg.Port
	}
	return tproxyDefaultPort
}

// TproxyInstancesFromInbound returns the three supervised processes for one
// inbound (MTProxy, tproxy-server, Caddy). Disabled/invalid rows yield three
// Enabled:false slots so reconcile tears them down.
func TproxyInstancesFromInbound(ib *model.Inbound, panelCert, panelKey string) ([]Instance, bool) {
	cfg, ok := TproxyConfigFromInbound(ib)
	if !ok {
		return nil, false
	}
	id := ib.Id
	disabled := []Instance{
		{Core: Mtproxy, Key: MtproxyKey(id), Enabled: false},
		{Core: Tproxy, Key: TproxyKey(id), Enabled: false},
		{Core: TproxyCaddy, Key: TproxyCaddyKey(id), Enabled: false},
	}
	if !cfg.Enabled {
		return disabled, true
	}
	if err := cfg.Validate(); err != nil {
		return disabled, true
	}
	certFile, keyFile := cfg.ResolveCertPaths(panelCert, panelKey)
	if err := validatePEMCert("tproxy", certFile, keyFile, cfg.Hostname); err != nil {
		return disabled, true
	}
	if err := ensureTelegramMtproxyFiles(); err != nil {
		return disabled, true
	}
	publicDir, publicUpstream, err := tproxyPublicSource(id, cfg)
	if err != nil {
		return disabled, true
	}

	mtH := tproxyLoopback(id, 0)
	mtStats := tproxyLoopback(id, 1)
	relayPort := tproxyLoopback(id, 2)
	adminPort := tproxyLoopback(id, 3)
	backend := net.JoinHostPort("127.0.0.1", strconv.Itoa(mtH))
	relayListen := net.JoinHostPort("127.0.0.1", strconv.Itoa(relayPort))
	adminListen := net.JoinHostPort("127.0.0.1", strconv.Itoa(adminPort))

	key := TproxyKey(id)
	profilesName := key + "-profiles.json"
	profilesPath := filepath.Join(workDir(), profilesName)
	if err := os.MkdirAll(workDir(), 0o755); err != nil {
		return disabled, true
	}
	cfgJSON, err := RenderTproxyConfigJSON(cfg.Hostname, relayListen, adminListen, publicDir, publicUpstream, absPath(profilesPath))
	if err != nil {
		return disabled, true
	}
	profilesJSON, err := RenderTproxyProfilesJSON("default", cfg.Secret, backend, cfg.CarrierMode)
	if err != nil {
		return disabled, true
	}
	caddyfile := RenderTproxyCaddyfile(cfg.Hostname, cfg.Port, certFile, keyFile, relayPort)
	cfgPath := configPathFor(key, Tproxy)
	caddyPath := configPathFor(TproxyCaddyKey(id), TproxyCaddy)

	mtArgs := mtproxyArgs(mtStats, mtH, cfg.Secret)
	if len(mtArgs) == 0 {
		return disabled, true
	}

	return []Instance{
		{
			Core:      Mtproxy,
			Key:       MtproxyKey(id),
			Enabled:   true,
			Args:      mtArgs,
			ProbePort: mtH,
		},
		{
			Core:       Tproxy,
			Key:        key,
			Enabled:    true,
			ConfigText: cfgJSON,
			ExtraFiles: map[string]string{profilesName: profilesJSON},
			Args:       []string{"-config", absPath(cfgPath), "-profiles-file", absPath(profilesPath)},
			ProbePort:  relayPort,
		},
		{
			Core:             TproxyCaddy,
			Key:              TproxyCaddyKey(id),
			Enabled:          true,
			ConfigText:       caddyfile,
			Args:             []string{"run", "--config", absPath(caddyPath), "--adapter", "caddyfile"},
			FingerprintExtra: CertFileHash(certFile),
			ProbePort:        cfg.Port,
		},
	}, true
}

func tproxyPublicSource(id int, cfg TproxyConfig) (publicDir, publicUpstream string, err error) {
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
		dir := TproxySiteDir(id)
		if err := RequireIndexHTML(dir); err != nil {
			return "", "", err
		}
		return absPath(dir), "", nil
	}
}
