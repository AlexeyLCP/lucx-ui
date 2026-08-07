// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import "testing"

func TestToolsVersionRegex(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"amneziawg-tools v3.0.20260730 - https://amnezia.org", "3.0.20260730"},
		{"amneziawg-tools v1.0.20241101\n", "1.0.20241101"},
		{"no version here", ""},
	}
	for _, tc := range cases {
		m := awgToolsVersionRe.FindStringSubmatch(tc.in)
		got := ""
		if len(m) > 1 {
			got = m[1]
		}
		if got != tc.want {
			t.Errorf("in %q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCollectHostStatus_NonLinux(t *testing.T) {
	// On Windows CI/dev host the probe must not panic and should report empty.
	hs := CollectHostStatus()
	if hs.Interfaces < 0 {
		t.Fatalf("interfaces negative: %+v", hs)
	}
}
