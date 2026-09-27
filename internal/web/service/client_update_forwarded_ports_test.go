package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func inboundForwardedPorts(t *testing.T, inboundSvc *InboundService, ibId int, email string) string {
	t.Helper()
	ib, err := inboundSvc.GetInbound(ibId)
	if err != nil {
		t.Fatalf("GetInbound %d: %v", ibId, err)
	}
	clients, err := inboundSvc.GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients %d: %v", ibId, err)
	}
	for i := range clients {
		if clients[i].Email == email {
			return clients[i].ForwardedPorts
		}
	}
	t.Fatalf("email %q not found on inbound %d", email, ibId)
	return ""
}

func TestUpdatePersistsForwardedPortsOnAWG(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}

	seeded := model.Client{
		Email:      "fwd@x",
		SubID:      "sub-fwd",
		Enable:     true,
		PublicKey:  "pub",
		PrivateKey: "priv",
		AllowedIPs: []string{"10.8.0.2/32"},
	}
	ib := mkInbound(t, 51820, model.AWG, clientsSettings(t, []model.Client{seeded}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{seeded}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated := seeded
	updated.ForwardedPorts = "8080"
	if _, err := svc.Update(inboundSvc, lookupClientRecord(t, "fwd@x").Id, updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got := inboundForwardedPorts(t, inboundSvc, ib.Id, "fwd@x"); got != "8080" {
		t.Fatalf("inbound forwardedPorts = %q, want 8080", got)
	}
	if got := lookupClientRecord(t, "fwd@x").ForwardedPorts; got != "8080" {
		t.Fatalf("client record forwardedPorts = %q, want 8080", got)
	}
}
