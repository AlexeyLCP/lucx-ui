// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telemt

import (
	"strings"
	"testing"
)

func TestGenerateSecret(t *testing.T) {
	s1 := GenerateSecret()
	s2 := GenerateSecret()
	if len(s1) != 34 {
		t.Errorf("expected 34 chars (ee + 32 hex), got %d: %s", len(s1), s1)
	}
	if !strings.HasPrefix(s1, "ee") {
		t.Errorf("secret must start with 'ee': %s", s1)
	}
	if s1 == s2 {
		t.Error("two secrets should differ")
	}
}

func TestGenerateProxyLink(t *testing.T) {
	link := GenerateProxyLink("5.9.1.2", 443, "ee00000000000000000000000000000000")
	expected := "tg://proxy?server=5.9.1.2&port=443&secret=ee00000000000000000000000000000000"
	if link != expected {
		t.Errorf("expected %s, got %s", expected, link)
	}
}

func TestGenerateProxyLink_NonStandardPort(t *testing.T) {
	link := GenerateProxyLink("example.com", 8080, "eeabcdef1234567890abcdef1234567890")
	if !strings.Contains(link, "port=8080") {
		t.Error("link must contain port")
	}
}
