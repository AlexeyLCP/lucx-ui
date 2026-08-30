// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
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
}

// Preview lists unmanaged AWG configs on this host.
func (s *AwgImportService) Preview() AwgImportPreview {
	dismissed, _ := (&SettingService{}).getString(awgImportDismissKey)
	found := awg.Discover(awg.DefaultDiscoverPaths())
	if found == nil {
		found = []awg.ImportCandidate{}
	}
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
	out := make([]AwgImportResult, 0, len(ids))
	for _, id := range ids {
		c, ok := found[id]
		if !ok {
			out = append(out, AwgImportResult{ID: id, Error: "candidate not found"})
			continue
		}
		out = append(out, s.commitOne(userId, c))
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
	created, _, err := s.Inbound.addInbound(built.Inbound, true)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.InboundId = created.Id
	res.Remark = created.Remark
	inst, ok := awg.InstanceFromInbound(created)
	if !ok {
		res.Error = "inbound saved but is not a usable AWG instance"
		return res
	}
	liveName := ""
	if c.Live && c.Backend == "kernel" {
		liveName = c.Ifname
	}
	if err := awg.GetManager().Adopt(inst, liveName, !c.DropOnImport); err != nil {
		logger.Warningf("awg import: adopt %s: %v", c.ID, err)
		res.Error = fmt.Sprintf("saved, adopt failed: %v", err)
		return res
	}
	res.Adopted = true
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
