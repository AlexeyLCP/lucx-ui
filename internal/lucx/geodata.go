// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package lucx

import (
	"encoding/json"
	"strings"
)

const GeodataCron = "0 4 * * *"

type GeodataAsset struct {
	URL  string `json:"url"`
	File string `json:"file"`
}

var GeodataAssets = []GeodataAsset{
	{"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat", "geoip.dat"},
	{"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", "geosite.dat"},
	{"https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat", "geoip_IR.dat"},
	{"https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat", "geosite_IR.dat"},
	{"https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat", "geoip_RU.dat"},
	{"https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat", "geosite_RU.dat"},
	{"https://github.com/hydraponique/roscomvpn-geoip/releases/latest/download/geoip.dat", "geoip_ROSCOM.dat"},
	{"https://github.com/hydraponique/roscomvpn-geosite/releases/latest/download/geosite.dat", "geosite_ROSCOM.dat"},
}

func geodataAssetMaps() []map[string]any {
	out := make([]map[string]any, 0, len(GeodataAssets))
	for _, a := range GeodataAssets {
		out = append(out, map[string]any{"url": a.URL, "file": a.File})
	}
	return out
}

func loyalsoldierPairOnly(assets []any) bool {
	if len(assets) != 2 {
		return false
	}
	files := map[string]string{}
	for _, raw := range assets {
		obj, ok := raw.(map[string]any)
		if !ok {
			return false
		}
		file, _ := obj["file"].(string)
		url, _ := obj["url"].(string)
		if file == "" || !strings.Contains(url, "Loyalsoldier/v2ray-rules-dat") {
			return false
		}
		files[file] = url
	}
	_, ip := files["geoip.dat"]
	_, site := files["geosite.dat"]
	return ip && site
}

func ApplyGeodata(raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, false, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return raw, false, err
	}
	geo, _ := cfg["geodata"].(map[string]any)
	if geo == nil {
		cfg["geodata"] = map[string]any{
			"assets": geodataAssetMaps(),
			"cron":   GeodataCron,
		}
		out, err := json.MarshalIndent(cfg, "", "  ")
		return string(out) + "\n", true, err
	}
	assets, _ := geo["assets"].([]any)
	if len(assets) == 0 {
		geo["assets"] = geodataAssetMaps()
		if cron, _ := geo["cron"].(string); strings.TrimSpace(cron) == "" {
			geo["cron"] = GeodataCron
		}
		cfg["geodata"] = geo
		out, err := json.MarshalIndent(cfg, "", "  ")
		return string(out) + "\n", true, err
	}
	if !loyalsoldierPairOnly(assets) {
		return raw, false, nil
	}
	have := map[string]struct{}{}
	for _, rawAsset := range assets {
		obj, ok := rawAsset.(map[string]any)
		if !ok {
			continue
		}
		file, _ := obj["file"].(string)
		if file != "" {
			have[file] = struct{}{}
		}
	}
	changed := false
	for _, extra := range GeodataAssets[2:] {
		if _, ok := have[extra.File]; ok {
			continue
		}
		assets = append(assets, map[string]any{"url": extra.URL, "file": extra.File})
		changed = true
	}
	if !changed {
		return raw, false, nil
	}
	geo["assets"] = assets
	cfg["geodata"] = geo
	out, err := json.MarshalIndent(cfg, "", "  ")
	return string(out) + "\n", true, err
}
