package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestDisableGatewayMask_NodeInboundIsSkipped guards the adopted-gateway
// deadlock: setRemoteTraffic mirrors a node panel's SNI gateway onto the
// master with node_id set, and its settings snapshot carries node-side
// inbound ids. Toggling off or deleting that mirror must not run the master
// revert (it previously failed with "gateway inbound not found" and blocked
// node deletion) — the node reverts its own mask.
func TestDisableGatewayMask_NodeInboundIsSkipped(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	other := &model.Node{Name: "other"}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	otherID := other.Id
	if err := db.Create(&model.Inbound{UserId: 1, Tag: "n1-in-443", Enable: true, Port: 443, Protocol: model.VLESS, NodeID: &otherID}).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}

	node := &model.Node{Name: "n1"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	nid := node.Id
	snap := `{"enabled":true,"snapshot":[{"inboundId":7}]}`
	ib := model.Inbound{UserId: 1, Tag: "gateway-1", Enable: true, Port: 443, Protocol: model.Gateway, NodeID: &nid, Settings: snap}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatalf("create node gateway inbound: %v", err)
	}

	reverted, err := svc.DisableGatewayMask(ib.Id)
	if err != nil {
		t.Fatalf("DisableGatewayMask on node gateway: %v", err)
	}
	if reverted {
		t.Fatal("node gateway mask must not be reverted by the master")
	}

	var stored model.Inbound
	database.GetDB().First(&stored, ib.Id)
	var cfg struct {
		Snapshot []json.RawMessage `json:"snapshot"`
	}
	if err := json.Unmarshal([]byte(stored.Settings), &cfg); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if len(cfg.Snapshot) != 1 {
		t.Fatalf("snapshot rows = %d, want 1 (node mask untouched)", len(cfg.Snapshot))
	}
}
