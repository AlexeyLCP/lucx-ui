// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

type GatewayApplyRequest struct {
	Selected   []int `json:"selected"`
	Steal      []int `json:"steal"`
	PublicHost string `json:"publicHost"`
}

type GatewayPreviewResult struct {
	Applied    bool                `json:"applied"`
	PublicHost string              `json:"publicHost"`
	Rows       []tunnel.PreviewRow `json:"rows"`
}

func (s *InboundService) GatewayPreview(gatewayID int, publicHost string) (*GatewayPreviewResult, error) {
	gw, others, err := s.gatewayAndOthers(gatewayID)
	if err != nil {
		return nil, err
	}
	cfg, _ := tunnel.GatewayConfigFromInbound(gw)
	if publicHost == "" {
		publicHost = cfg.PublicHost
	}
	return &GatewayPreviewResult{
		Applied:    cfg.Applied(),
		PublicHost: publicHost,
		Rows:       tunnel.BuildPreview(gw.Port, publicHost, others),
	}, nil
}

func (s *InboundService) GatewayApply(gatewayID int, req GatewayApplyRequest) error {
	gw, others, err := s.gatewayAndOthers(gatewayID)
	if err != nil {
		return err
	}
	cfg, _ := tunnel.GatewayConfigFromInbound(gw)
	if cfg.Applied() {
		return common.NewError("gateway: revert first")
	}
	byID := map[int]*model.Inbound{}
	for _, o := range others {
		byID[o.Id] = o
	}
	rows := tunnel.BuildPreview(gw.Port, req.PublicHost, others)
	selected := map[int]bool{}
	for _, id := range req.Selected {
		selected[id] = true
	}
	steal := map[int]bool{}
	for _, id := range req.Steal {
		steal[id] = true
	}
	if len(selected) == 0 {
		return common.NewError("gateway: nothing selected")
	}
	var snap []tunnel.GatewaySnapshotRow
	db := database.GetDB()
	for _, row := range rows {
		if !selected[row.InboundID] {
			continue
		}
		ib := byID[row.InboundID]
		if ib == nil {
			continue
		}
		sr := tunnel.GatewaySnapshotRow{InboundID: ib.Id, Listen: ib.Listen, Port: ib.Port, StreamSettings: ib.StreamSettings}
		ib.Listen = row.NewListen
		ib.Port = row.NewPort
		if steal[row.InboundID] && row.StealDest != "" {
			ib.StreamSettings = tunnel.SetRealityDest(ib.StreamSettings, row.StealDest)
		}
		if err := db.Model(ib).Select("listen", "port", "stream_settings").Updates(ib).Error; err != nil {
			return err
		}
		if row.HostAddress != "" && row.Class == tunnel.ClassPassthrough {
			h := model.Host{
				GroupId:   random.NumLower(16),
				InboundId: ib.Id,
				Remark:    "gateway",
				Address:   row.HostAddress,
				Port:      row.HostPort,
				Security:  "same",
			}
			if err := db.Create(&h).Error; err != nil {
				return err
			}
			sr.HostID = h.Id
		}
		snap = append(snap, sr)
	}
	cfg.PublicHost = req.PublicHost
	cfg.Snapshot = snap
	cfg.Routes = tunnel.RoutesFromPreview(rows, selected)
	cfg.Enabled = true
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	gw.Settings = string(body)
	gw.Enable = true
	if gw.Port <= 0 {
		gw.Port = 443
	}
	if err := db.Model(gw).Select("settings", "enable", "port").Updates(gw).Error; err != nil {
		return err
	}
	s.ensureGatewayRuntime(gw, others)
	return nil
}

func (s *InboundService) GatewayRevert(gatewayID int) error {
	gw, others, err := s.gatewayAndOthers(gatewayID)
	if err != nil {
		return err
	}
	cfg, _ := tunnel.GatewayConfigFromInbound(gw)
	if !cfg.Applied() {
		return common.NewError("gateway: nothing to revert")
	}
	tunnel.GetManager().Remove(tunnel.GatewayKey(gw.Id))
	db := database.GetDB()
	byID := map[int]*model.Inbound{}
	for _, o := range others {
		byID[o.Id] = o
	}
	for _, sr := range cfg.Snapshot {
		if sr.HostID > 0 {
			_ = db.Delete(&model.Host{}, sr.HostID).Error
		}
		ib := byID[sr.InboundID]
		if ib == nil {
			continue
		}
		ib.Listen = sr.Listen
		ib.Port = sr.Port
		if sr.StreamSettings != "" {
			ib.StreamSettings = sr.StreamSettings
		}
		_ = db.Model(ib).Select("listen", "port", "stream_settings").Updates(ib).Error
	}
	cfg.Snapshot = nil
	cfg.Routes = nil
	cfg.Enabled = false
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	gw.Settings = string(body)
	gw.Enable = false
	if err := db.Model(gw).Select("settings", "enable").Updates(gw).Error; err != nil {
		return err
	}
	s.ensureGatewayRuntime(gw, others)
	return nil
}

func (s *InboundService) gatewayAndOthers(id int) (*model.Inbound, []*model.Inbound, error) {
	all, err := s.GetAllInbounds()
	if err != nil {
		return nil, nil, err
	}
	var gw *model.Inbound
	var others []*model.Inbound
	for _, ib := range all {
		if ib == nil || ib.NodeID != nil {
			continue
		}
		if ib.Id == id {
			gw = ib
			continue
		}
		others = append(others, ib)
	}
	if gw == nil || gw.Protocol != model.Gateway {
		return nil, nil, common.NewError("gateway inbound not found")
	}
	return gw, others, nil
}

func (s *InboundService) ensureGatewayRuntime(gw *model.Inbound, others []*model.Inbound) {
	all := append([]*model.Inbound{gw}, others...)
	cert, key := panelCertFiles()
	mgr := tunnel.GetManager()
	if inst, ok := tunnel.GatewayInstanceFromInbound(gw, others); ok {
		_ = mgr.Ensure(inst)
	}
	for _, ib := range others {
		if ib == nil || ib.Protocol != model.Cover {
			continue
		}
		if inst, ok := tunnel.CoverInstanceFromInbound(ib, all, nil, cert, key); ok {
			_ = mgr.Ensure(inst)
		}
	}
}
