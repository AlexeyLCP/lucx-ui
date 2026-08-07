// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestResolveInboundShareHost(t *testing.T) {
	cases := []struct {
		name     string
		ib       *model.Inbound
		node     string
		fallback string
		want     string
	}{
		{
			name:     "fallback when empty inbound",
			ib:       &model.Inbound{},
			fallback: "1.2.3.4",
			want:     "1.2.3.4",
		},
		{
			name:     "never prefer empty over fallback",
			ib:       &model.Inbound{Listen: "0.0.0.0", ShareAddrStrategy: "listen"},
			fallback: "vpn.example.com",
			want:     "vpn.example.com",
		},
		{
			name:     "listen strategy uses routable listen",
			ib:       &model.Inbound{Listen: "203.0.113.7", ShareAddrStrategy: "listen", Port: 1},
			fallback: "fallback.example",
			want:     "203.0.113.7",
		},
		{
			name: "custom share addr wins",
			ib: &model.Inbound{
				ShareAddrStrategy: "custom",
				ShareAddr:         "edge.example.com",
				Listen:            "203.0.113.7",
			},
			node:     "node.example.com",
			fallback: "fallback.example",
			want:     "edge.example.com",
		},
		{
			name:     "node strategy prefers node over listen",
			ib:       &model.Inbound{Listen: "203.0.113.7", ShareAddrStrategy: "node"},
			node:     "node.example.com",
			fallback: "fallback.example",
			want:     "node.example.com",
		},
		{
			name:     "loopback listen skipped",
			ib:       &model.Inbound{Listen: "127.0.0.1", ShareAddrStrategy: "listen"},
			fallback: "1.2.3.4",
			want:     "1.2.3.4",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveInboundShareHost(tc.ib, tc.node, tc.fallback)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatEndpointHost_IPv6(t *testing.T) {
	if got := formatEndpointHost("2001:db8::1"); got != "[2001:db8::1]" {
		t.Fatalf("got %q", got)
	}
	if got := formatEndpointHost("1.2.3.4"); got != "1.2.3.4" {
		t.Fatalf("got %q", got)
	}
	if got := formatEndpointHost("[2001:db8::1]"); got != "[2001:db8::1]" {
		t.Fatalf("got %q", got)
	}
}
