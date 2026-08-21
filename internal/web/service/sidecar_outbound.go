// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type SidecarOutboundService struct{}

func validSidecarProtocol(p string) bool {
	switch p {
	case tunnel.SidecarProtocolNaive, tunnel.SidecarProtocolMieru, tunnel.SidecarProtocolTrustTunnel:
		return true
	default:
		return false
	}
}

func (s *SidecarOutboundService) AddOutbound(o *model.SidecarOutbound) (*model.SidecarOutbound, error) {
	o.Protocol = strings.TrimSpace(o.Protocol)
	if !validSidecarProtocol(o.Protocol) {
		return nil, common.NewError("sidecar-outbound: protocol must be naive, mieru, or trusttunnel")
	}
	tag := strings.TrimSpace(o.Tag)
	if tag != "" {
		o.Tag = tag
		if err := checkTagUnique(o.Tag, 0, 0); err != nil {
			return nil, err
		}
	} else {
		o.Tag = ""
	}
	settings, err := s.normalizeSettings(o, tunnel.SidecarSettings{})
	if err != nil {
		return nil, err
	}
	o.Settings = tunnel.SettingsJSON(settings)
	db := database.GetDB()
	if err := db.Create(o).Error; err != nil {
		return nil, err
	}
	if o.Tag == "" {
		o.Tag = tunnel.DefaultSidecarTag(o.Protocol, o.Id)
		if err := db.Model(o).Update("tag", o.Tag).Error; err != nil {
			return nil, err
		}
		if err := checkTagUnique(o.Tag, 0, o.Id); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (s *SidecarOutboundService) DelOutbound(id int) error {
	return database.GetDB().Delete(&model.SidecarOutbound{}, id).Error
}

func (s *SidecarOutboundService) UpdateOutbound(o *model.SidecarOutbound) error {
	if !validSidecarProtocol(o.Protocol) {
		return common.NewError("sidecar-outbound: protocol must be naive, mieru, or trusttunnel")
	}
	if err := checkTagUnique(o.Tag, 0, o.Id); err != nil {
		return err
	}
	old, err := s.GetOutbound(o.Id)
	if err != nil {
		return err
	}
	var prev tunnel.SidecarSettings
	_ = json.Unmarshal([]byte(old.Settings), &prev)
	settings, err := s.normalizeSettings(o, prev)
	if err != nil {
		return err
	}
	o.Settings = tunnel.SettingsJSON(settings)
	return database.GetDB().Save(o).Error
}

func (s *SidecarOutboundService) SetOutboundEnable(id int, enable bool) error {
	return database.GetDB().Model(&model.SidecarOutbound{}).Where("id = ?", id).Update("enable", enable).Error
}

func (s *SidecarOutboundService) GetOutbounds() ([]*model.SidecarOutbound, error) {
	var out []*model.SidecarOutbound
	err := database.GetDB().Order("id ASC").Find(&out).Error
	return out, err
}

func (s *SidecarOutboundService) GetOutbound(id int) (*model.SidecarOutbound, error) {
	o := &model.SidecarOutbound{}
	if err := database.GetDB().First(o, id).Error; err != nil {
		return nil, err
	}
	return o, nil
}

func (s *SidecarOutboundService) ActiveOutboundTags() ([]string, error) {
	var tags []string
	err := database.GetDB().Model(&model.SidecarOutbound{}).
		Where("enable = ?", true).
		Order("id ASC").
		Pluck("tag", &tags).Error
	return tags, err
}

func (s *SidecarOutboundService) ParseLink(text string) (string, tunnel.SidecarSettings, error) {
	return tunnel.ParseSidecarLink(text)
}

func (s *SidecarOutboundService) Reconcile() {
	rows, err := s.GetOutbounds()
	if err != nil {
		logger.Warning("sidecar-outbound: list failed:", err)
		return
	}
	var naive, mieru, tt []tunnel.Instance
	m := tunnel.GetManager()
	for _, o := range rows {
		if !o.Enable {
			m.Remove(tunnel.SidecarManageKey(o.Protocol, o.Id))
			continue
		}
		inst, ok := tunnel.InstanceFromSidecarOutbound(o)
		if !ok {
			continue
		}
		switch o.Protocol {
		case tunnel.SidecarProtocolNaive:
			naive = append(naive, inst)
		case tunnel.SidecarProtocolMieru:
			mieru = append(mieru, inst)
		case tunnel.SidecarProtocolTrustTunnel:
			tt = append(tt, inst)
		}
	}
	m.ReconcileWanted(tunnel.NaiveClient, "naiveout-", "", naive)
	m.ReconcileWanted(tunnel.MieruClient, "mieruout-", "", mieru)
	m.ReconcileWanted(tunnel.TrustTunnelClient, "ttout-", "", tt)
}

func (s *SidecarOutboundService) normalizeSettings(o *model.SidecarOutbound, prev tunnel.SidecarSettings) (tunnel.SidecarSettings, error) {
	var snew tunnel.SidecarSettings
	if strings.TrimSpace(o.Settings) != "" {
		if err := json.Unmarshal([]byte(o.Settings), &snew); err != nil {
			return snew, common.NewError("sidecar-outbound: settings JSON: ", err)
		}
	}
	if link := strings.TrimSpace(snew.Link); link != "" {
		proto, parsed, err := tunnel.ParseSidecarLink(link)
		if err == nil {
			socks := snew.SocksPort
			parsed.SocksPort = socks
			if parsed.Host == "" {
				parsed.Host = snew.Host
			}
			snew = parsed
			if proto != "" && o.Protocol == "" {
				o.Protocol = proto
			}
		}
	}
	if prev.SocksPort > 0 {
		snew.SocksPort = prev.SocksPort
	}
	if snew.SocksPort <= 0 {
		port, err := mtproto.FreeLocalPort()
		if err != nil {
			return snew, common.NewError("sidecar-outbound: allocate SOCKS port: ", err)
		}
		snew.SocksPort = port
	}
	if !snew.Valid() {
		return snew, common.NewError("sidecar-outbound: host, user and socks port are required")
	}
	return snew, nil
}

func (s *SidecarOutboundService) DeleteBinary(protocol string) error {
	core, ok := tunnel.SidecarCore(protocol)
	if !ok {
		return common.NewError("sidecar-outbound: unknown protocol")
	}
	path := core.BinaryPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *SidecarOutboundService) BinaryStatus() map[string]any {
	out := map[string]any{}
	for _, p := range []string{tunnel.SidecarProtocolNaive, tunnel.SidecarProtocolMieru, tunnel.SidecarProtocolTrustTunnel} {
		core, _ := tunnel.SidecarCore(p)
		out[p] = map[string]any{
			"exists": tunnel.BinaryExists(core),
			"path":   core.BinaryPath(),
			"name":   core.BinaryName(),
		}
	}
	return out
}
