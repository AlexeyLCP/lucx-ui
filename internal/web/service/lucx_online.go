// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// sidecarOnlineProc holds local/node online maps when Xray is not running, so
// sidecar jobs and node sync still have a place to stamp emails/tags.
var sidecarOnlineProc = xray.NewProcess(nil)

func onlineProcess() *xray.Process {
	if p := currentXrayProcess(); p != nil {
		return p
	}
	return sidecarOnlineProc
}

func shareOnlySidecar(p model.Protocol) bool {
	return p == model.Qwdtt || p == model.Csqtt || p == model.Olcrtc || p == model.Openflux || p == model.Tproxy
}

type shareOnlySlimClient struct {
	InboundID int
	Email     string
	Enable    bool
	Comment   string
}

func (s *InboundService) injectShareOnlySlimClients(db *gorm.DB, inbounds []*model.Inbound) {
	if db == nil || len(inbounds) == 0 {
		return
	}
	ids := make([]int, 0)
	want := make(map[int]*model.Inbound)
	for _, ib := range inbounds {
		if ib == nil || !shareOnlySidecar(ib.Protocol) {
			continue
		}
		ids = append(ids, ib.Id)
		want[ib.Id] = ib
	}
	if len(ids) == 0 {
		return
	}
	var rows []shareOnlySlimClient
	err := db.Table("client_inbounds").
		Select("client_inbounds.inbound_id as inbound_id, clients.email as email, clients.enable as enable, clients.comment as comment").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("client_inbounds.inbound_id IN ?", ids).
		Scan(&rows).Error
	if err != nil {
		return
	}
	byID := make(map[int][]shareOnlySlimClient, len(ids))
	for _, row := range rows {
		byID[row.InboundID] = append(byID[row.InboundID], row)
	}
	for id, ib := range want {
		ib.Settings = mergeShareOnlySlimClients(ib.Settings, byID[id])
	}
}

func mergeShareOnlySlimClients(settings string, clients []shareOnlySlimClient) string {
	raw := map[string]any{}
	if strings.TrimSpace(settings) != "" {
		_ = json.Unmarshal([]byte(settings), &raw)
	}
	slim := make([]any, 0, len(clients))
	for _, c := range clients {
		email := strings.TrimSpace(c.Email)
		if email == "" {
			continue
		}
		row := map[string]any{"email": email, "enable": c.Enable}
		if comment := strings.TrimSpace(c.Comment); comment != "" {
			row["comment"] = comment
		}
		slim = append(slim, row)
	}
	raw["clients"] = slim
	out, err := json.Marshal(raw)
	if err != nil {
		return settings
	}
	return string(out)
}

// detachShareOnlyClient removes emails from a share-only sidecar inbound:
// the client_inbounds links only, plus the stat/IP purge for emails that
// survive on no other inbound. These cores authenticate one shared secret,
// not a per-client list, so there is no settings.clients blob to rewrite and
// no Xray user to remove — the tunnel reconcile job owns the runtime. The
// caller holds the inbound lock (lockInbound already taken on every path
// that routes here).
func (s *ClientService) detachShareOnlyClient(inboundSvc *InboundService, inboundId int, emails []string, keepTraffic bool) error {
	clean := make([]string, 0, len(emails))
	for _, e := range emails {
		if e = strings.TrimSpace(e); e != "" {
			clean = append(clean, e)
		}
	}
	if len(clean) == 0 {
		return nil
	}

	var shared map[string]bool
	if !keepTraffic {
		var err error
		shared, err = inboundSvc.emailsUsedByOtherInbounds(clean, inboundId)
		if err != nil {
			return err
		}
	}

	return runSerializedTx(func(tx *gorm.DB) error {
		if !keepTraffic {
			for _, email := range clean {
				if shared[strings.ToLower(email)] {
					continue
				}
				if e := inboundSvc.DelClientIPs(tx, email); e != nil {
					logger.Error("Error in delete client IPs")
					return e
				}
				if e := inboundSvc.DelClientStat(tx, email); e != nil {
					logger.Error("Delete stats Data Error")
					return e
				}
			}
		}
		return s.ApplyInboundClientDelta(tx, inboundId, nil, clean)
	})
}
