// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"
)

func TestDefaultOlcrtcConfig(t *testing.T) {
	cfg := DefaultOlcrtcConfig()
	if cfg.Provider != "jitsi" || cfg.Transport != "datachannel" {
		t.Fatalf("defaults = %+v", cfg)
	}
	if cfg.DNS != "8.8.8.8:53" || cfg.VP8Fps != 60 || cfg.VP8Batch != 64 {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestOlcrtcValidate(t *testing.T) {
	base := func() OlcrtcConfig {
		c := DefaultOlcrtcConfig()
		c.RoomID = "https://meet.example.org/room"
		c.CryptoKey = strings.Repeat("ab", 32)
		return c
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*OlcrtcConfig)
	}{
		{"no room", func(c *OlcrtcConfig) { c.RoomID = " " }},
		{"bad provider", func(c *OlcrtcConfig) { c.Provider = "zoom" }},
		{"bad transport", func(c *OlcrtcConfig) { c.Transport = "seichannel" }},
		{"telemost+datachannel", func(c *OlcrtcConfig) { c.Provider = "telemost"; c.Transport = "datachannel" }},
		{"dns bare ip", func(c *OlcrtcConfig) { c.DNS = "8.8.8.8" }},
		{"bad key", func(c *OlcrtcConfig) { c.CryptoKey = "short" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	// Empty key is OK at Validate — EnsureCryptoKey fills it on save.
	emptyKey := base()
	emptyKey.CryptoKey = ""
	if err := emptyKey.Validate(); err != nil {
		t.Fatalf("empty key must pass Validate: %v", err)
	}
}

func TestOlcrtcRenderYAML(t *testing.T) {
	cfg := DefaultOlcrtcConfig()
	cfg.RoomID = "https://meet.example.org/r"
	cfg.CryptoKey = strings.Repeat("cd", 32)
	cfg.Debug = true
	got := cfg.RenderYAML("/tmp/olcrtc-data")
	for _, want := range []string{
		"mode: srv",
		`provider: "jitsi"`,
		`id: "https://meet.example.org/r"`,
		`key: "` + cfg.CryptoKey + `"`,
		`transport: "datachannel"`,
		`dns: "8.8.8.8:53"`,
		`data: "/tmp/olcrtc-data"`,
		"debug: true",
		"interval: 10s",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("YAML missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "vp8:") {
		t.Errorf("datachannel must not render vp8 block:\n%s", got)
	}

	cfg.Transport = "vp8channel"
	cfg.VP8Fps = 30
	cfg.VP8Batch = 16
	got = cfg.RenderYAML("")
	for _, want := range []string{
		`transport: "vp8channel"`,
		"vp8:",
		"fps: 30",
		"batch_size: 16",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("vp8 YAML missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "data:") {
		t.Errorf("empty dataDir must omit data key:\n%s", got)
	}
}

func TestOlcrtcClientURI(t *testing.T) {
	cfg := DefaultOlcrtcConfig()
	cfg.RoomID = "https://meet.example.org/r"
	cfg.CryptoKey = strings.Repeat("ef", 32)
	got := cfg.ClientURI()
	want := "olcrtc://jitsi?datachannel@https://meet.example.org/r#" + cfg.CryptoKey
	if got != want {
		t.Fatalf("URI = %q, want %q", got, want)
	}

	cfg.Transport = "vp8channel"
	cfg.VP8Fps = 60
	cfg.VP8Batch = 64
	got = cfg.ClientURI()
	if !strings.Contains(got, "vp8channel<vp8-fps=60&vp8-batch=64>") {
		t.Fatalf("vp8 URI = %q", got)
	}

	cfg.RoomID = ""
	if cfg.ClientURI() != "" {
		t.Fatal("empty room must yield empty URI")
	}
}

func TestOlcrtcEnsureCryptoKey(t *testing.T) {
	cfg := DefaultOlcrtcConfig()
	cfg.RoomID = "room"
	got, err := cfg.EnsureCryptoKey()
	if err != nil {
		t.Fatal(err)
	}
	if !validCryptoKey(got.CryptoKey) {
		t.Fatalf("generated key invalid: %q", got.CryptoKey)
	}
	// Second call keeps the existing key.
	keep := got.CryptoKey
	got2, err := got.EnsureCryptoKey()
	if err != nil || got2.CryptoKey != keep {
		t.Fatalf("EnsureCryptoKey must keep existing key, got %q", got2.CryptoKey)
	}
}

func TestOlcrtcYAMLEscapes(t *testing.T) {
	cfg := DefaultOlcrtcConfig()
	cfg.RoomID = `room"quote`
	cfg.CryptoKey = strings.Repeat("11", 32)
	got := cfg.RenderYAML("")
	if !strings.Contains(got, `id: "room\"quote"`) {
		t.Fatalf("room quote not escaped:\n%s", got)
	}
}
