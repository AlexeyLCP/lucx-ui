// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"net/netip"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// awgPeerIPRepairEnabled gates repairAwgStalePeerIPs. Default false (lucx.106):
// never rewrite client tunnel IPs on panel start — live servers must stay
// opt-in only (lesson from lucx.91→92). Flip only for a controlled one-shot.
var awgPeerIPRepairEnabled = false

// repairAwgStalePeerIPs re-allocates single-host peer AllowedIPs that sit
// outside their AWG inbound's tunnel subnet. Multi-attach client updates used
// to broadcast one tunnel IP into every inbound settings blob → awg-quick
// "ip route add … RTNETLINK: File exists" and client .conf Address from the
// wrong subnet. Idempotent. Keys/PSK/email unchanged.
// NOT called from InitDB unless awgPeerIPRepairEnabled (default off).
func repairAwgStalePeerIPs() {
	if db == nil || !awgPeerIPRepairEnabled {
		return
	}
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", model.AWG).Find(&inbounds).Error; err != nil {
		logger.Warning("repair awg peer ips: list failed:", err)
		return
	}
	fixed := 0
	for i := range inbounds {
		ib := &inbounds[i]
		n, err := repairOneAwgInboundPeerIPs(ib)
		if err != nil {
			logger.Warning("repair awg peer ips: inbound ", ib.Id, ": ", err)
			continue
		}
		if n == 0 {
			continue
		}
		if err := db.Model(ib).Update("settings", ib.Settings).Error; err != nil {
			logger.Warning("repair awg peer ips: save inbound ", ib.Id, ": ", err)
			continue
		}
		fixed += n
	}
	if fixed > 0 {
		logger.Info("repair awg peer ips: reallocated ", fixed, " peer address(es)")
	}
}

func repairOneAwgInboundPeerIPs(ib *model.Inbound) (int, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		return 0, err
	}
	serverAddr, _ := settings["address"].(string)
	subnet, err := netip.ParsePrefix(strings.TrimSpace(serverAddr))
	if err != nil {
		return 0, nil
	}
	subnet = subnet.Masked()
	clients, _ := settings["clients"].([]any)
	if len(clients) == 0 {
		return 0, nil
	}

	used := map[netip.Addr]struct{}{}
	if srv := addrFromCIDR(serverAddr); srv.IsValid() {
		used[srv] = struct{}{}
	}
	for _, raw := range clients {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, ip := range anyStringSliceLocal(m["allowedIPs"]) {
			if a, ok := parseHostAddr(ip); ok && subnet.Contains(a) {
				used[a] = struct{}{}
			}
		}
	}

	changed := 0
	for i, raw := range clients {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ips := anyStringSliceLocal(m["allowedIPs"])
		if !peerIPsStale(ips, subnet) {
			continue
		}
		addr, ok := nextHostInSubnet(subnet, used)
		if !ok {
			logger.Warning("repair awg peer ips: inbound ", ib.Id, " subnet full, skip client")
			continue
		}
		used[addr] = struct{}{}
		host := addr.String() + "/32"
		if addr.Is6() {
			host = addr.String() + "/128"
		}
		m["allowedIPs"] = []any{host}
		clients[i] = m
		changed++
		if email, _ := m["email"].(string); email != "" {
			logger.Info("repair awg peer ips: inbound ", ib.Id, " client ", email, " → ", host)
		}
	}
	if changed == 0 {
		return 0, nil
	}
	settings["clients"] = clients
	bs, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return 0, err
	}
	ib.Settings = string(bs)
	return changed, nil
}

func peerIPsStale(ips []string, subnet netip.Prefix) bool {
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

func parseHostAddr(raw string) (netip.Addr, bool) {
	raw = strings.TrimSpace(raw)
	if p, err := netip.ParsePrefix(raw); err == nil {
		if p.Bits() == p.Addr().BitLen() {
			return p.Addr(), true
		}
		return netip.Addr{}, false
	}
	a, err := netip.ParseAddr(raw)
	return a, err == nil
}

func addrFromCIDR(s string) netip.Addr {
	if p, err := netip.ParsePrefix(strings.TrimSpace(s)); err == nil {
		return p.Addr()
	}
	if a, err := netip.ParseAddr(strings.TrimSpace(s)); err == nil {
		return a
	}
	return netip.Addr{}
}

func nextHostInSubnet(subnet netip.Prefix, used map[netip.Addr]struct{}) (netip.Addr, bool) {
	start := subnet.Addr()
	for a := start; subnet.Contains(a); a = a.Next() {
		if !a.IsValid() {
			break
		}
		if a == start {
			continue
		}
		if _, taken := used[a]; taken {
			continue
		}
		if a.Is4() && subnet.Bits() <= 24 {
			b := a.As4()
			if b[3] == 0 || b[3] == 255 {
				continue
			}
		}
		return a, true
	}
	return netip.Addr{}, false
}

func anyStringSliceLocal(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
