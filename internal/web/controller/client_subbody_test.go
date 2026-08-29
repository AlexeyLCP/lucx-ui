// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import "testing"

func TestMatchSubRoute(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		format string
		ok     bool
	}{
		{"sub default", "/sub/abc123", "sub", true},
		{"json default", "/json/abc123", "json", true},
		{"clash default", "/clash/abc123", "clash", true},
		{"awg default", "/awg/abc123", "awg", true},
		{"awg bare prefix", "/awg/", "", false},
		{"foreign path", "/evil/abc123", "", false},
		{"prefix lookalike", "/submarine/abc", "", false},
		{"subscription vs sub", "/subscription/abc", "", false},
		{"custom path", "/my-sub/x9", "sub", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			subPath, jsonPath, clashPath, awgPath := "/sub/", "/json/", "/clash/", "/awg/"
			if tc.name == "custom path" {
				subPath = "/my-sub/"
			}
			format, ok := matchSubRoute(tc.path, subPath, jsonPath, clashPath, awgPath)
			if ok != tc.ok || format != tc.format {
				t.Fatalf("matchSubRoute(%q) = %q,%v; want %q,%v", tc.path, format, ok, tc.format, tc.ok)
			}
		})
	}
	if format, ok := matchSubRoute("/subscription/abc", "/sub", "/json/", "/clash/", "/awg/"); ok {
		t.Fatalf("/sub must not match /subscription, got %q", format)
	}
}
