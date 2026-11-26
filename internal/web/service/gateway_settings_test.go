package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

// TestNormalizeGatewaySettings_PreservesAppliedStateOnDrawerEdit guards the
// masking wipe (tester VladufQa, 26.11.2026): the inbound drawer round-trips
// only {publicHost} for a gateway, and saving it (the tester toggled
// balancer-only) reset the settings blob — the applied snapshot vanished,
// the masking site came down and the panel fronted by the gateway dropped
// the operator's session. A drawer edit of an applied gateway must carry the
// applied state forward; a payload that already carries it (GatewayApply)
// is left untouched.
func TestNormalizeGatewaySettings_PreservesAppliedStateOnDrawerEdit(t *testing.T) {
	_ = initTrafficTestDB(t)
	svc := &InboundService{}

	applied := `{"enabled":true,"publicHost":"mask.example.com","snapshot":[{"inboundId":11,"listen":"","port":443}],"appliedSettings":"{}","hidePanel":true}`
	ib := model.Inbound{UserId: 1, Tag: "gateway-1", Enable: true, Port: 443, Protocol: model.Gateway, Settings: applied}
	if err := database.GetDB().Create(&ib).Error; err != nil {
		t.Fatalf("create gateway inbound: %v", err)
	}

	// The drawer edit: bare settings from the form round-trip.
	edit := model.Inbound{Id: ib.Id, UserId: 1, Tag: "gateway-1", Enable: true, Port: 443, Protocol: model.Gateway, Settings: `{"publicHost":"mask2.example.com"}`}
	svc.normalizeGatewaySettings(&edit)

	var stored model.Inbound
	if err := database.GetDB().First(&stored, ib.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	var cfg tunnel.GatewayConfig
	if json.Unmarshal([]byte(edit.Settings), &cfg) != nil {
		t.Fatalf("settings unparsable: %s", edit.Settings)
	}
	if cfg.PublicHost != "mask2.example.com" {
		t.Fatalf("publicHost = %q, want the edited value", cfg.PublicHost)
	}
	if !cfg.Applied() {
		t.Fatalf("applied state wiped by the drawer edit, settings = %s", edit.Settings)
	}
	if len(cfg.Snapshot) != 1 || cfg.Snapshot[0].InboundID != 11 {
		t.Fatalf("snapshot = %+v, want the applied one", cfg.Snapshot)
	}
	if !cfg.HidePanel {
		t.Fatal("hidePanel lost in the drawer round-trip")
	}

	// A payload that already carries the applied state (GatewayApply) wins.
	reapply := model.Inbound{Id: ib.Id, UserId: 1, Tag: "gateway-1", Enable: true, Port: 443, Protocol: model.Gateway, Settings: `{"publicHost":"mask3.example.com","snapshot":[{"inboundId":12}]}`}
	svc.normalizeGatewaySettings(&reapply)
	var cfg2 tunnel.GatewayConfig
	if json.Unmarshal([]byte(reapply.Settings), &cfg2) != nil {
		t.Fatalf("settings unparsable: %s", reapply.Settings)
	}
	if len(cfg2.Snapshot) != 1 || cfg2.Snapshot[0].InboundID != 12 {
		t.Fatalf("gateway apply state must not be second-guessed, snapshot = %+v", cfg2.Snapshot)
	}
}
