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
	"strings"
)

type TproxyImport struct {
	Hostname string
	Secret   string
	Upstream string
	Carrier  string
	Listen   string
	ConfPath string
	Live     bool
}

func DiscoverTproxy() []TproxyImport {
	return DiscoverTproxyAt("/etc/tproxy-server")
}

func DiscoverTproxyAt(dir string) []TproxyImport {
	cfgPath := filepath.Join(dir, "config.json")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	host := jsonString(cfg, "public_hostname", "hostname")
	listen := jsonString(cfg, "listen", "listen_addr")
	upstream := jsonString(cfg, "public_upstream", "publicUpstream")
	profilesPath := jsonString(cfg, "profiles_file", "profilesFile")
	secret, carrier := readTproxyProfile(dir, profilesPath)
	if host == "" || secret == "" {
		return nil
	}
	if carrier == "" {
		carrier = "https"
	}
	live := listen != "" && probeListening(listen)
	return []TproxyImport{{
		Hostname: strings.ToLower(host),
		Secret:   strings.ToLower(secret),
		Upstream: upstream,
		Carrier:  carrier,
		Listen:   listen,
		ConfPath: cfgPath,
		Live:     live,
	}}
}

func (t TproxyImport) Config() TproxyConfig {
	cfg := DefaultTproxyConfig()
	cfg.Hostname = t.Hostname
	cfg.Secret = t.Secret
	cfg.CarrierMode = t.Carrier
	cfg.ExternalTLS = true
	cfg.Port = 443
	if strings.TrimSpace(t.Upstream) != "" {
		cfg.SiteSource = "upstream"
		cfg.SiteUpstream = t.Upstream
	}
	return cfg
}

func readTproxyProfile(dir, profilesPath string) (secret, carrier string) {
	candidates := []string{profilesPath, filepath.Join(dir, "profiles.json")}
	for _, p := range candidates {
		if strings.TrimSpace(p) == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var wrap struct {
			Profiles []map[string]any `json:"profiles"`
		}
		if err := json.Unmarshal(raw, &wrap); err != nil || len(wrap.Profiles) == 0 {
			continue
		}
		row := wrap.Profiles[0]
		secret = jsonString(row, "secret")
		carrier = jsonString(row, "carrier_mode", "carrierMode")
		if secret != "" {
			return secret, carrier
		}
	}
	return "", ""
}

func jsonString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
