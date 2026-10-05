//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"net/netip"
	"os/exec"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// ensurePeerRoutesLocked converges the kernel routes to the LAN subnets behind
// AWG clients (issue #126). `Table = off` means awg-quick installs nothing
// beyond the peer /32s, so a client's LAN (AllowedIPs contains
// 192.168.10.0/24) needs `ip route replace <prefix> dev awgN`. Runs from the
// reconcile loop, so it self-heals after reboot and AWG restart the same way
// ensureNatRules does. The installed set is kept on the managed entry
// (peerRoutes), so removal deletes exactly what LucX added — a foreign route
// on the same prefix stays. Caller holds m.mu.
func (m *Manager) ensurePeerRoutesLocked(inst Instance) {
	if inst.Ifname == "" || inst.RouteThroughXray {
		return
	}
	cur, ok := m.procs[inst.Id]
	if !ok || !cur.proc.IsRunning() {
		return
	}
	if err := exec.CommandContext(context.Background(), "ip", "link", "show", inst.Ifname).Run(); err != nil {
		return
	}
	old := cur.peerRoutes
	want := map[string]struct{}{}
	var order []string
	for _, peer := range inst.Peers {
		for _, sub := range peerRoutePrefixes(inst.Address, peer.AllowedIPs) {
			if _, dup := want[sub]; dup {
				continue
			}
			want[sub] = struct{}{}
			order = append(order, sub)
		}
	}
	// Drop routes whose prefix left the peer set; replace (idempotent) the rest.
	for _, sub := range old {
		if _, keep := want[sub]; keep {
			continue
		}
		delPeerRoute(inst.Ifname, sub)
	}
	for _, sub := range order {
		if out, err := exec.CommandContext(context.Background(), "ip", "route", "replace", sub, "dev", inst.Ifname).CombinedOutput(); err != nil {
			logger.Warningf("awg: peer route (%s %s): %v\n%s", inst.Ifname, sub, err, strings.TrimSpace(string(out)))
			// Keep the previous set so the next tick retries; a prefix
			// whose replace failed is dropped from the record only when
			// it also left the peer set (handled above).
			cur.peerRoutes = old
			return
		}
	}
	cur.peerRoutes = order
}

// flushPeerRoutesLocked deletes the peer-LAN routes recorded on a managed
// entry. Caller holds m.mu; runs for inbound removal / reconcile sweep. The
// interface may already be down — route deletes are then no-ops against the
// dead device's entries (kernel already dropped routes with the device).
func (m *Manager) flushPeerRoutesLocked(id int) {
	cur, ok := m.procs[id]
	if !ok {
		return
	}
	installed := cur.peerRoutes
	cur.peerRoutes = nil
	for _, sub := range installed {
		delPeerRoute(cur.ifname, sub)
	}
}

func delPeerRoute(ifname, sub string) {
	if ifname == "" {
		return
	}
	if _, err := netip.ParsePrefix(sub); err != nil {
		return
	}
	// Device-scoped delete: a same-prefix route on another dev is not ours.
	if out, err := exec.CommandContext(context.Background(), "ip", "route", "del", sub, "dev", ifname).CombinedOutput(); err != nil && !strings.Contains(string(out), "No such process") {
		logger.Warningf("awg: peer route flush (%s %s): %v\n%s", ifname, sub, err, strings.TrimSpace(out))
	}
}
