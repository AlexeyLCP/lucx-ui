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

// migrateAwgVersion backfills the awgVersion field on pre-lucx.50 AWG inbounds
// and outbounds and prunes a stale headerProtectionKey that would otherwise
// break reconcile. It does two things in one pass per row:
//
//  1. If awgVersion is absent, set it to "2" (the safe default — matches every
//     shipped LucX-UI release before AWG3, emits no HeaderProtectionKey, and is
//     accepted by the current kernel module without S-range constraints).
//  2. If awgVersion != "3" and headerProtectionKey is non-empty, clear it.
//
// Step 2 fixes the lucx.47 regression: the generate-obfuscation endpoint handed
// the inbound form a freshly generated key on every click, and at the time the
// master kernel module had no parser for `HeaderProtectionKey`, so the rendered
// .conf made `awg setconf` abort with "Line unrecognized" + "Configuration
// parsing error"; awg-quick deleted the half-built interface and reconcile
// failed every 10 seconds. Operators who pressed the button and saved carry a
// poisoned settings blob. As of lucx.50 the upstream module (v3.0.20260731) +
// tools (v3.0.20260730) parse the field, but only a version-"3" inbound opts
// into writing it — so a key stored on a v1/v2 (or pre-version) inbound is
// either the lucx.47 poison or a value that would never reach the .conf anyway.
// Either way it must go; a v3 inbound's hand-typed key is preserved.
//
// Why clearing is safe rather than lossy: no released LucX-UI version before
// lucx.50 ever wrote the field to the .conf, so a stored value cannot be in
// productive use — it can only describe a configuration that refused to start.
// The key is a symmetric secret that is regenerated, not derived, so nothing
// else references it.
//
// Idempotent: rows already carrying the right version and an empty/absent key
// take zero writes, so a fresh DB and an already-migrated DB are both no-ops.
func migrateAwgVersion() error {
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		return err
	}
	for _, ib := range inbounds {
		cleaned, changed, reason := normalizeAwgSettings(ib.Settings)
		if !changed {
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", cleaned).Error; err != nil {
			log.Printf("[LUCX-AWG] migration: failed to update AWG inbound %d: %v", ib.Id, err)
			continue
		}
		log.Printf("[LUCX-AWG] migration: AWG inbound %d %s", ib.Id, reason)
	}

	var outbounds []model.AwgOutbound
	if err := db.Find(&outbounds).Error; err != nil {
		return err
	}
	for _, o := range outbounds {
		cleaned, changed, reason := normalizeAwgSettings(o.Settings)
		if !changed {
			continue
		}
		if err := db.Model(&model.AwgOutbound{}).Where("id = ?", o.Id).Update("settings", cleaned).Error; err != nil {
			log.Printf("[LUCX-AWG] migration: failed to update AWG outbound %d: %v", o.Id, err)
			continue
		}
		log.Printf("[LUCX-AWG] migration: AWG outbound %d %s", o.Id, reason)
	}
	return nil
}

// normalizeAwgSettings applies the migrateAwgVersion rules to one settings blob
// and returns the cleaned JSON, whether anything changed, and a short human
// reason for the log line. Returns changed=false for invalid JSON.
func normalizeAwgSettings(settings string) (string, bool, string) {
	var m map[string]any
	if err := json.Unmarshal([]byte(settings), &m); err != nil {
		return settings, false, ""
	}
	changed := false
	var reasons []string

	// Backfill awgVersion = "2" when absent. The renderer normalizes "" to "2"
	// at runtime too, but persisting it keeps the stored settings explicit so
	// the clients page sees a real ceiling without relying on the default.
	v, hasVersion := m["awgVersion"]
	versionStr, _ := v.(string)
	if !hasVersion || (versionStr != "1.5" && versionStr != "2" && versionStr != "3") {
		m["awgVersion"] = "2"
		versionStr = "2"
		changed = true
		reasons = append(reasons, "backfilled awgVersion=\"2\"")
	}

	// Prune a non-empty headerProtectionKey on anything that is not version
	// "3". On v1/v2 the key never reaches the .conf, so a stored value can only
	// be the lucx.47 poison; keeping it would be confusing and risks a future
	// version bump silently writing a key the operator did not intend.
	if versionStr != "3" {
		if hpk, ok := m["headerProtectionKey"].(string); ok && hpk != "" {
			m["headerProtectionKey"] = ""
			changed = true
			reasons = append(reasons, "cleared stale headerProtectionKey (not AWG3)")
		}
	}
	if !changed {
		return settings, false, ""
	}
	out, err := json.Marshal(m)
	if err != nil {
		return settings, false, ""
	}
	return string(out), true, strings.Join(reasons, "; ")
}
