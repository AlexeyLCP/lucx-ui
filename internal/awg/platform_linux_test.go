//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import "testing"

// TestIsInboundAwgInterface locks in the rule that the inbound orphan sweep
// matches only inbound AWG interfaces (awg1, awg2, ...) and never the
// outbound namespace (awgo-1, awgo-42). Regressing this caused the inbound
// sweep to ip-link-del the operator's outbound tunnels on every panel
// restart — see the gate-blocking review finding for Task 9.
func TestIsInboundAwgInterface(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"awg1", true},
		{"awg2", true},
		{"awg0", true},
		{"awg42", true},
		{"awg", false},    // no digit suffix — not a valid inbound interface name
		{"awgo-1", false}, // outbound namespace — must NOT be swept
		{"awgo-42", false},
		{"awgo-", false},
		{"awgo", false},
		{"awgsomething", false}, // non-digit after awg
		{"eth0", false},
		{"wg0", false},
		{"", false},
		{"tun0", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := isInboundAwgInterface(c.name); got != c.want {
				t.Fatalf("isInboundAwgInterface(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
