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
	"sync"
	"time"

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
	// Honour the inbound's Listen IP: loopback behind masking, a specific
	// address on multi-IP hosts. Empty/wildcard keeps every interface.
	if IsLoopbackListen(ib.Listen) {
		cfg.Bind = "127.0.0.1"
	} else if ip := net.ParseIP(strings.TrimSpace(ib.Listen)); ip != nil && !ip.IsUnspecified() {
		cfg.Bind = ip.String()
	}
	return cfg, true
}

// AnytlsInstanceFromInbound builds a supervised Instance for one AnyTls
// inbound. secret derives each attached client's own password (empty = the
// shared password only); panelCert/panelKey are the ACME pair used when the
// inbound has no explicit paths. Disabled, invalid, or uncertified rows yield
// Enabled:false so reconcile converges them down.
func AnytlsInstanceFromInbound(ib *model.Inbound, secret []byte, panelCert, panelKey string) (Instance, bool) {
	cfg, ok := AnytlsConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	key := AnytlsKey(ib.Id)
	if !cfg.Enabled {
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	if err := cfg.Validate(); err != nil {
		logger.Warning("tunnel: ", key, " disabled: ", err)
		return Instance{Core: Anytls, Key: key, Enabled: false}, true
	}
	certFile, keyFile := cfg.ResolveCertPaths(panelCert, panelKey)
	// Read+validate once; the copies below reuse the SAME validated bytes
	// instead of re-reading paths that may have been swapped in between.
	certPEM, keyPEM, certErr := validatePEMCertBytes("anytls", certFile, keyFile, cfg.SNI)
	if certErr != nil {
		logger.Warning("tunnel: ", key, " disabled: ", certErr)
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
			extraFiles = map[string]string{
				key + "-cert.pem": string(certPEM),
				key + "-key.pem":  string(keyPEM),
			}
			certFile = filepath.Join(workDir(), key+"-cert.pem")
			keyFile = filepath.Join(workDir(), key+"-key.pem")
			runUid = uid
		} else {
			logAnytlsDegradedOnce(key + ": lucx-mtproxy user missing, routeThroughXray stays a no-op (direct egress) — save the inbound after provisioning the user")
		}
	}
	pwFile := filepath.Join(dataDirFor(key, Anytls), "password")
	if err := mkdirPrivate(filepath.Dir(pwFile)); err != nil {
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
	// Per-user passwords: the users file sits beside the password file and
	// is not part of the fingerprint, so adding or removing a client reloads
	// it inside the running server instead of restarting it. The shared
	// password stays valid (older links keep working).
	if users := AnytlsUsers(secret, ib); len(users) > 0 && AnytlsSupportsUsers() {
		usersFile := filepath.Join(filepath.Dir(pwFile), "users")
		if err := writeFileIfChanged(usersFile, RenderAnytlsUsers(users)); err != nil {
			logger.Warning("tunnel: ", key, " users file: ", err)
		} else if runUid > 0 {
			if _, err := prepareRunAssets([]string{usersFile}, runUid); err != nil {
				logger.Warning("tunnel: ", key, " users file: ", err)
			}
		}
		cfg.usersFile = usersFile
		cfg.statsFile = filepath.Join(filepath.Dir(pwFile), "stats.json")
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

// AnytlsUser is one attached client and its derived AnyTLS password.
type AnytlsUser struct {
	Email    string
	Password string
}

// AnytlsUsers lists the enabled clients of ib with their derived passwords.
func AnytlsUsers(secret []byte, ib *model.Inbound) []AnytlsUser {
	if len(secret) == 0 || ib == nil {
		return nil
	}
	var s struct {
		Clients []struct {
			Email  string `json:"email"`
			Enable bool   `json:"enable"`
		} `json:"clients"`
	}
	_ = json.Unmarshal([]byte(ib.Settings), &s)
	var out []AnytlsUser
	seen := map[string]bool{}
	for _, c := range s.Clients {
		email := strings.TrimSpace(c.Email)
		// ':' separates name and password in the users file.
		if !c.Enable || email == "" || strings.ContainsAny(email, ":\r\n") || seen[email] {
			continue
		}
		seen[email] = true
		out = append(out, AnytlsUser{Email: email, Password: InboundAuthPair(secret, ib, email).Pass})
	}
	return out
}

// RenderAnytlsUsers renders the overlay's -users-file ("email:password").
func RenderAnytlsUsers(users []AnytlsUser) string {
	var b strings.Builder
	for _, u := range users {
		b.WriteString(u.Email + ":" + u.Password + "\n")
	}
	return b.String()
}

// AnytlsStats is the overlay's -stats file: per-user bytes since Started and
// the IPs each user is (or was in the last minutes) connected from. The
// shared password's user is "".
type AnytlsStats struct {
	Started int64                      `json:"started"`
	Updated int64                      `json:"updated"`
	Users   map[string]AnytlsUserStats `json:"users"`
}

type AnytlsUserStats struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
	IPs  map[string]struct {
		Active int   `json:"active"`
		Last   int64 `json:"last"`
	} `json:"ips"`
}

// anytlsStatsMaxAge drops a stats file the server stopped updating.
const anytlsStatsMaxAge = 30 * time.Second

// ReadAnytlsStats reads the stats file of AnyTLS inbound id; ok=false when
// the server doesn't write one (old binary, no clients) or it is stale.
func ReadAnytlsStats(id int) (AnytlsStats, bool) {
	key := AnytlsKey(id)
	b, err := os.ReadFile(filepath.Join(dataDirFor(key, Anytls), "stats.json"))
	if err != nil {
		return AnytlsStats{}, false
	}
	var st AnytlsStats
	if json.Unmarshal(b, &st) != nil || time.Since(time.Unix(st.Updated, 0)) > anytlsStatsMaxAge {
		return AnytlsStats{}, false
	}
	return st, true
}
