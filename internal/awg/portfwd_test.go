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

func TestParseForwardedPorts(t *testing.T) {
	got := parseForwardedPorts("80, 443; 8000-8002")
	want := []int{80, 443, 8000, 8001, 8002}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseForwardedPorts = %v, want %v", got, want)
	}
	if parseForwardedPorts("") != nil {
		t.Fatal("empty must be nil")
	}
	if parseForwardedPorts("nope, 0, 70000") != nil {
		t.Fatal("invalid tokens must drop")
	}
}

func TestFirstIPv4Host(t *testing.T) {
	if got := firstIPv4Host("10.8.0.2/32, ::/0"); got != "10.8.0.2" {
		t.Fatalf("got %q", got)
	}
	if got := firstIPv4Host("::/0"); got != "" {
		t.Fatalf("v6-only = %q", got)
	}
}
