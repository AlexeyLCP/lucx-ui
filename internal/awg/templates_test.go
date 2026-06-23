// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"
)

func TestRenderPostUp(t *testing.T) {
	data := TemplateData{
		AWGInterface:   "awg0",
		AWGServerIP:    "10.0.0.1",
		AWGSubnet:      "10.0.0.0/24",
		AWGPort:        34567,
		RouteTable:     "100",
		RouteTableName: "awg0",
		RoutePref:      1000,
		MTU:            1320,
	}

	script, err := RenderPostUp(data)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	checks := []string{
		"ip addr add 10.0.0.1/24 dev awg0",
		"ip link set awg0 mtu 1320",
		"ip link set awg0 up",
		"sysctl -w net.ipv4.ip_forward=1",
	}
	for _, check := range checks {
		if !strings.Contains(script, check) {
			t.Errorf("PostUp script missing: %s", check)
		}
	}
}

func TestRenderPostDown(t *testing.T) {
	data := TemplateData{
		AWGInterface:   "awg0",
		AWGSubnet:      "10.0.0.0/24",
		RouteTable:     "100",
		RouteTableName: "awg0",
		RoutePref:      1000,
	}

	script, err := RenderPostDown(data)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	checks := []string{
		"set +e",
		"ip link del awg0",
	}
	for _, check := range checks {
		if !strings.Contains(script, check) {
			t.Errorf("PostDown script missing: %s", check)
		}
	}
}

func TestRenderPostDown_Idempotent(t *testing.T) {
	data := TemplateData{
		AWGInterface:   "awg1",
		AWGSubnet:      "10.1.0.0/24",
		RouteTable:     "101",
		RouteTableName: "awg1",
		RoutePref:      1001,
	}

	script, err := RenderPostDown(data)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	if !strings.Contains(script, "2>/dev/null") {
		t.Error("PostDown must use 2>/dev/null guards for idempotent cleanup")
	}
	if !strings.Contains(script, "set +e") {
		t.Error("PostDown must start with 'set +e' to continue on errors")
	}
}
