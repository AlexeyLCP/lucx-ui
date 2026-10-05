// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"net/netip"
	"strings"
)

// peerRoutePrefixes extracts the LAN subnets behind a client from its
// AllowedIPs: prefixes wider than /32 (//128) that are not the catch-all
// default and do not overlap the server's own tunnel subnet. The peer /32
// itself already gets a route from awg-quick, so only these need one.
func peerRoutePrefixes(tunnelAddress, allowed string) []string {
	tunnel := clientSubnet(tunnelAddress)
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.Split(allowed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil || !prefix.Addr().Is4() {
			continue
		}
		if prefix.IsSingleIP() || prefix.Bits() == 0 {
			continue
		}
		masked := prefix.Masked()
		sub := masked.String()
		if sub == tunnel {
			continue
		}
		if _, dup := seen[sub]; dup {
			continue
		}
		seen[sub] = struct{}{}
		out = append(out, sub)
	}
	return out
}
