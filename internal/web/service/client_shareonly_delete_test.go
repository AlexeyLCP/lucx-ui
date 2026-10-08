// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func countClientRecordsByEmail(t *testing.T, email string) int64 {
	t.Helper()
	var n int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", email).Count(&n).Error; err != nil {
		t.Fatalf("count client records: %v", err)
	}
	return n
}

func countInboundLinks(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := database.GetDB().Table("client_inbounds").Count(&n).Error; err != nil {
		t.Fatalf("count client_inbounds: %v", err)
	}
	return n
}

// Reproduces the tester report: deleting a client from a share-only sidecar
// inbound (qWDTT / CSQTT / Telegram Web Proxy / OpenFlux) failed with
// "invalid clients format in inbound settings" because the deletion still
// expected the client list in settings.clients, while these protocols keep
// clients only in the client_inbounds link table.
func TestDeleteClientOnShareOnlyInbound(t *testing.T) {
	for _, proto := range []model.Protocol{model.Qwdtt, model.Csqtt, model.Openflux, model.Tproxy} {
		t.Run(string(proto), func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			share := mkInbound(t, 57100, proto, `{"remark":"share"}`)
			if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
				Client:     model.Client{Email: "fox", SubID: "sub-fox", Enable: true},
				InboundIds: []int{share.Id},
			}); err != nil {
				t.Fatalf("Create on %s: %v", proto, err)
			}
			rec := lookupClientRecord(t, "fox")

			if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
				t.Fatalf("Delete from %s: %v", proto, err)
			}

			if n := countClientRecordsByEmail(t, "fox"); n != 0 {
				t.Fatalf("client record survived delete (%d rows)", n)
			}
			if got := countInboundLinks(t); got != 0 {
				t.Fatalf("client_inbounds rows after delete = %d, want 0", got)
			}
			if got := mustInboundSettings(t, inboundSvc, share.Id); got != `{"remark":"share"}` {
				t.Fatalf("%s settings mutated by delete: %s", proto, got)
			}
		})
	}
}

// Reproduces the UNIQUE constraint failure: renaming a client whose only home
// is a share-only sidecar inbound used to route through the settings-JSON-based
// UpdateInboundClient, which inserted a second clients row under the new email.
func TestRenameClientOnShareOnlyInbound(t *testing.T) {
	for _, proto := range []model.Protocol{model.Qwdtt, model.Csqtt, model.Openflux, model.Tproxy} {
		t.Run(string(proto), func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			share := mkInbound(t, 57200, proto, `{"remark":"share"}`)
			if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
				Client:     model.Client{Email: "fox", SubID: "sub-fox", Enable: true},
				InboundIds: []int{share.Id},
			}); err != nil {
				t.Fatalf("Create on %s: %v", proto, err)
			}
			rec := lookupClientRecord(t, "fox")

			updated := *rec.ToClient()
			updated.Email = "renamed"
			updated.SubID = rec.SubID
			if _, err := svc.Update(inboundSvc, rec.Id, updated, 0); err != nil {
				t.Fatalf("Update (rename) on %s: %v", proto, err)
			}

			if got := lookupClientRecord(t, "renamed"); got.Id != rec.Id {
				t.Fatalf("renamed record = %+v, want id %d", got, rec.Id)
			}
			if n := countClientRecordsByEmail(t, "fox"); n != 0 {
				t.Fatalf("old email still present (%d rows)", n)
			}
			clients, err := svc.ListForInbound(nil, share.Id)
			if err != nil || len(clients) != 1 || clients[0].Email != "renamed" {
				t.Fatalf("inbound clients after rename = %+v %v", clients, err)
			}
		})
	}
}
