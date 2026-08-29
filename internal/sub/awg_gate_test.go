// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import "testing"

// A share link that turns on an AWG 3.x field the server could not apply hands
// the client a config the server then refuses: RandomTrailers is gated on the
// receiver's own flag, so a mismatched pair drops every handshake.
func TestAwgVersionFieldsAllowed(t *testing.T) {
	for _, tc := range []struct {
		name         string
		versionAsks  bool
		localInbound bool
		hostSupports bool
		want         bool
	}{
		{"version does not ask", false, true, true, false},
		{"local host supports it", true, true, true, true},
		{"local host does not support it", true, true, false, false},
		// A node's capability is not knowable from here: the master may not run
		// AWG at all, so its own probe says nothing about the node.
		{"node inbound, master supports", true, false, true, true},
		{"node inbound, master does not", true, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := awgVersionFieldsAllowed(tc.versionAsks, tc.localInbound, tc.hostSupports); got != tc.want {
				t.Fatalf("awgVersionFieldsAllowed(%v, %v, %v) = %v, want %v",
					tc.versionAsks, tc.localInbound, tc.hostSupports, got, tc.want)
			}
		})
	}
}
