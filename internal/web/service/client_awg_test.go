// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"net/netip"
	"testing"
)

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

// TestMigrateAwgClientSubnets covers the Pattern 1h fix: changing an AWG
// inbound's Address must re-allocate peer AllowedIPs from the old subnet
// into the new one, leave custom entries untouched, and be a no-op when the
// subnet didn't really change.
func TestMigrateAwgClientSubnets(t *testing.T) {
	cases := []struct {
		name     string
		oldAddr  string
		newAddr  string
		settings string
		check    func(t *testing.T, out string)
	}{
		{
			name:     "subnet change reallocates clients",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.5.1/24",
			settings: `{"address":"10.8.5.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]},{"email":"b","allowedIPs":["10.8.0.3/32"]}]}`,
			check: func(t *testing.T, out string) {
				var s struct {
					Address string `json:"address"`
					Clients []struct {
						Email      string   `json:"email"`
						AllowedIPs []string `json:"allowedIPs"`
					} `json:"clients"`
				}
				if err := json.Unmarshal([]byte(out), &s); err != nil {
					t.Fatalf("settings not valid JSON: %v", out)
				}
				if s.Address != "10.8.5.1/24" {
					t.Errorf("address mutated to %q", s.Address)
				}
				seen := map[string]struct{}{}
				for _, c := range s.Clients {
					if len(c.AllowedIPs) != 1 {
						t.Fatalf("client %s: expected 1 allowedIP, got %v", c.Email, c.AllowedIPs)
					}
					ip := c.AllowedIPs[0]
					seen[ip] = struct{}{}
					p, err := netip.ParsePrefix(ip)
					if err != nil {
						t.Fatalf("client %s: %q not a prefix", c.Email, ip)
					}
					if !mustParsePrefix("10.8.5.0/24").Contains(p.Addr()) {
						t.Errorf("client %s: %q not in new subnet 10.8.5.0/24", c.Email, ip)
					}
					if p.Addr() == netip.MustParseAddr("10.8.5.1") {
						t.Errorf("client %s: reused the server's .1", c.Email)
					}
				}
				if len(seen) != len(s.Clients) {
					t.Errorf("duplicate allowedIPs after migration: %v", seen)
				}
			},
		},
		{
			name:     "same subnet different host is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.0.5/24",
			settings: `{"address":"10.8.0.5/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"10.8.0.5/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}` {
					t.Errorf("same-subnet change must be a no-op, got %q", out)
				}
			},
		},
		{
			name:     "address unchanged is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.0.1/24",
			settings: `{"address":"10.8.0.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"10.8.0.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}` {
					t.Errorf("unchanged address must be a no-op, got %q", out)
				}
			},
		},
		{
			name:     "custom allowedIPs left untouched",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.5.1/24",
			settings: `{"address":"10.8.5.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]},{"email":"b","allowedIPs":["0.0.0.0/0"]}]}`,
			check: func(t *testing.T, out string) {
				var s struct {
					Clients []struct {
						Email      string   `json:"email"`
						AllowedIPs []string `json:"allowedIPs"`
					} `json:"clients"`
				}
				_ = json.Unmarshal([]byte(out), &s)
				if s.Clients[1].AllowedIPs[0] != "0.0.0.0/0" {
					t.Errorf("custom 0.0.0.0/0 must be preserved, got %v", s.Clients[1].AllowedIPs)
				}
				if s.Clients[0].AllowedIPs[0] == "10.8.0.2/32" {
					t.Errorf("old-subnet client was not migrated: %v", s.Clients[0].AllowedIPs)
				}
			},
		},
		{
			name:     "no clients is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.5.1/24",
			settings: `{"address":"10.8.5.1/24","clients":[]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"10.8.5.1/24","clients":[]}` {
					t.Errorf("no-clients must be a no-op, got %q", out)
				}
			},
		},
		{
			name:     "invalid new address is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "not-an-ip",
			settings: `{"address":"not-an-ip","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"not-an-ip","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}` {
					t.Errorf("invalid address must be a no-op, got %q", out)
				}
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			c.check(t, migrateAwgClientSubnets(c.oldAddr, c.newAddr, c.settings))
		})
	}
}

func mustParsePrefix(s string) netip.Prefix {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		panic(err)
	}
	return p
}
