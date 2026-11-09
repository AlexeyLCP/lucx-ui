package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

func TestScanguardLoadConfigWhitelistIsAlwaysArray(t *testing.T) {
	checkWhitelist := func(t *testing.T, cfg any) {
		t.Helper()

		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}

		var payload map[string]json.RawMessage
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}

		if got := string(payload["whitelist"]); got != "[]" {
			t.Fatalf("expected whitelist to be [], got %s", got)
		}
	}

	t.Run("missing setting", func(t *testing.T) {
		dbDir := t.TempDir()
		t.Setenv("XUI_DB_FOLDER", dbDir)
		dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

		cfg := (&ScanguardService{}).LoadConfig()
		checkWhitelist(t, cfg)
	})

	t.Run("stored null", func(t *testing.T) {
		dbDir := t.TempDir()
		t.Setenv("XUI_DB_FOLDER", dbDir)
		dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

		if err := saveScanguardSetting(scanguardWhitelistKey, "null"); err != nil {
			t.Fatal(err)
		}

		cfg := (&ScanguardService{}).LoadConfig()
		checkWhitelist(t, cfg)
	})
}
