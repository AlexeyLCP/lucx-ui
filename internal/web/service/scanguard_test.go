package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

// Tunnel-port keeps must follow the live tunnel inbounds: enabled non-node
// csqtt/qwdtt rows feed the scanner-guard exemption list (lucx.358).
func TestScanguardKeepPortsFromInbounds(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	svc := &ScanguardService{}
	if got := svc.keepPorts(); len(got) != 0 {
		t.Fatalf("empty db must yield no keeps, got %v", got)
	}

	seed := &model.Inbound{Tag: "sg-csqtt", Protocol: model.Csqtt, Enable: true, Port: 46001, Settings: `{"listenAddr":"0.0.0.0:46000","password":"p"}`}
	if err := database.GetDB().Create(seed).Error; err != nil {
		t.Fatal(err)
	}
	qwdtt := &model.Inbound{Tag: "sg-qwdtt", Protocol: model.Qwdtt, Enable: true, Port: 56005}
	if err := database.GetDB().Create(qwdtt).Error; err != nil {
		t.Fatal(err)
	}
	disabled := &model.Inbound{Tag: "sg-csqtt-off", Protocol: model.Csqtt, Enable: false, Port: 47999}
	if err := database.GetDB().Create(disabled).Error; err != nil {
		t.Fatal(err)
	}
	nodeBind := 7
	noded := &model.Inbound{Tag: "sg-qwdtt-node", Protocol: model.Qwdtt, Enable: true, Port: 57000, NodeID: &nodeBind}
	if err := database.GetDB().Create(noded).Error; err != nil {
		t.Fatal(err)
	}

	got := svc.keepPorts()
	want := []int{46001, 56001, 56005} // csqtt public + qwdtt DTLS + qwdt WG (defaults merged with Port)
	if len(got) != len(want) {
		t.Fatalf("keep ports = %v, want %v", got, want)
	}
	for i, p := range want {
		if got[i] != p {
			t.Fatalf("keep ports = %v, want %v", got, want)
		}
	}
}
