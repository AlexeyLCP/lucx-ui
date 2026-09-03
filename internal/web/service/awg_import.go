// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

const awgImportDismissKey = "awgImportBannerDismissed"

// AwgImportService discovers foreign AWG interfaces and commits them as inbounds.
type AwgImportService struct {
	Inbound InboundService
}

// AwgImportPreview is the GET discover payload.
type AwgImportPreview struct {
	Dismissed  bool                  `json:"dismissed"`
	Candidates []awg.ImportCandidate `json:"candidates"`
}

// AwgImportResult is one committed interface.
type AwgImportResult struct {
	ID          string `json:"id"`
	InboundId   int    `json:"inboundId"`
	Remark      string `json:"remark"`
	Clients     int    `json:"clients"`
	MissingKeys int    `json:"missingKeys"`
	Adopted     bool   `json:"adopted"`
	Stopped     bool   `json:"stopped"`
	Error       string `json:"error,omitempty"`
	Warning     string `json:"warning,omitempty" example:"saved, I-fields will not be applied"`
}

// Preview lists unmanaged AWG configs on this host.
func (s *AwgImportService) Preview() AwgImportPreview {
	dismissed, _ := (&SettingService{}).getString(awgImportDismissKey)
	found := awg.Discover(awg.DefaultDiscoverPaths())
	if found == nil {
		found = []awg.ImportCandidate{}
	}
	found = append(found, tproxyCandidates()...)
	return AwgImportPreview{
		Dismissed:  dismissed == "1",
		Candidates: found,
	}
}

// Dismiss hides the inbound-page banner until the next explicit menu open.
func (s *AwgImportService) Dismiss() error {
	return (&SettingService{}).setString(awgImportDismissKey, "1")
}

// Commit imports the selected candidate IDs.
func (s *AwgImportService) Commit(userId int, ids []string) []AwgImportResult {
	found := map[string]awg.ImportCandidate{}
	for _, c := range awg.Discover(awg.DefaultDiscoverPaths()) {
		found[c.ID] = c
	}
	for _, c := range tproxyCandidates() {
		found[c.ID] = c
	}
	out := make([]AwgImportResult, 0, len(ids))
	for _, id := range ids {
		c, ok := found[id]
		if !ok {
			out = append(out, AwgImportResult{ID: id, Error: "candidate not found"})
			continue
		}
		switch c.Source {
		case awg.ImportSourceOutbound:
			out = append(out, s.commitOutbound(c))
		case awg.ImportSourceTproxy:
			out = append(out, s.commitTproxy(userId, c))
		default:
			out = append(out, s.commitOne(userId, c))
		}
	}
	return out
}

func (s *AwgImportService) reservedEmails() map[string]struct{} {
	used := map[string]struct{}{}
	emails, err := s.Inbound.GetAllEmails()
	if err != nil {
		return used
	}
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			used[e] = struct{}{}
		}
	}
	return used
}

// addImportedInbound saves a discovered server, returning the operator note an
// over-budget I-set earns: it stays stored, but no renderer will emit it.
func (s *AwgImportService) addImportedInbound(ib *model.Inbound) (*model.Inbound, string, error) {
	var notes []string
	if measured := AwgIFieldBudgetWarning(ib.Settings); measured != "" {
		notes = append(notes, "saved, I-fields will not be applied: "+measured)
	}
	if lost := AwgIFieldExportNote(ib.Settings); lost != "" {
		notes = append(notes, "saved, these will not reach a client: "+lost)
	}
	warn := strings.Join(notes, "; ")
	created, _, err := s.Inbound.addInbound(ib, true)
	return created, warn, err
}

func (s *AwgImportService) commitOne(userId int, c awg.ImportCandidate) AwgImportResult {
	res := AwgImportResult{ID: c.ID, Clients: c.PeerCount}
	if _, err := awg.BackupImportSources(c); err != nil {
		res.Error = "backup failed: " + err.Error()
		return res
	}
	built, err := awg.BuildInbound(c, userId, s.reservedEmails())
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.MissingKeys = built.MissingKeys
	wantEnable := built.Inbound.Enable
	built.Inbound.Enable = false
	created, iFieldWarn, err := s.addImportedInbound(built.Inbound)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.InboundId = created.Id
	res.Remark = created.Remark
	res.Warning = iFieldWarn
	inst, ok := awg.InstanceFromInbound(created)
	if !ok {
		if delErr := s.rollbackImported(created.Id); delErr != nil {
			res.Error = "inbound saved but is not a usable AWG instance; rollback failed: " + delErr.Error()
			return res
		}
		res.InboundId = 0
		res.Error = "inbound rolled back: not a usable AWG instance"
		return res
	}
	liveName := ""
	if c.Live && c.Backend == "kernel" {
		liveName = c.Ifname
	}
	if err := awg.GetManager().Adopt(inst, liveName, !c.DropOnImport); err != nil {
		logger.Warningf("awg import: adopt %s: %v", c.ID, err)
		if delErr := s.rollbackImported(created.Id); delErr != nil {
			res.Error = fmt.Sprintf("saved, adopt failed: %v; rollback failed: %v", err, delErr)
			return res
		}
		res.InboundId = 0
		res.Error = "adopt failed, inbound rolled back: " + err.Error()
		return res
	}
	res.Adopted = true
	if wantEnable {
		if _, err := s.Inbound.SetInboundEnable(created.Id, true); err != nil {
			logger.Warningf("awg import: enable %d after adopt: %v", created.Id, err)
		}
	}
	if c.ConfPath != "" && !awg.ConfigPathIsManaged(c.ConfPath) {
		if err := awg.BackupForeignConf(c.ConfPath); err != nil {
			logger.Warningf("awg import: backup %s: %v", c.ConfPath, err)
		}
	}
	if c.StopTarget != "" {
		if err := awg.StopImportSource(c); err != nil {
			logger.Warningf("awg import: stop %s: %v", c.StopTarget, err)
			if res.Error == "" {
				res.Error = "saved, stop old source failed: " + err.Error()
			}
		} else {
			res.Stopped = true
			if c.DropOnImport {
				if _, err := s.Inbound.SetInboundEnable(created.Id, true); err != nil {
					logger.Warningf("awg import: enable %d after stop: %v", created.Id, err)
				}
			}
		}
	}
	return res
}

func (s *AwgImportService) rollbackImported(id int) error {
	_, err := s.Inbound.DelInbound(id)
	if err != nil {
		logger.Warningf("awg import: rollback inbound %d: %v", id, err)
	}
	return err
}

func tproxyCandidates() []awg.ImportCandidate {
	var out []awg.ImportCandidate
	for _, t := range tunnel.DiscoverTproxy() {
		cfg := t.Config()
		raw, err := json.Marshal(cfg)
		if err != nil {
			continue
		}
		out = append(out, awg.ImportCandidate{
			ID:         awg.ImportSourceTproxy + ":" + t.Hostname,
			Source:     awg.ImportSourceTproxy,
			Ifname:     "tproxy-server",
			ConfPath:   t.ConfPath,
			Live:       t.Live,
			Port:       443,
			Address:    t.Hostname,
			Warning:    "Existing nginx/Caddy keeps TLS. Panel will not bind :443 or start another tproxy-server.",
			PeerCount:  1,
			NamedPeers: 1,
			KeysFound:  1,
			ConfText:   string(raw),
			Peers: []awg.ImportPeer{{
				Email:     t.Hostname,
				PublicKey: t.Listen,
				HasKey:    true,
			}},
		})
	}
	return out
}

func (s *AwgImportService) commitOutbound(c awg.ImportCandidate) AwgImportResult {
	res := AwgImportResult{ID: c.ID, Clients: 1}
	built, err := awg.BuildOutbound(c)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	outSvc := &AwgOutboundService{}
	created, err := outSvc.AddOutbound(built)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.InboundId = created.Id
	res.Remark = created.Remark
	ci, ok := awg.ClientInstanceFromOutbound(created)
	if !ok {
		_ = outSvc.DelOutbound(created.Id)
		res.InboundId = 0
		res.Error = "outbound rolled back: not a usable AWG client"
		return res
	}
	liveName := ""
	if c.Live {
		liveName = c.Ifname
	}
	if err := awg.GetManager().AdoptClient(ci, liveName); err != nil {
		logger.Warningf("awg import: adopt exit %s: %v", c.ID, err)
		if delErr := outSvc.DelOutbound(created.Id); delErr != nil {
			res.Error = fmt.Sprintf("saved, adopt failed: %v; rollback failed: %v", err, delErr)
			return res
		}
		res.InboundId = 0
		res.Error = "adopt failed, outbound rolled back: " + err.Error()
		return res
	}
	res.Adopted = true
	if err := outSvc.SetOutboundEnable(created.Id, true); err != nil {
		logger.Warningf("awg import: enable outbound %d: %v", created.Id, err)
	}
	if c.ConfPath != "" && !awg.ConfigPathIsManaged(c.ConfPath) {
		_ = awg.BackupForeignConf(c.ConfPath)
	}
	return res
}

func (s *AwgImportService) commitTproxy(userId int, c awg.ImportCandidate) AwgImportResult {
	res := AwgImportResult{ID: c.ID, Clients: 1}
	var cfg tunnel.TproxyConfig
	if err := json.Unmarshal([]byte(c.ConfText), &cfg); err != nil {
		res.Error = "tproxy settings missing"
		return res
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	ib := &model.Inbound{
		UserId:         userId,
		Port:           443,
		Protocol:       model.Tproxy,
		Remark:         "imported-" + cfg.Hostname,
		Enable:         true,
		Settings:       string(raw),
		StreamSettings: `{}`,
		Sniffing:       `{}`,
	}
	created, _, err := s.Inbound.addInbound(ib, true)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.InboundId = created.Id
	res.Remark = created.Remark
	res.Adopted = true
	return res
}
