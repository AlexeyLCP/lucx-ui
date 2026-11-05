// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLookupIPProviderWithoutLocalDatabases(t *testing.T) {
	defer func(v bool) { ipDBDownloads = v }(ipDBDownloads)
	ipDBDownloads = false
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if got := LookupIPProvider("8.8.8.8"); got.ASN != 0 || got.Org != "" || got.Country != "" || got.City != "" || got.Source != "" {
		t.Errorf("no local database: want an empty label, got %+v", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("downloads off: lookup must not create files, found %v", entries)
	}
	// An unreadable file is skipped too.
	_ = os.WriteFile(filepath.Join(dir, "GeoLite2-City.mmdb"), []byte("not a database"), 0o644)
	if got := LookupIPProvider("8.8.8.8"); got.Country != "" {
		t.Errorf("broken file: want an empty label, got %+v", got)
	}
}

func TestIPDBSource(t *testing.T) {
	if ipDBSource("GeoLite2-City.mmdb") != "maxmind" || ipDBSource("dbip-city-lite.mmdb") != "dbip" {
		t.Error("wrong database source names")
	}
}

func TestIPDBDirsAndFreshestCopy(t *testing.T) {
	db, bin := t.TempDir(), t.TempDir()
	t.Setenv("XUI_DB_FOLDER", db)
	t.Setenv("XUI_BIN_FOLDER", bin)
	if d := ipDBDirs(); len(d) != 2 || d[0] != db || d[1] != bin {
		t.Fatalf("want the database folder first (download target), then bin: %v", d)
	}
	old := filepath.Join(bin, "dbip-city-lite.mmdb")
	_ = os.WriteFile(old, []byte("x"), 0o644)
	if p, _ := freshestPath(ipDBDirs(), "dbip-city-lite.mmdb"); p != old {
		t.Fatalf("a copy only in bin must still be used, got %q", p)
	}
	newer := filepath.Join(db, "dbip-city-lite.mmdb")
	_ = os.WriteFile(newer, []byte("x"), 0o644)
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(newer, future, future)
	if p, _ := freshestPath(ipDBDirs(), "dbip-city-lite.mmdb"); p != newer {
		t.Fatalf("the newest copy wins, got %q", p)
	}
}
