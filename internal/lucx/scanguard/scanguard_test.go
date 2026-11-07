// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package scanguard

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func TestParsePrefixList(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		want        map[string]struct{}
		wantSkipped int
		wantErr     bool
	}{
		{
			name: "cidr bare ip comments dedupe",
			text: "# header comment\n1.2.3.0/24\n\n1.2.3.4\n  5.6.7.8  \n1.2.3.0/24   # trailing comment\n2001:db8::/32\n2001:db8::1\n",
			want: map[string]struct{}{
				"1.2.3.0/24":      {},
				"1.2.3.4/32":      {},
				"5.6.7.8/32":      {},
				"2001:db8::/32":   {},
				"2001:db8::1/128": {},
			},
		},
		{name: "junk line fails the list", text: "1.2.3.0/24\nnot-a-cidr\n", wantErr: true},
		{name: "empty list fails", text: "# nothing here\n\n", wantErr: true},
		{
			name:        "broad and private prefixes skipped",
			text:        "0.0.0.0/1\n10.0.0.0/8\n192.168.1.0/24\n8.8.8.8\n::/0\nfd00::/8\n",
			want:        map[string]struct{}{"8.8.8.8/32": {}},
			wantSkipped: 5,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, skipped, err := parsePrefixList(tc.text)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skipped != tc.wantSkipped {
				t.Fatalf("skipped = %d, want %d", skipped, tc.wantSkipped)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parsed %v, want %v", got, tc.want)
			}
			for k := range tc.want {
				if _, ok := got[netip.MustParsePrefix(k)]; !ok {
					t.Fatalf("missing prefix %s in %v", k, got)
				}
			}
		})
	}
}

func TestCoveredBy(t *testing.T) {
	wl := map[netip.Prefix]struct{}{
		netip.MustParsePrefix("203.0.113.0/24"): {},
		netip.MustParsePrefix("2001:db8::/64"):  {},
	}
	tests := []struct {
		name string
		p    string
		want bool
	}{
		{"exact match", "203.0.113.0/24", true},
		{"contained", "203.0.113.5/32", true},
		{"partial overlap kept", "203.0.113.0/23", false},
		{"other family", "203.0.114.0/24", false},
		{"v6 contained", "2001:db8::1/128", true},
		{"v6 partial kept", "2001:db8::/56", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := coveredBy(netip.MustParsePrefix(tc.p), wl); got != tc.want {
				t.Fatalf("coveredBy(%s) = %v, want %v", tc.p, got, tc.want)
			}
		})
	}
}

func TestCollect(t *testing.T) {
	origFetch := httpFetch
	t.Cleanup(func() { httpFetch = origFetch })

	bodies := map[string]string{
		"https://x/one":    "# scanners\n1.2.3.0/24\n203.0.113.5/32\n1.2.3.0/24\n",
		"https://x/two":    "4.4.4.4\n2001:db8::/32\n",
		"https://x/broken": "",
	}
	httpFetch = func(url string) ([]byte, error) {
		body, ok := bodies[url]
		if !ok {
			t.Fatalf("unexpected source %s", url)
		}
		if url == "https://x/broken" {
			return nil, errors.New("HTTP 503")
		}
		return []byte(body), nil
	}

	t.Run("dedupe skip whitelist count families", func(t *testing.T) {
		cfg := Config{
			Sources:   []string{"https://x/one", "https://x/two", "   "},
			Whitelist: []string{"203.0.113.0/24"},
		}
		v4, v6, skipped, err := Collect(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(v4) != 2 || len(v6) != 1 {
			t.Fatalf("v4 = %v, v6 = %v", v4, v6)
		}
		if skipped != 1 {
			t.Fatalf("skipped = %d, want 1 (whitelisted 203.0.113.5/32)", skipped)
		}
	})

	t.Run("broken source aborts", func(t *testing.T) {
		if _, _, _, err := Collect(Config{Sources: []string{"https://x/broken"}}); err == nil || !strings.Contains(err.Error(), "source fetch failed") {
			t.Fatalf("broken source must abort with a fetch error, got %v", err)
		}
	})

	t.Run("no sources fails", func(t *testing.T) {
		if _, _, _, err := Collect(Config{}); err == nil {
			t.Fatal("empty sources must fail")
		}
	})
}
