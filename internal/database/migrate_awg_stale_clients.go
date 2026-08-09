// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"log"
	"net/netip"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// migrateAwgStaleClients repairs AWG clients whose stored tunnel address no
// longer belongs to the subnet of the inbound they are attached to (lucx.92).
// The damage path: a client created on an AWG inbound, detached, and
// re-attached later — after the inbound subnet changed or onto another AWG
// inbound — kept its old single-host address, because the allocator only
// fills BLANK credentials. The kernel interface then carries a peer routable
// to a network it does not own: the handshake completes (keys match) but
// traffic dies — the live symptom reported on lucx.85–90 ("коннект есть,
// трафика нет"). The attach path re-allocates on the fly from lucx.92; this
// migration fixes clients that are ALREADY stored stale, without waiting for
// a manual re-attach.
//
// Only single-host addresses (/32, /128) outside the current subnet are
// rotated; custom allowedIPs (0.0.0.0/0 etc.) are operator-managed and stay
// untouched. Keys and PSK are preserved — only the address changes, so the
// client just re-imports the refreshed config. Idempotent: a second run
// finds nothing stale.
func migrateAwgStaleClients() error {
	awgoIPs := awgoOutboundIPs()

	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		return err
	}
	for _, ib := range inbounds {
		newSettings, fixes := fixStaleAwgClients(ib.Settings, awgoIPs)
		if len(fixes) == 0 {
			continue
		}
		if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", newSettings).Error; err != nil {
			log.Printf("[LUCX-AWG] migration: failed to fix stale clients on inbound %d: %v", ib.Id, err)
			continue
		}
		for email, fix := range fixes {
			log.Printf("[LUCX-AWG] migration: inbound %d client %q address %s -> %s (stale, outside the current subnet)", ib.Id, email, fix[0], fix[1])
			syncClientRecordAllowedIP(email, fix[0], fix[1])
		}
	}
	return nil
}

// awgoOutboundIPs collects the tunnel addresses of AWG outbounds so the
// re-allocator never hands a client an IP an awgo-N interface already owns
// (the same collision guard defaultAwgClients applies).
func awgoOutboundIPs() []string {
	var outbounds []model.AwgOutbound
	if err := db.Find(&outbounds).Error; err != nil {
		return nil
	}
	ips := make([]string, 0, len(outbounds))
	for _, o := range outbounds {
		var s struct {
			Address string `json:"address"`
		}
		if json.Unmarshal([]byte(o.Settings), &s) == nil && strings.TrimSpace(s.Address) != "" {
			ips = append(ips, s.Address)
		}
	}
	return ips
}

// fixStaleAwgClients rewrites the inbound settings JSON, re-allocating a
// current-subnet address for every client whose stored allowedIPs are stale
// (see migrateAwgStaleClients). Returns the unchanged JSON and no fixes when
// nothing is stale. fixes maps client email -> [oldJoined, newAddr] so the
// caller can sync the clients table.
func fixStaleAwgClients(settingsJSON string, awgoIPs []string) (string, map[string][2]string) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return settingsJSON, nil
	}
	addrRaw, _ := settings["address"].(string)
	subnet, err := netip.ParsePrefix(strings.TrimSpace(addrRaw))
	if err != nil {
		return settingsJSON, nil
	}
	subnet = subnet.Masked()

	clientsRaw, _ := settings["clients"].([]any)
	if len(clientsRaw) == 0 {
		return settingsJSON, nil
	}

	type clientEntry struct {
		m     map[string]any
		email string
		ips   []string
	}
	entries := make([]clientEntry, 0, len(clientsRaw))
	used := make(map[string]bool)
	for _, raw := range clientsRaw {
		m, _ := raw.(map[string]any)
		if m == nil {
			continue
		}
		email, _ := m["email"].(string)
		ips := awgMigrationStringSlice(m["allowedIPs"])
		entries = append(entries, clientEntry{m: m, email: email, ips: ips})
		for _, ip := range ips {
			if norm := normalizeHostEntry(ip); norm != "" {
				used[norm] = true
			}
		}
	}
	for _, ip := range awgoIPs {
		if norm := normalizeHostEntry(ip); norm != "" {
			used[norm] = true
		}
	}

	fixes := make(map[string][2]string)
	changed := false
	for _, e := range entries {
		if !awgMigrationIPsStale(e.ips, subnet) {
			continue
		}
		newAddr, ok := allocAwgAddressInSubnet(subnet, used)
		if !ok {
			continue
		}
		for _, ip := range e.ips {
			if norm := normalizeHostEntry(ip); norm != "" {
				delete(used, norm)
			}
		}
		used[normalizeHostEntry(newAddr)] = true
		e.m["allowedIPs"] = []string{newAddr}
		if strings.TrimSpace(e.email) != "" {
			fixes[e.email] = [2]string{strings.Join(e.ips, ","), newAddr}
		}
		changed = true
	}
	if !changed {
		return settingsJSON, nil
	}
	out, err := json.Marshal(settings)
	if err != nil {
		return settingsJSON, nil
	}
	return string(out), fixes
}

// awgMigrationIPsStale mirrors service.awgAllowedIPsStale against an
// already-parsed subnet: every entry must be a single-host address and at
// least one must fall outside the subnet.
func awgMigrationIPsStale(ips []string, subnet netip.Prefix) bool {
	if len(ips) == 0 {
		return false
	}
	outside := false
	for _, raw := range ips {
		p, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return false
		}
		if p.Bits() != p.Addr().BitLen() {
			return false
		}
		if !subnet.Contains(p.Addr()) {
			outside = true
		}
	}
	return outside
}

// normalizeHostEntry canonicalizes "10.8.0.2/32" (or a bare IP) to
// "10.8.0.2/32" form so used-set comparisons are exact. Non-host entries
// are returned as their parsed prefix string.
func normalizeHostEntry(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "/") {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			return ""
		}
		return addr.String() + "/" + strconv.Itoa(addr.BitLen())
	}
	p, err := netip.ParsePrefix(raw)
	if err != nil {
		return ""
	}
	return p.String()
}

// allocAwgAddressInSubnet returns the first free single-host address of the
// subnet (starting at base+2 — base+1 conventionally belongs to the server),
// skipping everything in used. Bounded so a misconfigured giant prefix cannot
// hang the migration.
func allocAwgAddressInSubnet(subnet netip.Prefix, used map[string]bool) (string, bool) {
	const maxProbes = 4096
	subnet = subnet.Masked()
	cur := subnet.Addr().Next().Next()
	for i := 0; i < maxProbes; i++ {
		if !subnet.Contains(cur) {
			break
		}
		entry := cur.String() + "/" + strconv.Itoa(cur.BitLen())
		if !used[entry] {
			return entry, true
		}
		if !subnet.Contains(cur.Next()) {
			break
		}
		cur = cur.Next()
	}
	return "", false
}

// awgMigrationStringSlice reads an allowedIPs value that may be a JSON array
// or (defensively) a comma-joined string.
func awgMigrationStringSlice(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return t
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				out = append(out, strings.TrimSpace(p))
			}
		}
		return out
	}
	return nil
}

// syncClientRecordAllowedIP updates the clients-table row when it still holds
// exactly the stale value (a row already carrying something else belongs to a
// newer attachment and must not be clobbered).
func syncClientRecordAllowedIP(email, oldJoined, newAddr string) {
	rec := &model.ClientRecord{}
	if err := db.Where("email = ?", email).First(rec).Error; err != nil {
		return
	}
	if rec.AllowedIPs != oldJoined {
		return
	}
	if err := db.Model(rec).Update("wg_allowed_ips", newAddr).Error; err != nil {
		log.Printf("[LUCX-AWG] migration: failed to sync clients row for %q: %v", email, err)
	}
}
