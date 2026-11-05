// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import "testing"

func TestCoverListensOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"203.0.113.5", "203.0.113.6", false},
		{"203.0.113.5", "203.0.113.5", true},
		{"", "203.0.113.5", true},
		{"0.0.0.0", "203.0.113.5", true},
		{"127.0.0.1", "", false},
		{"", "", true},
	}
	for _, c := range cases {
		if got := coverListensOverlap(c.a, c.b); got != c.want {
			t.Errorf("coverListensOverlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
