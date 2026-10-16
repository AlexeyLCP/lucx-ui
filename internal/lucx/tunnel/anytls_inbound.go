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
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
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
	if IsLoopbackListen(ib.Listen) {
		cfg.Bind = "127.0.0.1"
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
	// Routed egress: the process runs under the shared bridge uid so the
	// uid-REDIRECT (lucx.273) can catch its outbound TCP. The uid cannot read
	// the panel-side cert paths (root-only letsencrypt), so the certs are
	// carried as ExtraFiles copies — writeConfigFile writes them into the
	// tunnel dir, keyed uniquely per instance, and the fingerprint restarts
	// on any renewal because the content rides the fingerprint.
	runUid := 0
	var extraFiles map[string]string
	routed := cfg.RouteThroughXray && cfg.RouteXrayPort > 0
	if routed {
		if uid := redirectOwnerUID(); uid > 0 {
			certPEM, certErr := os.ReadFile(certFile)
			keyPEM, keyErr := os.ReadFile(keyFile)
			if certErr != nil || keyErr != nil {
				logAnytlsDegradedOnce(key + ": cert copy for the bridge uid failed, egress stays direct")
			} else {
				extraFiles = map[string]string{
					key + "-cert.pem": string(certPEM),
					key + "-key.pem":  string(keyPEM),
				}
				certFile = filepath.Join(workDir(), key+"-cert.pem")
				keyFile = filepath.Join(workDir(), key+"-key.pem")
				runUid = uid
			}
		} else {
			logAnytlsDegradedOnce(key + ": lucx-mtproxy user missing, routeThroughXray stays a no-op (direct egress) — save the inbound after provisioning the user")
		}
	}
	pwFile := filepath.Join(dataDirFor(key, Anytls), "password")
	if err := os.MkdirAll(filepath.Dir(pwFile), 0o700); err != nil {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	if err := os.WriteFile(pwFile, []byte(strings.TrimSpace(cfg.Password)+"\n"), 0o600); err != nil {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	// Routed uid cannot read the root-owned password file: pass -p inline,
	// like the pre-password-file builds did.
	pwArg := ""
	if !routed && anytlsSupportsPasswordFile() {
		pwArg = pwFile
	}
	fpExtra := CertFileHash(certFile)
	if runUid > 0 {
		// The copy content rides the fingerprint via ExtraFiles.
		fpExtra = ""
	}
	return Instance{
		Core:             Anytls,
		Key:              key,
		Enabled:          true,
		Args:             cfg.BuildArgs(certFile, keyFile, pwArg),
		ExtraFiles:       extraFiles,
		FingerprintExtra: fpExtra,
		RunUid:           runUid,
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

// anytlsWarnOnce logs a degraded-state reason once per manager life, so the
// 10 s reconcile does not flood the log with the same warning (the lucx.300
// lesson: repeated start failures already flooded once).
var (
	anytlsWarnOnceMu sync.Mutex
	anytlsWarnOnce   = map[string]struct{}{}
)

func logAnytlsDegradedOnce(reason string) {
	anytlsWarnOnceMu.Lock()
	_, seen := anytlsWarnOnce[reason]
	anytlsWarnOnce[reason] = struct{}{}
	anytlsWarnOnceMu.Unlock()
	if !seen {
		logger.Warning(reason)
	}
}
