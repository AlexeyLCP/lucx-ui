// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import "testing"

func TestCsqttMemberVkHashesNormalises(t *testing.T) {
	cfg := CsqttConfig{VkHashes: " aaaaaaaaaaaaaaaa,\nbbbbbbbbbbbbbbbb  cccccccccccccccc+dddddddddddddddd "}
	want := "aaaaaaaaaaaaaaaa,bbbbbbbbbbbbbbbb,cccccccccccccccc,dddddddddddddddd"
	if got := cfg.MemberVkHashes(); got != want {
		t.Fatalf("MemberVkHashes() = %q, want %q", got, want)
	}
	if got := (CsqttConfig{}).MemberVkHashes(); got != "" {
		t.Fatalf("empty MemberVkHashes() = %q", got)
	}
}

func TestCsqttMemberVkTokenYieldsToHashes(t *testing.T) {
	if !(CsqttConfig{VkTokenInLink: true}).MemberVkToken() {
		t.Fatal("token requested without hashes must be used")
	}
	if (CsqttConfig{VkTokenInLink: true, VkHashes: "aaaaaaaaaaaaaaaa"}).MemberVkToken() {
		t.Fatal("explicit VK hashes must win over the token")
	}
	if (CsqttConfig{}).MemberVkToken() {
		t.Fatal("token used without being requested")
	}
	both := CsqttConfig{ListenAddr: csqttDefaultListen, VkTokenInLink: true, VkHashes: "aaaaaaaaaaaaaaaa"}
	if err := both.Validate(); err != nil {
		t.Fatalf("hashes plus token must not disable the core: %v", err)
	}
}
