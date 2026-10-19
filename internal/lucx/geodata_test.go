// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package lucx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyGeodata(t *testing.T) {
	t.Parallel()

	empty := `{"log":{"loglevel":"warning"}}`
	patched, changed, err := ApplyGeodata(empty)
	if err != nil || !changed {
		t.Fatalf("empty geodata: changed=%v err=%v", changed, err)
	}
	assertAssetCount(t, patched, 10)
	assertHasFile(t, patched, "geoip_ROSCOM.dat")
	assertHasFile(t, patched, "geosite_GA.dat")
	assertCron(t, patched, GeodataCron)

	again, changed, err := ApplyGeodata(patched)
	if err != nil || changed {
		t.Fatalf("idempotent: changed=%v err=%v", changed, err)
	}
	if again != patched {
		t.Fatal("idempotent rewrite")
	}

	pair := `{
  "geodata": {
    "cron": "0 5 * * *",
    "assets": [
      {"url": "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat", "file": "geoip.dat"},
      {"url": "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", "file": "geosite.dat"}
    ]
  }
}`
	patched, changed, err = ApplyGeodata(pair)
	if err != nil || !changed {
		t.Fatalf("loyalsoldier pair: changed=%v err=%v", changed, err)
	}
	assertAssetCount(t, patched, 10)
	assertHasFile(t, patched, "geosite_IR.dat")
	assertHasFile(t, patched, "geoip_GA.dat")
	assertCron(t, patched, "0 5 * * *")

	custom := `{
  "geodata": {
    "assets": [
      {"url": "https://example.com/custom.dat", "file": "geoip.dat"}
    ]
  }
}`
	out, changed, err := ApplyGeodata(custom)
	if err != nil || changed {
		t.Fatalf("custom: changed=%v err=%v", changed, err)
	}
	if out != custom {
		t.Fatal("custom list must stay")
	}
}

func TestSanitizeGeodataAssets(t *testing.T) {
	t.Parallel()

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "geoip_GA.dat"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	raw := []byte(`{
  "cron": "0 4 * * *",
  "assets": [
    {"url": "https://raw.githubusercontent.com/Ground-Zerro/Geo-Aggregator/main/geodat/geoip_GA.dat", "file": "geoip_GA.dat"},
    {"url": "https://raw.githubusercontent.com/Ground-Zerro/Geo-Aggregator/main/geodat/geosite_GA.dat", "file": "geosite_GA.dat"}
  ]
}`)
	out := SanitizeGeodataAssets(raw, bin)
	var geo map[string]any
	if err := json.Unmarshal(out, &geo); err != nil {
		t.Fatalf("sanitized json: %v", err)
	}
	assets := geo["assets"].([]any)
	if len(assets) != 1 {
		t.Fatalf("assets=%d want 1 (only the present file survives)", len(assets))
	}
	file := assets[0].(map[string]any)["file"]
	if file != "geoip_GA.dat" {
		t.Fatalf("kept %v, want geoip_GA.dat", file)
	}

	same := SanitizeGeodataAssets(out, bin)
	if string(same) != string(out) {
		t.Fatal("idempotent rewrite")
	}

	if passthrough := SanitizeGeodataAssets([]byte(`{"log":{}}`), bin); string(passthrough) != `{"log":{}}"` && string(passthrough) != `{"log":{}}` {
		t.Fatalf("no-assets block must pass through, got %q", passthrough)
	}
	if bad := SanitizeGeodataAssets([]byte(`{not json`), bin); string(bad) != `{not json` {
		t.Fatalf("invalid json must pass through, got %q", bad)
	}
	if empty := SanitizeGeodataAssets(nil, bin); len(empty) != 0 {
		t.Fatalf("nil must pass through, got %q", empty)
	}
}

func assertAssetCount(t *testing.T, raw string, want int) {
	t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	geo := cfg["geodata"].(map[string]any)
	assets := geo["assets"].([]any)
	if len(assets) != want {
		t.Fatalf("assets=%d want %d", len(assets), want)
	}
}

func assertHasFile(t *testing.T, raw, file string) {
	t.Helper()
	if !strings.Contains(raw, `"file": "`+file+`"`) && !strings.Contains(raw, `"file":"`+file+`"`) {
		t.Fatalf("missing %s", file)
	}
}

func assertCron(t *testing.T, raw, want string) {
	t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	geo := cfg["geodata"].(map[string]any)
	if geo["cron"] != want {
		t.Fatalf("cron=%v want %s", geo["cron"], want)
	}
}
