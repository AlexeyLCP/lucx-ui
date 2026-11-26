// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import "testing"

func TestParsePingLatency(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want int
	}{
		{
			name: "ipv4 summary",
			out:  "PING 1.1.1.1...\n--- statistics ---\nrtt min/avg/max/mdev = 12.345/14.567/16.789/1.234 ms",
			want: 14,
		},
		{
			name: "ipv6 round-trip summary",
			out:  "PING 2606:4700::1111...\nround-trip min/avg/max/mdev = 30.001/33.501/37.002/2.100 ms",
			want: 33,
		},
		{
			name: "no summary line",
			out:  "ping: unknown host example.invalid",
			want: 0,
		},
		{
			name: "split quartet without equals",
			out:  "avg 3/4/5",
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePingLatency(tc.out); got != tc.want {
				t.Fatalf("parsePingLatency(%q) = %d, want %d", tc.out, got, tc.want)
			}
		})
	}
}
