package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestSweepOrphanGatewayHosts_NodeAdopted: hosts adopted from remote nodes
// live on node-mirrored inbounds and their gateway snapshot exists only on the
// node, so the master's orphan sweep must leave them alone (#195) while still
// sweeping the master's own stale gateway hosts — through DeleteHosts, so the
// delete is tombstoned.
func TestSweepOrphanGatewayHosts_NodeAdopted(t *testing.T) {
	setupBulkDB(t)

	nodeID := 1
	nodeIb := &model.Inbound{Tag: "node-ib", Enable: true, Port: 61001, Protocol: model.VLESS, Settings: `{"clients":[]}`, NodeID: &nodeID}
	if err := database.GetDB().Create(nodeIb).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}
	masterIb := &model.Inbound{Tag: "master-ib", Enable: true, Port: 61002, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(masterIb).Error; err != nil {
		t.Fatalf("create master inbound: %v", err)
	}

	old := time.Now().Add(-10 * time.Minute).UnixMilli()
	nodeHost := &model.Host{InboundId: nodeIb.Id, Remark: gatewayHostRemark, CreatedAt: old}
	masterHost := &model.Host{InboundId: masterIb.Id, Remark: gatewayHostRemark, CreatedAt: old}
	if err := database.GetDB().Create(nodeHost).Error; err != nil {
		t.Fatalf("create node host: %v", err)
	}
	if err := database.GetDB().Create(masterHost).Error; err != nil {
		t.Fatalf("create master host: %v", err)
	}

	(&InboundService{}).sweepOrphanGatewayHosts()

	var left model.Host
	if err := database.GetDB().First(&left, nodeHost.Id).Error; err != nil {
		t.Fatalf("node-adopted gateway host must survive the sweep: %v", err)
	}
	var cnt int64
	if err := database.GetDB().Model(&model.Host{}).Where("id = ?", masterHost.Id).Count(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatal("master's stale gateway host must be swept")
	}
	var tombstones int64
	if err := database.GetDB().Model(&model.HostTombstone{}).
		Where("host_id IN ?", []int{nodeHost.Id, masterHost.Id}).
		Count(&tombstones).Error; err != nil {
		t.Fatal(err)
	}
	if tombstones != 1 {
		t.Fatalf("tombstones = %d, want 1 (only the swept master host)", tombstones)
	}
}
