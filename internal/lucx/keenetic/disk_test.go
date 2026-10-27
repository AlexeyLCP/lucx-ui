// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package keenetic

import (
	"errors"
	"testing"
)

func TestParseOPKGDisk_MapKey(t *testing.T) {
	used, total, ok := parseOPKGDisk(
		[]byte(`{"disk":"mydisk"}`),
		[]byte(`{"mydisk:":{"free":1048576,"total":2097152}}`),
	)
	if !ok || used != 1048576 || total != 2097152 {
		t.Fatalf("got %d/%d ok=%v, want 1048576/2097152", used, total, ok)
	}
}

func TestParseOPKGDisk_EntryLabel(t *testing.T) {
	used, total, ok := parseOPKGDisk(
		[]byte(`{"disk":"storage:/"}`),
		[]byte(`{"entry":{"a:":{"label":"storage","free":10,"total":40}}}`),
	)
	if !ok || used != 30 || total != 40 {
		t.Fatalf("got %d/%d ok=%v, want 30/40", used, total, ok)
	}
}

func TestParseOPKGDisk_RejectsGarbage(t *testing.T) {
	if _, _, ok := parseOPKGDisk([]byte(`{}`), []byte(`{}`)); ok {
		t.Fatal("empty disk label must not match")
	}
	if _, _, ok := parseOPKGDisk([]byte(`{"disk":"rom"}`), []byte(`{"other:":{"free":1,"total":2}}`)); ok {
		t.Fatal("unrelated volume must not match")
	}
	if _, _, ok := parseOPKGDisk([]byte(`{"disk":"rom"}`), []byte(`{"rom:":{"free":5,"total":4}}`)); ok {
		t.Fatal("free > total must not match")
	}
}

func TestUsageFromRCI(t *testing.T) {
	get := func(path string) ([]byte, error) {
		switch path {
		case "/show/sc/opkg/disk":
			return []byte(`{"disk":"rom"}`), nil
		case "/ls":
			return []byte(`{"rom:":{"free":1,"total":4}}`), nil
		default:
			return nil, errors.New(path)
		}
	}
	used, total, ok := usageFromRCI(get)
	if !ok || used != 3 || total != 4 {
		t.Fatalf("got %d/%d ok=%v", used, total, ok)
	}
}

func TestOPKGUsage_NotEntware(t *testing.T) {
	if entware() {
		t.Skip("this host has Entware")
	}
	if _, _, ok := OPKGUsage(); ok {
		t.Fatal("OPKGUsage must stay off when /opt/bin/opkg is absent")
	}
}
