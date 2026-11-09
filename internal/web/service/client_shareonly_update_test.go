package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUpdateAfterShareOnlyAttach(t *testing.T) {
	for _, proto := range []model.Protocol{model.Qwdtt, model.Olcrtc, model.Tproxy} {
		t.Run(string(proto), func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			source := []model.Client{{
				Email:  "fox",
				ID:     "aaaaaaaa-0000-0000-0000-0000000000aa",
				SubID:  "sub-fox",
				Enable: true,
			}}
			vless := mkInbound(t, 22101, model.VLESS, clientsSettings(t, source))
			if err := svc.SyncInbound(nil, vless.Id, source); err != nil {
				t.Fatalf("seed vless: %v", err)
			}
			share := mkInbound(t, 56001, proto, `{"remark":"share"}`)
			rec := lookupClientRecord(t, "fox")

			if _, err := svc.Attach(inboundSvc, rec.Id, []int{share.Id}); err != nil {
				t.Fatalf("Attach %s: %v", proto, err)
			}

			updated := source[0]
			updated.TotalGB = 11
			if _, err := svc.Update(inboundSvc, rec.Id, updated, 0); err != nil {
				t.Fatalf("Update after %s attach: %v", proto, err)
			}
			if got := lookupClientRecord(t, "fox").TotalGB; got != 11 {
				t.Fatalf("totalGB after update = %d, want 11", got)
			}
			if got := mustInboundSettings(t, inboundSvc, share.Id); got != `{"remark":"share"}` {
				t.Fatalf("%s settings mutated: %s", proto, got)
			}
		})
	}
}

// TestUpdateShareOnlyInboundKeepsMembers: a plain settings edit via
// UpdateInbound must not detach share-only members (#196) — their membership
// lives in client_inbounds, settings never carries a clients array.
func TestUpdateShareOnlyInboundKeepsMembers(t *testing.T) {
	cases := []struct {
		proto    model.Protocol
		port     int
		settings string
	}{
		{model.Qwdtt, 56011, `{"remark":"qwdtt"}`},
		{model.Csqtt, 56012, `{"remark":"csqtt"}`},
		{model.Olcrtc, 56013, `{"remark":"olcrtc"}`},
		{model.Tproxy, 56014, `{"hostname":"x.example","secret":"000102030405060708090a0b0c0d0e0f","siteSource":"upstream","siteUpstream":"http://127.0.0.1:60141"}`},
	}
	for _, tc := range cases {
		t.Run(string(tc.proto), func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			source := []model.Client{{Email: "fox", SubID: "sub-fox", Enable: true}}
			vless := mkInbound(t, 22111, model.VLESS, clientsSettings(t, source))
			if err := svc.SyncInbound(nil, vless.Id, source); err != nil {
				t.Fatalf("seed vless: %v", err)
			}
			share := mkInbound(t, tc.port, tc.proto, tc.settings)
			rec := lookupClientRecord(t, "fox")
			if _, err := svc.Attach(inboundSvc, rec.Id, []int{share.Id}); err != nil {
				t.Fatalf("Attach %s: %v", tc.proto, err)
			}

			loaded, err := inboundSvc.GetInbound(share.Id)
			if err != nil {
				t.Fatalf("GetInbound: %v", err)
			}
			update := *loaded
			update.Remark = "edited"
			if _, _, err := inboundSvc.UpdateInbound(&update); err != nil {
				t.Fatalf("UpdateInbound(%s): %v", tc.proto, err)
			}

			got, err := svc.ListForInbound(nil, share.Id)
			if err != nil {
				t.Fatalf("ListForInbound: %v", err)
			}
			if len(got) != 1 || got[0].Email != "fox" {
				t.Fatalf("%s: UpdateInbound detached members: %+v", tc.proto, got)
			}
		})
	}
}

func TestUpdateShareOnlyOnlyClient(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{{Email: "fox", SubID: "sub-fox", Enable: true}}
	share := mkInbound(t, 56002, model.Qwdtt, `{"remark":"qwdtt"}`)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     source[0],
		InboundIds: []int{share.Id},
	}); err != nil {
		t.Fatalf("Create on qWDTT: %v", err)
	}
	rec := lookupClientRecord(t, "fox")
	updated := source[0]
	updated.TotalGB = 7
	updated.Comment = "ok"
	if _, err := svc.Update(inboundSvc, rec.Id, updated, 0); err != nil {
		t.Fatalf("Update qWDTT-only: %v", err)
	}
	got := lookupClientRecord(t, "fox")
	if got.TotalGB != 7 || got.Comment != "ok" {
		t.Fatalf("record after update totalGB=%d comment=%q", got.TotalGB, got.Comment)
	}
}

func TestTproxyInboundSaveMustNotSyncEmptyClients(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	source := []model.Client{{Email: "fox", SubID: "sub-fox", Enable: true, ID: "aaaaaaaa-0000-0000-0000-0000000000aa"}}
	vless := mkInbound(t, 22102, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, vless.Id, source); err != nil {
		t.Fatal(err)
	}
	share := mkInbound(t, 443, model.Tproxy, `{"hostname":"x.example","secret":"000102030405060708090a0b0c0d0e0f"}`)
	rec := lookupClientRecord(t, "fox")
	if _, err := svc.Attach(inboundSvc, rec.Id, []int{share.Id}); err != nil {
		t.Fatal(err)
	}
	if !shareOnlySidecar(model.Tproxy) {
		t.Fatal("tproxy must be share-only: GetClients is empty and SyncInbound would drop attaches")
	}
	clients, err := inboundSvc.GetClients(share)
	if err != nil || len(clients) != 0 {
		t.Fatalf("tproxy GetClients = %d %v", len(clients), err)
	}
	got, err := svc.ListForInbound(nil, share.Id)
	if err != nil || len(got) != 1 || got[0].Email != "fox" {
		t.Fatalf("attach after tproxy save-guard: %+v %v", got, err)
	}
}
