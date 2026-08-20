package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUpdateAfterShareOnlyAttach(t *testing.T) {
	for _, proto := range []model.Protocol{model.Qwdtt, model.Olcrtc} {
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
			if _, err := svc.Update(inboundSvc, rec.Id, updated); err != nil {
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
	if _, err := svc.Update(inboundSvc, rec.Id, updated); err != nil {
		t.Fatalf("Update qWDTT-only: %v", err)
	}
	got := lookupClientRecord(t, "fox")
	if got.TotalGB != 7 || got.Comment != "ok" {
		t.Fatalf("record after update totalGB=%d comment=%q", got.TotalGB, got.Comment)
	}
}
