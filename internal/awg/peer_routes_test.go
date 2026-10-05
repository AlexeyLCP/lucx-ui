// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"reflect"
	"testing"
)

func TestPeerRoutePrefixes(t *testing.T) {
	cases := []struct {
		address, allowed string
		want             []string
	}{
		// issue #126 exact case
		{"10.201.0.1/24", "10.201.0.3/32, 192.168.10.0/24", []string{"192.168.10.0/24"}},
		// peer /32 and default never get a route
		{"10.8.0.1/24", "10.8.0.2/32, 0.0.0.0/0, ::/0", nil},
		// server tunnel subnet excluded
		{"10.8.0.1/24", "10.8.0.2/32, 10.8.0.0/24", nil},
		// multiple LANs, dedup
		{
			"10.9.0.1/24", "10.9.0.2/32, 192.168.10.0/24, 192.168.10.0/24, 192.168.11.0/24",
			[]string{"192.168.10.0/24", "192.168.11.0/24"},
		},
		// v6 skipped, host /32 skipped
		{"10.8.0.1/24", "fd00::/64, 10.8.0.3/32", nil},
		// empty allowed
		{"10.8.0.1/24", "", nil},
	}
	for _, c := range cases {
		if got := peerRoutePrefixes(c.address, c.allowed); !reflect.DeepEqual(got, c.want) {
			t.Errorf("peerRoutePrefixes(%q, %q) = %v, want %v", c.address, c.allowed, got, c.want)
		}
	}
}
