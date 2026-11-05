// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import "testing"

func TestMaskIP(t *testing.T) {
	cases := []struct{ ip, mode, want string }{
		{"203.0.113.10", "", "203.0.113.10"},
		{"203.0.113.10", "quarter", "203.0.113.*"},
		{"203.0.113.10", "half", "203.0.*.*"},
		{"203.0.113.10", "full", ""},
		{"::ffff:203.0.113.10", "quarter", "203.0.113.*"},
		{"2a02:6b8:1:2:3:4:5:6", "quarter", "2a02:6b8:1::/48"},
		{"2a02:6b8:1:2:3:4:5:6", "half", "2a02:6b8::/32"},
		{"2a02:6b8:1:2:3:4:5:6", "full", ""},
		{"not-an-ip", "half", "not-an-ip"},
	}
	for _, c := range cases {
		if got := MaskIP(c.ip, c.mode); got != c.want {
			t.Errorf("MaskIP(%q, %q) = %q, want %q", c.ip, c.mode, got, c.want)
		}
	}
}
