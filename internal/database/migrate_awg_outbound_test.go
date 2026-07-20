//go:build cgo

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestMigrateAwgOutbounds_Idempotent confirms migrateAwgOutbounds creates the
// awg_outbounds table and is a no-op on a second run, leaving it empty.
//
// Tagged //go:build cgo because the project's SQLite driver (mattn/go-sqlite3)
// requires cgo. On hosts without gcc the file is excluded from the build, so
// `go test` does not fail with a CGO stub error — verify on a cgo-enabled host
// or in CI.
func TestMigrateAwgOutbounds_Idempotent(t *testing.T) {
	dbDir := t.TempDir()
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if err := migrateAwgOutbounds(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrateAwgOutbounds(); err != nil {
		t.Fatalf("second migrate (idempotent): %v", err)
	}

	var count int64
	if err := db.Model(&model.AwgOutbound{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows, got %d", count)
	}

	// Sanity: the table exists and accepts a row.
	row := &model.AwgOutbound{Tag: "awgo-test", Remark: "test", Enable: true}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create row: %v", err)
	}
	if row.Id == 0 {
		t.Fatal("expected autoincrement id > 0")
	}
	var got model.AwgOutbound
	if err := db.First(&got, row.Id).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.Tag != row.Tag {
		t.Fatalf("roundtrip tag: got %q want %q", got.Tag, row.Tag)
	}
}