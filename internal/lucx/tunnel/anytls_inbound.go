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
)

// AnytlsKey is the manager key of one AnyTls inbound instance.
func AnytlsKey(id int) string {
	return "anytls-" + strconv.Itoa(id)
}

// AnytlsConfigFromInbound maps an inbound row to AnytlsConfig.
func AnytlsConfigFromInbound(ib *model.Inbound) (AnytlsConfig, bool) {
	if ib == nil || ib.Protocol != model.Anytls {
		return AnytlsConfig{}, false
	}
	cfg := DefaultAnytlsConfig()
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

// AnytlsInstanceFromInbound builds a supervised Instance for one AnyTls
// inbound. panelCert/panelKey are the ACME pair used when the inbound has
// no explicit paths. Disabled, invalid, or uncertified rows yield
// Enabled:false so reconcile converges them down.
func AnytlsInstanceFromInbound(ib *model.Inbound, panelCert, panelKey string) (Instance, bool) {
	cfg, ok := AnytlsConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := AnytlsKey(ib.Id)
	if !cfg.Enabled {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	if err := cfg.Validate(); err != nil {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	certFile, keyFile := cfg.ResolveCertPaths(panelCert, panelKey)
	if err := validatePEMCert("anytls", certFile, keyFile, cfg.SNI); err != nil {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	pwFile := filepath.Join(dataDirFor(key, Anytls), "password")
	if err := os.MkdirAll(filepath.Dir(pwFile), 0o700); err != nil {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	if err := os.WriteFile(pwFile, []byte(strings.TrimSpace(cfg.Password)+"\n"), 0o600); err != nil {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	return Instance{
		Core:             Anytls,
		Key:              key,
		Enabled:          true,
		Args:             cfg.BuildArgs(certFile, keyFile, pwFile),
		FingerprintExtra: CertFileHash(certFile),
		ProbePort:        cfg.Port,
	}, true
}

// AnytlsPrimaryPort returns the TCP listen port for inbound.Port.
func AnytlsPrimaryPort(cfg AnytlsConfig) int {
	if cfg.Port > 0 {
		return cfg.Port
	}
	return 8443
}
