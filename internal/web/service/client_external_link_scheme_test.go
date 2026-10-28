// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import "testing"

// Issue #145: external client links must accept any scheme://value URI, not
// only schemes the panel can parse (openflux, snolc, ... were rejected with
// "unsupported or invalid share link").
func TestNormalizeExternalLinksAcceptsUnknownSchemes(t *testing.T) {
	enable := true
	rows, err := normalizeExternalLinks([]ExternalLinkInput{
		{Kind: "link", Value: "openflux://v1/AZY....fV19", Enable: &enable},
		{Kind: "link", Value: "snolc://some-host:443#me", Enable: &enable},
		{Kind: "", Value: "vless://uuid@example.com:443", Enable: &enable},
		{Kind: "subscription", Value: "https://sub.example.com/s", Enable: &enable},
	})
	if err != nil {
		t.Fatalf("unknown schemes must be accepted: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("want 4 rows, got %d", len(rows))
	}
}

func TestNormalizeExternalLinksStillRejectsGarbage(t *testing.T) {
	enable := true
	for _, bad := range []string{"not a link at all", "//no-scheme.example.com/x", "1nvalid://x", "vmess:///"} {
		if _, err := normalizeExternalLinks([]ExternalLinkInput{{Kind: "link", Value: bad, Enable: &enable}}); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	if _, err := normalizeExternalLinks([]ExternalLinkInput{{Kind: "subscription", Value: "ftp://example.com/s", Enable: &enable}}); err == nil {
		t.Error("non-http(s) subscription must be rejected")
	}
}
