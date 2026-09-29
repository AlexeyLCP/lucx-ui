// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

// AnyTLS routeThroughXray bridge (lucx.273): anytls-go has no SOCKS dialer
// option, so the sidecar's own outbound TCP is redirected by uid (shared
// lucx-mtproxy user) into the tproxy-style listener (23990) which relays into
// the hidden Xray SOCKS inbound. The rule/listener are shared with a routed
// tproxy inbound; redirectOwners in tproxy_firewall_linux.go decides when the
// physical rule may actually be removed.

func EnsureAnytlsXraySocks(socksPort int) {
	redirectOwnerEnsure("anytls", socksPort)
}

func ClearAnytlsXraySocks() {
	redirectOwnerClear("anytls")
}
