// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"net/netip"
	"strconv"
	"strings"
	"testing"
)

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatal(err)
	}
	return p.Masked()
}

func TestAwgMigrationIPsStale(t *testing.T) {
	subnet := mustPrefix(t, "10.9.0.0/24")
	tests := []struct {
		name string
		ips  []string
		want bool
	}{
		{"inside", []string{"10.9.0.7/32"}, false},
		{"outside", []string{"10.8.0.7/32"}, true},
		{"empty", nil, false},
		{"route entry never stale", []string{"0.0.0.0/0"}, false},
		{"unparseable never stale", []string{"???"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := awgMigrationIPsStale(tt.ips, subnet); got != tt.want {
				t.Errorf("awgMigrationIPsStale(%v) = %v, want %v", tt.ips, got, tt.want)
			}
		})
	}
}

func TestNormalizeHostEntry(t *testing.T) {
	tests := []struct{ in, want string }{
		{"10.8.0.2/32", "10.8.0.2/32"},
		{"10.8.0.2", "10.8.0.2/32"},
		{" 10.8.0.2/32 ", "10.8.0.2/32"},
		{"0.0.0.0/0", "0.0.0.0/0"},
		{"junk", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeHostEntry(tt.in); got != tt.want {
			t.Errorf("normalizeHostEntry(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAllocAwgAddressInSubnet(t *testing.T) {
	subnet := mustPrefix(t, "10.9.0.0/24")
	addr, ok := allocAwgAddressInSubnet(subnet, map[string]bool{})
	if !ok || addr != "10.9.0.2/32" {
		t.Fatalf("first allocation = %q (%v), want 10.9.0.2/32", addr, ok)
	}
	addr, ok = allocAwgAddressInSubnet(subnet, map[string]bool{"10.9.0.2/32": true})
	if !ok || addr != "10.9.0.3/32" {
		t.Fatalf("second allocation = %q (%v), want 10.9.0.3/32", addr, ok)
	}
	full := make(map[string]bool)
	for i := 0; i < 256; i++ {
		full["10.9.0."+strconv.Itoa(i)+"/32"] = true
	}
	if _, ok := allocAwgAddressInSubnet(subnet, full); ok {
		t.Fatal("full subnet must yield no address")
	}
}

func TestFixStaleAwgClients(t *testing.T) {
	settings := `{"address":"10.9.0.1/24","clients":[` +
		`{"email":"fresh","allowedIPs":["10.9.0.2/32"]},` +
		`{"email":"stale","allowedIPs":["10.8.0.9/32"]},` +
		`{"email":"custom","allowedIPs":["0.0.0.0/0"]}]}`

	out, fixes := fixStaleAwgClients(settings, nil)
	if len(fixes) != 1 {
		t.Fatalf("fixes = %v, want exactly one (stale)", fixes)
	}
	fix, ok := fixes["stale"]
	if !ok {
		t.Fatalf("fixes must key the stale client: %v", fixes)
	}
	if fix[0] != "10.8.0.9/32" {
		t.Errorf("old value = %q, want 10.8.0.9/32", fix[0])
	}
	if !strings.HasPrefix(fix[1], "10.9.0.") || !strings.HasSuffix(fix[1], "/32") {
		t.Errorf("new address %q must come from the current subnet", fix[1])
	}
	if fix[1] == "10.9.0.2/32" {
		t.Errorf("new address %q must not collide with the fresh client", fix[1])
	}

	var parsed struct {
		Clients []struct {
			Email      string   `json:"email"`
			AllowedIPs []string `json:"allowedIPs"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal(err)
	}
	byEmail := make(map[string][]string)
	for _, c := range parsed.Clients {
		byEmail[c.Email] = c.AllowedIPs
	}
	if byEmail["fresh"][0] != "10.9.0.2/32" {
		t.Errorf("fresh client must be untouched: %v", byEmail["fresh"])
	}
	if byEmail["custom"][0] != "0.0.0.0/0" {
		t.Errorf("custom client must be untouched: %v", byEmail["custom"])
	}
	if byEmail["stale"][0] != fix[1] {
		t.Errorf("stale client must carry the new address: %v", byEmail["stale"])
	}

	// Idempotent: the fixed output has nothing stale left.
	out2, fixes2 := fixStaleAwgClients(out, nil)
	if len(fixes2) != 0 {
		t.Fatalf("second pass must be a no-op, got %v", fixes2)
	}
	if out2 != out {
		t.Error("second pass must return the input unchanged")
	}
}

func TestFixStaleAwgClientsNoOps(t *testing.T) {
	cases := []string{
		`{"address":"10.9.0.1/24","clients":[{"email":"a","allowedIPs":["10.9.0.2/32"]}]}`,
		`{"address":"10.9.0.1/24","clients":[]}`,
		`{"address":"10.9.0.1/24"}`,
		`{broken`,
	}
	for _, in := range cases {
		out, fixes := fixStaleAwgClients(in, nil)
		if len(fixes) != 0 || out != in {
			t.Errorf("expected no-op for %q, got %d fixes", in, len(fixes))
		}
	}
}

func TestFixStaleAwgClientsAvoidsAwgoIPs(t *testing.T) {
	settings := `{"address":"10.8.0.1/24","clients":[{"email":"stale","allowedIPs":["10.9.9.9/32"]}]}`
	out, fixes := fixStaleAwgClients(settings, []string{"10.8.0.2/32"})
	if len(fixes) != 1 {
		t.Fatalf("fixes = %v, want one", fixes)
	}
	if fixes["stale"][1] == "10.8.0.2/32" {
		t.Error("re-allocation must skip the awgo outbound IP")
	}
	if fixes["stale"][1] != "10.8.0.3/32" {
		t.Errorf("expected 10.8.0.3/32, got %q", fixes["stale"][1])
	}
	_ = out
}
