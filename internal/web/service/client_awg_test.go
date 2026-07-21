// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import "testing"

func TestAwgAllocationFallback(t *testing.T) {
	tests := []struct {
		serverAddr string
		want       string
	}{
		{"10.9.0.1/24", "10.9.0.0/24"},
		{"192.168.100.1/16", "192.168.0.0/16"},
		{"", defaultAwgBase},
		{"   ", defaultAwgBase},
		{"not-an-ip", defaultAwgBase},
		{"10.8.0.1", defaultAwgBase},
	}
	for _, tt := range tests {
		t.Run(tt.serverAddr, func(t *testing.T) {
			if got := awgAllocationFallback(tt.serverAddr); got != tt.want {
				t.Errorf("awgAllocationFallback(%q) = %q, want %q", tt.serverAddr, got, tt.want)
			}
		})
	}
}

func TestAwgSettingsAddress(t *testing.T) {
	if got := awgSettingsAddress(`{"address":"10.9.0.1/24","mtu":1320}`); got != "10.9.0.1/24" {
		t.Errorf("awgSettingsAddress = %q, want 10.9.0.1/24", got)
	}
	if got := awgSettingsAddress(`{"mtu":1320}`); got != "" {
		t.Errorf("missing address must yield empty, got %q", got)
	}
	if got := awgSettingsAddress(`{broken`); got != "" {
		t.Errorf("malformed JSON must yield empty, got %q", got)
	}
	if got := awgSettingsAddress(""); got != "" {
		t.Errorf("empty settings must yield empty, got %q", got)
	}
}

// TestParseAwgOutboundAddress covers the pure-function path of
// ActiveOutboundAddresses — parsing the tunnel Address from one AWG
// outbound settings JSON blob. This is the collision guard that keeps
// defaultAwgClients from handing a new client the same IP an awgo-N
// interface already owns.
func TestParseAwgOutboundAddress(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"with address", `{"address":"10.8.0.2/32","endpoint":"up:51820"}`, "10.8.0.2/32"},
		{"no address", `{"endpoint":"up:51820"}`, ""},
		{"empty address", `{"address":""}`, ""},
		{"whitespace address", `{"address":"  "}`, ""},
		{"malformed json", `{broken`, ""},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAwgOutboundAddress(tt.in); got != tt.want {
				t.Errorf("parseAwgOutboundAddress(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
