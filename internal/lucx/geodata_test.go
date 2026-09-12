// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package lucx

import (
	"encoding/json"
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
	assertAssetCount(t, patched, 8)
	assertHasFile(t, patched, "geoip_ROSCOM.dat")
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
	assertAssetCount(t, patched, 8)
	assertHasFile(t, patched, "geosite_IR.dat")
	assertHasFile(t, patched, "geoip_RU.dat")
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
