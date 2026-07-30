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

// pruneAwgHeaderProtectionKey clears a non-empty headerProtectionKey from AWG
// inbound and AWG outbound settings.
//
// Why this is needed: lucx.47 added the field as AWG3 forward-compat, and the
// generate-obfuscation endpoint handed the inbound form a freshly generated key
// on every click. The current master amneziawg kernel module has no parser for
// `HeaderProtectionKey`, so the rendered .conf made `awg setconf` abort with
// "Line unrecognized" + "Configuration parsing error"; awg-quick then deleted
// the half-built interface and reconcile failed every 10 seconds, taking the
// tunnel down. Operators who pressed the button and saved carry a poisoned
// settings blob, so shipping the renderer fix alone would leave them broken
// until they hand-edited the field.
//
// Why clearing is safe rather than lossy: no released kernel module reads the
// field, so a stored value cannot be in productive use — it can only describe a
// configuration that refuses to start. The key is a symmetric secret that is
// regenerated, not derived, so nothing else references it. Once feat/awg3 lands
// in the master module the field can be populated again (the schema keeps it).
//
// Idempotent: rows without the key, with an empty key, or with unparsable
// settings are left untouched, so a fresh DB and an already-migrated DB both
// take zero writes.
func pruneAwgHeaderProtectionKey() error {
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		return err
	}
	for _, ib := range inbounds {
		// Cheap string pre-filter so untouched rows never hit the JSON decoder.
		if !strings.Contains(ib.Settings, "headerProtectionKey") {
			continue
		}
		cleaned, changed := stripHeaderProtectionKey(ib.Settings)
		if !changed {
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", cleaned).Error; err != nil {
			log.Printf("[LUCX-AWG] migration: failed to clear headerProtectionKey for inbound %d: %v", ib.Id, err)
			continue
		}
		log.Printf("[LUCX-AWG] migration: cleared headerProtectionKey from AWG inbound %d (unsupported by the current kernel module, broke awg setconf)", ib.Id)
	}

	var outbounds []model.AwgOutbound
	if err := db.Find(&outbounds).Error; err != nil {
		return err
	}
	for _, o := range outbounds {
		if !strings.Contains(o.Settings, "headerProtectionKey") {
			continue
		}
		cleaned, changed := stripHeaderProtectionKey(o.Settings)
		if !changed {
			continue
		}
		if err := db.Model(&model.AwgOutbound{}).Where("id = ?", o.Id).Update("settings", cleaned).Error; err != nil {
			log.Printf("[LUCX-AWG] migration: failed to clear headerProtectionKey for awg outbound %d: %v", o.Id, err)
			continue
		}
		log.Printf("[LUCX-AWG] migration: cleared headerProtectionKey from AWG outbound %d (unsupported by the current kernel module, broke awg setconf)", o.Id)
	}
	return nil
}

// stripHeaderProtectionKey blanks a non-empty headerProtectionKey in a settings
// JSON blob, reporting whether anything changed. The key is set to "" rather
// than deleted so the shape the frontend Zod schema expects is preserved (it
// defaults the field to an empty string). Returns changed=false for invalid
// JSON, a missing key, or a key that is already empty.
func stripHeaderProtectionKey(settings string) (string, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(settings), &m); err != nil {
		return settings, false
	}
	v, ok := m["headerProtectionKey"]
	if !ok {
		return settings, false
	}
	if s, isString := v.(string); isString && s == "" {
		return settings, false
	}
	m["headerProtectionKey"] = ""
	out, err := json.Marshal(m)
	if err != nil {
		return settings, false
	}
	return string(out), true
}
