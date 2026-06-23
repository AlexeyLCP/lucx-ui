// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import "testing"

func TestStripHiddenKeys_RemovesBoth(t *testing.T) {
	in := `{"privateKey":"k","hiddenSOCKSPort":10801,"hiddenInboundTag":"awg-hidden-1","jc":8}`
	out := stripHiddenKeys(in)
	if contains(out, "hiddenSOCKSPort") || contains(out, "hiddenInboundTag") {
		t.Fatalf("hidden keys not stripped: %s", out)
	}
	if !contains(out, "privateKey") || !contains(out, "jc") {
		t.Fatalf("legitimate keys dropped: %s", out)
	}
}

func TestStripHiddenKeys_NoopWhenAbsent(t *testing.T) {
	in := `{"privateKey":"k","jc":8}`
	out := stripHiddenKeys(in)
	if out != in {
		t.Fatalf("expected unchanged, got %s", out)
	}
}

func TestStripHiddenKeys_NoopOnBadJSON(t *testing.T) {
	in := "not json"
	out := stripHiddenKeys(in)
	if out != in {
		t.Fatalf("expected unchanged on bad json, got %s", out)
	}
}

func TestStripHiddenKeys_NoopOnEmpty(t *testing.T) {
	out := stripHiddenKeys("")
	if out != "" {
		t.Fatalf("expected empty unchanged, got %s", out)
	}
}

func TestStripHiddenKeys_OnlyOnePresent(t *testing.T) {
	in := `{"hiddenSOCKSPort":10801,"jc":8}`
	out := stripHiddenKeys(in)
	if contains(out, "hiddenSOCKSPort") {
		t.Fatalf("hiddenSOCKSPort not stripped: %s", out)
	}
	if !contains(out, "jc") {
		t.Fatalf("jc dropped: %s", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}