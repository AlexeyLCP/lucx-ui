// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"log"
	"slices"
	"strings"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// repairClobberedTunnelFields undoes the damage every web update did until
// lucx.197: `x-ui migrate` walked the KEYLESS inbounds (vless/trojan/…), parsed
// their settings.clients[] and merged the result into the clients table. Those
// entries carry a stale copy of the tunnel keypair, PSK and address of any
// identity that also belongs to an AWG/WireGuard inbound, and the merge takes a
// non-empty incoming value — so the copy won. The subscription .conf is built
// from the client record, so the operator hands out a config whose key no longer
// matches the peer the kernel actually holds, while the tunnel itself keeps
// working: the interface is built from the AWG inbound's own settings, which
// this bug never touched. That is why it goes unnoticed until someone
// re-downloads their config.
//
// The repair is deliberately narrow. A field is restored only when the record
// holds EXACTLY what a keyless inbound stores for that email and something
// different from what the tunnel inbound stores. That pair of conditions is the
// signature of the clobber and nothing else: a value the operator set by hand,
// a partially repaired install, or an identity with no keyless attachment all
// fail it and are left alone. Where two tunnel inbounds disagree about a field
// there is no single truth to restore, so that field is skipped rather than
// guessed.
//
// Second pass drains the source. Those four keys have no meaning on a keyless
// protocol — clearForeignTunnelFields already strips them from every write
// since lucx.190 — but the rows written before that still carry them, and any
// future code that reads a client out of an inbound could pick the wrong one up
// again. It must run after the repair: the stale copy is the evidence the
// repair recognises.
//
// Self-gated on the "RepairClobberedTunnelFields" seeder row.
func repairClobberedTunnelFields() error {
	var history []string
	if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &history).Error; err != nil {
		return err
	}
	if slices.Contains(history, "RepairClobberedTunnelFields") {
		return nil
	}

	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		fromTunnel, ambiguous := tunnelTruthByEmail(inbounds)
		fromKeyless := keylessCopiesByEmail(inbounds)

		var records []model.ClientRecord
		if err := tx.Find(&records).Error; err != nil {
			return err
		}
		for i := range records {
			rec := &records[i]
			truth, ok := fromTunnel[rec.Email]
			if !ok {
				continue
			}
			if n := restoreTunnelFields(rec, truth, ambiguous[rec.Email], fromKeyless[rec.Email]); n > 0 {
				if err := tx.Save(rec).Error; err != nil {
					return err
				}
				log.Printf("repair tunnel fields: restored %d field(s) for %s from its tunnel inbound", n, rec.Email)
			}
		}

		return tx.Create(&model.HistoryOfSeeders{SeederName: "RepairClobberedTunnelFields"}).Error
	})
}

// stripTunnelFieldsFromKeylessInbounds drains the source the repair reads from,
// so nothing can pick a tunnel credential out of a keyless inbound again.
//
// Deliberately NOT seeder-gated, unlike the repair above. A node whose master
// has not upgraded yet is handed those fields back on the next snapshot push,
// and a restored backup brings them back too; the write only happens when
// something was actually there, so a clean install pays one read.
func stripTunnelFieldsFromKeylessInbounds() error {
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return stripTunnelFieldsTx(tx, inbounds)
	})
}

// tunnelProtocol reports a protocol that gives a client its own keypair, PSK and
// tunnel address. Mirrors service.isTunnelProtocol, which this package cannot
// import (the service layer imports database, not the other way round).
func tunnelProtocol(p model.Protocol) bool {
	return p == model.AWG || p == model.WireGuard || p == model.AmneziaWG
}

// tunnelFields is the four values a tunnel protocol owns, in the form the
// clients table stores them — allowedIPs joined, matching Client.ToRecord.
type tunnelFields struct {
	priv    string
	pub     string
	psk     string
	allowed string
}

func (f tunnelFields) empty() bool {
	return f.priv == "" && f.pub == "" && f.psk == "" && f.allowed == ""
}

func inboundClientEntries(ib model.Inbound) []map[string]any {
	if strings.TrimSpace(ib.Settings) == "" {
		return nil
	}
	var parsed struct {
		Clients []map[string]any `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		return nil
	}
	return parsed.Clients
}

func tunnelFieldsOf(obj map[string]any) (string, tunnelFields) {
	email, _ := obj["email"].(string)
	email = strings.TrimSpace(email)
	str := func(key string) string {
		s, _ := obj[key].(string)
		return strings.TrimSpace(s)
	}
	f := tunnelFields{priv: str("privateKey"), pub: str("publicKey"), psk: str("preSharedKey")}
	switch v := obj["allowedIPs"].(type) {
	case string:
		f.allowed = strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, raw := range v {
			if s, ok := raw.(string); ok {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		f.allowed = strings.Join(parts, ",")
	}
	return email, f
}

// tunnelTruthByEmail returns what the tunnel inbounds say each identity's four
// values are, plus the emails where two of them disagree on a field.
func tunnelTruthByEmail(inbounds []model.Inbound) (map[string]tunnelFields, map[string]tunnelFields) {
	truth := map[string]tunnelFields{}
	conflict := map[string]tunnelFields{}
	for _, ib := range inbounds {
		if !tunnelProtocol(ib.Protocol) {
			continue
		}
		for _, obj := range inboundClientEntries(ib) {
			email, f := tunnelFieldsOf(obj)
			if email == "" || f.empty() {
				continue
			}
			seen, ok := truth[email]
			if !ok {
				truth[email] = f
				continue
			}
			c := conflict[email]
			if seen.priv != f.priv {
				c.priv = "x"
			}
			if seen.pub != f.pub {
				c.pub = "x"
			}
			if seen.psk != f.psk {
				c.psk = "x"
			}
			if seen.allowed != f.allowed {
				c.allowed = "x"
			}
			conflict[email] = c
		}
	}
	return truth, conflict
}

func keylessCopiesByEmail(inbounds []model.Inbound) map[string][]tunnelFields {
	out := map[string][]tunnelFields{}
	for _, ib := range inbounds {
		if tunnelProtocol(ib.Protocol) {
			continue
		}
		for _, obj := range inboundClientEntries(ib) {
			email, f := tunnelFieldsOf(obj)
			if email == "" || f.empty() {
				continue
			}
			out[email] = append(out[email], f)
		}
	}
	return out
}

// restoreTunnelFields rewrites the fields that carry the clobber's signature and
// returns how many it changed. A field with a conflicting truth, an empty truth,
// or a record value no keyless inbound explains is left exactly as it is.
func restoreTunnelFields(rec *model.ClientRecord, truth, conflict tunnelFields, copies []tunnelFields) int {
	changed := 0
	repair := func(cur *string, want, clash string, copyOf func(tunnelFields) string) {
		if want == "" || clash != "" || *cur == want {
			return
		}
		for _, c := range copies {
			if v := copyOf(c); v != "" && v == *cur {
				*cur = want
				changed++
				return
			}
		}
	}
	repair(&rec.PrivateKey, truth.priv, conflict.priv, func(f tunnelFields) string { return f.priv })
	repair(&rec.PublicKey, truth.pub, conflict.pub, func(f tunnelFields) string { return f.pub })
	repair(&rec.PreSharedKey, truth.psk, conflict.psk, func(f tunnelFields) string { return f.psk })
	repair(&rec.AllowedIPs, truth.allowed, conflict.allowed, func(f tunnelFields) string { return f.allowed })
	return changed
}

func stripTunnelFieldsTx(tx *gorm.DB, inbounds []model.Inbound) error {
	for _, ib := range inbounds {
		if tunnelProtocol(ib.Protocol) || strings.TrimSpace(ib.Settings) == "" {
			continue
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		list, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		changed := false
		for _, raw := range list {
			obj, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range []string{"privateKey", "publicKey", "preSharedKey", "allowedIPs"} {
				if _, present := obj[key]; present {
					delete(obj, key)
					changed = true
				}
			}
		}
		if !changed {
			continue
		}
		blob, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			continue
		}
		if err := tx.Model(&model.Inbound{}).Where("id = ?", ib.Id).
			Update("settings", string(blob)).Error; err != nil {
			return err
		}
		log.Printf("repair tunnel fields: dropped tunnel credentials from %s inbound %d", ib.Protocol, ib.Id)
	}
	return nil
}
