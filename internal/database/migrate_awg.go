// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// pruneLegacyAwgHiddenChildren removes the hidden SOCKS5 child inbounds the
// old LucX-UI (pre-sidecar) created for each AWG inbound. The old architecture
// stored them as separate rows with tag="awg-hidden-<id>" and protocol="socks";
// the new sidecar architecture routes AWG traffic through an injected Xray TUN
// inbound (injectAwgEgress), so these child rows are stale and would otherwise
// linger in the inbounds table as orphan SOCKS5 listeners on loopback ports.
//
// Idempotent: a v3.3.1-fresh DB has no such rows, and a previously-migrated DB
// has them already gone, so the DELETE matches zero rows and returns nil.
// Safe: only touches rows whose tag matches the "awg-hidden-" prefix, so a
// user-created SOCKS5 inbound with a different tag is never affected.
func pruneLegacyAwgHiddenChildren() error {
	var stale []model.Inbound
	// The old schema stored these as protocol="socks" with tag="awg-hidden-*".
	// We match on the tag prefix (stable, set only by the old LucX-UI) rather
	// than ParentID, which does not exist in the v3.3.1 schema.
	if err := db.Where("tag LIKE 'awg-hidden-%'").Find(&stale).Error; err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	log.Printf("[LUCX-AWG] migration: removing %d legacy hidden SOCKS5 child inbound(s) from old architecture", len(stale))
	for _, ib := range stale {
		// Log each removal so the operator can audit what was pruned.
		log.Printf("[LUCX-AWG] migration: deleting stale child inbound id=%d tag=%s protocol=%s port=%d",
			ib.Id, ib.Tag, ib.Protocol, ib.Port)
	}
	// Delete in one statement; the child inbounds had no client_traffics of
	// their own (traffic was accounted on the parent AWG inbound), so there
	// are no FK rows to clean up.
	if err := db.Where("tag LIKE 'awg-hidden-%'").Delete(&model.Inbound{}).Error; err != nil {
		return err
	}
	// Also strip the now-orphan hiddenSOCKSPort / hiddenInboundTag fields the
	// old architecture wrote into the parent AWG inbound's settings JSON, so
	// the new sidecar does not read stale port/tag values. We do this row by
	// row because SQLite has no JSON mutation function; the Go-side decode/
	// re-encode keeps it dialect-agnostic.
	var awgInbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&awgInbounds).Error; err != nil {
		return err
	}
	for _, ib := range awgInbounds {
		if !strings.Contains(ib.Settings, "hiddenSOCKSPort") && !strings.Contains(ib.Settings, "hiddenInboundTag") {
			continue
		}
		cleaned := stripHiddenKeys(ib.Settings)
		if cleaned == ib.Settings {
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", cleaned).Error; err != nil {
			log.Printf("[LUCX-AWG] migration: failed to clean settings for inbound %d: %v", ib.Id, err)
		}
	}
	return nil
}

// stripHiddenKeys removes the hiddenSOCKSPort and hiddenInboundTag keys the
// old LucX-UI wrote into a parent AWG inbound's settings JSON. Returns the
// input unchanged when it is not valid JSON or contains neither key.
func stripHiddenKeys(settings string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(settings), &m); err != nil {
		return settings
	}
	changed := false
	for _, k := range []string{"hiddenSOCKSPort", "hiddenInboundTag"} {
		if _, ok := m[k]; ok {
			delete(m, k)
			changed = true
		}
	}
	if !changed {
		return settings
	}
	out, err := json.Marshal(m)
	if err != nil {
		return settings
	}
	return string(out)
}