package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func naiveGuardFixture(id int, remark, domain string, behindCover bool) *model.Inbound {
	m := map[string]any{
		"domain": domain, "behindCover": behindCover,
		"probeResistance": true, "authUser": remark, "authPass": "test",
		"routeThroughXray": true, "routeXrayPort": 30000 + id,
	}
	if behindCover {
		m["hideOn443"] = true
	}
	raw, _ := json.Marshal(m)
	return &model.Inbound{Id: id, Protocol: model.Naive, Remark: remark, Tag: remark, Enable: true, Settings: string(raw)}
}

func TestGuardCoverNaives(t *testing.T) {
	host := "site.example.com"
	byID := map[int]*model.Inbound{
		4: naiveGuardFixture(4, "NaiveFI", host, true),
		7: naiveGuardFixture(7, "NaiveNLv", "", false),
	}
	secret := []byte("test-secret")
	original := byID[7].Settings
	if err := guardCoverNaives(byID, host, []int{7}, secret); err != nil {
		t.Fatalf("compatible Naives must be accepted: %v", err)
	}
	if byID[7].Settings != original {
		t.Fatal("preflight changed live inbound settings")
	}
	byID[7].Settings = strings.ReplaceAll(original, `"NaiveNLv"`, `"NaiveFI"`)
	err := guardCoverNaives(byID, host, []int{7}, secret)
	if err == nil || !strings.Contains(err.Error(), "duplicate credentials") {
		t.Fatalf("expected a credential conflict, got %v", err)
	}
}
