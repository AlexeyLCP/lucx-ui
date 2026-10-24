// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

// WgoFirewallState is the desired direct-port block state of one kernel
// WireGuard inbound: when Block is true, external UDP to the WG listen port
// is dropped so only the obfuscated path stays public (operator opt-in,
// Rule 0 — default off).
type WgoFirewallState struct {
	ID     int
	WGPort int
	Block  bool
}

// ObfuscationTargets derives the wgobfs engine instance (key wgo-<id>) and
// firewall state of one kernel WireGuard inbound. ok is false when the
// inbound has no obfuscation block at all; a present-but-disabled block
// yields Enabled=false so the manager stops the process and clears the rule.
func ObfuscationTargets(ib *model.Inbound) (tunnel.Instance, WgoFirewallState, bool) {
	o, ok := tunnel.ParseObfuscation(ib)
	if !ok {
		return tunnel.Instance{}, WgoFirewallState{}, false
	}
	inst := tunnel.Instance{
		Core:       tunnel.Wgo,
		Key:        tunnel.WgoKey(ib.Id),
		Enabled:    o.Enabled && ib.Enable,
		ConfigText: tunnel.RenderWgoConf(o, ib.Port),
	}
	fw := WgoFirewallState{ID: ib.Id, WGPort: ib.Port, Block: o.BlockDirectPort && inst.Enabled}
	return inst, fw, true
}
