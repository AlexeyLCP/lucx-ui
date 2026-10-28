// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	stdnet "net"
	"testing"
)

func TestPublicIPString(t *testing.T) {
	deny := map[string]string{
		"loopback":    "127.0.0.1",
		"loopback6":   "::1",
		"rfc1918":     "192.168.1.5",
		"rfc1918-10":  "10.0.0.1",
		"rfc1918-172": "172.16.0.9",
		"ula":         "fc00::1",
		"cgnat-low":   "100.64.0.1",
		"cgnat-high":  "100.127.255.255",
		"linklocal":   "fe80::1",
		"multicast":   "ff02::1",
		"unspecified": "0.0.0.0",
	}

	for name, s := range deny {
		if str, ok := publicIPString(stdnet.ParseIP(s)); ok {
			t.Errorf("%s %s accepted as public (got %q)", name, s, str)
		}
	}

	allow := map[string]string{
		"cgnat-below": "100.63.255.255",
		"cgnat-above": "100.128.0.1",
		"public-v4":   "203.0.113.10",
		"public-v6":   "2001:db8::5",
	}
	for name, s := range allow {
		str, ok := publicIPString(stdnet.ParseIP(s))
		if !ok {
			t.Errorf("%s %s rejected as public", name, s)
			continue
		}
		if want := stdnet.ParseIP(s).String(); str != want {
			t.Errorf("%s: got %q want %q", name, str, want)
		}
	}
}

func TestPublicIPStringCgnatBoundaries(t *testing.T) {
	// 100.64.0.0/10 spans 100.64.0.0 .. 100.127.255.255.
	if _, ok := publicIPString(stdnet.ParseIP("100.64.0.0")); ok {
		t.Error("100.64.0.0 must be CGNAT")
	}
	if _, ok := publicIPString(stdnet.ParseIP("100.127.255.255")); ok {
		t.Error("100.127.255.255 must be CGNAT")
	}
	if _, ok := publicIPString(stdnet.ParseIP("100.63.0.1")); !ok {
		t.Error("100.63.0.1 is outside CGNAT, must be public")
	}
	if _, ok := publicIPString(stdnet.ParseIP("100.128.0.0")); !ok {
		t.Error("100.128.0.0 is outside CGNAT, must be public")
	}
}
