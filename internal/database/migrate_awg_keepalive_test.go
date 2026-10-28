// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func setupKeepAliveDB(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
}

// The pre-lucx.329 storage: ranges live inside wg_keep_alive (TEXT). The
// split must relocate them into wg_keep_alive_range and normalize numerics,
// keeping every value readable through the computed KeepAlive field.
func TestMigrateClientKeepAliveRangeSplit(t *testing.T) {
	setupKeepAliveDB(t)
	if !GetDB().Migrator().HasTable(&model.ClientRecord{}) ||
		!GetDB().Migrator().HasColumn(&model.ClientRecord{}, "wg_keep_alive") {
		t.Fatal("clients table with wg_keep_alive must exist after InitDB")
	}
	if err := GetDB().Exec(
		`INSERT INTO clients (id, email, wg_keep_alive, wg_keep_alive_range) VALUES
			(1, 'range@example.com', '15-25', ''),
			(2, 'int@example.com', '25', ''),
			(3, 'zero@example.com', '0', '')`,
	).Error; err != nil {
		t.Fatalf("seed rows: %v", err)
	}
	_ = GetDB().Exec(`DELETE FROM client_inbounds WHERE client_id IN (1,2,3)`)

	if err := migrateClientKeepAliveRangeSplit(); err != nil {
		t.Fatalf("split migration: %v", err)
	}

	var rangeRow model.ClientRecord
	if err := GetDB().Table("clients").Where("id = 1").First(&rangeRow).Error; err != nil {
		t.Fatalf("load range row: %v", err)
	}
	if rangeRow.KeepAliveRange != "15-25" || rangeRow.KeepAliveLegacy != 0 {
		t.Fatalf("range not moved: legacy=%d range=%q", rangeRow.KeepAliveLegacy, rangeRow.KeepAliveRange)
	}
	if rangeRow.KeepAlive != "15-25" {
		t.Fatalf("computed KeepAlive = %q, want \"15-25\"", rangeRow.KeepAlive)
	}

	var intRow model.ClientRecord
	if err := GetDB().Table("clients").Where("id = 2").First(&intRow).Error; err != nil {
		t.Fatalf("load int row: %v", err)
	}
	if intRow.KeepAliveLegacy != 25 || intRow.KeepAliveRange != "" {
		t.Fatalf("numeric row changed: legacy=%d range=%q", intRow.KeepAliveLegacy, intRow.KeepAliveRange)
	}

	// Idempotent second run: nothing breaks.
	if err := migrateClientKeepAliveRangeSplit(); err != nil {
		t.Fatalf("second split run: %v", err)
	}
}

func TestClientKeepAlive_ReadsLegacyIntegerColumn(t *testing.T) {
	setupKeepAliveDB(t)

	if err := GetDB().Exec(
		`CREATE TABLE legacy_clients (id INTEGER PRIMARY KEY, email TEXT, wg_keep_alive INTEGER DEFAULT 0)`,
	).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := GetDB().Exec(
		`INSERT INTO legacy_clients (id, email, wg_keep_alive) VALUES
			(1, 'legacy@example.com', 25),
			(2, 'other@example.com', 0)`,
	).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	var page []model.ClientRecord
	if err := GetDB().Table("legacy_clients").Find(&page).Error; err != nil {
		t.Fatalf("Find slice (clients page): %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("Find = %d rows, want 2", len(page))
	}

	var loaded model.ClientRecord
	if err := GetDB().Table("legacy_clients").Where("id = ?", 1).First(&loaded).Error; err != nil {
		t.Fatalf("load client with integer keepalive: %v", err)
	}
	if loaded.KeepAlive != "25" {
		t.Fatalf("KeepAlive = %q, want \"25\"", loaded.KeepAlive)
	}
	if loaded.KeepAlive.Seconds() != 25 {
		t.Fatalf("KeepAlive.Seconds() = %d, want 25", loaded.KeepAlive.Seconds())
	}

	if err := GetDB().Exec(
		`ALTER TABLE legacy_clients ADD COLUMN wg_keep_alive_range TEXT DEFAULT ''`,
	).Error; err != nil {
		t.Fatalf("add range column: %v", err)
	}
	if err := GetDB().Exec(
		`UPDATE legacy_clients SET wg_keep_alive = 0, wg_keep_alive_range = '15-25' WHERE id = 1`,
	).Error; err != nil {
		t.Fatalf("store range keepalive: %v", err)
	}
	var reloaded model.ClientRecord
	if err := GetDB().Table("legacy_clients").Where("id = ?", 1).First(&reloaded).Error; err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if reloaded.KeepAlive != "15-25" {
		t.Fatalf("KeepAlive = %q, want \"15-25\"", reloaded.KeepAlive)
	}
}

func TestMigrateClientKeepAliveRangeSplit_Postgres(t *testing.T) {
	if os.Getenv("XUI_TEST_PG_MUTATION") != "1" ||
		strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" ||
		os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_TEST_PG_MUTATION=1 XUI_DB_TYPE=postgres XUI_DB_DSN=... to run")
	}
	if err := InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	GetDB().Exec(`DELETE FROM clients WHERE email = 'legacy-pg@example.com'`)
	// Recreate a pre-lucx.329 text column: ranges inline.
	GetDB().Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive TYPE text USING COALESCE(split_part(wg_keep_alive::text, '-', 1), '0')`)

	row := &model.ClientRecord{Email: "legacy-pg@example.com", KeepAliveRange: "15-25"}
	if err := GetDB().Create(row).Error; err != nil {
		t.Fatalf("create client with range keepalive: %v", err)
	}
	t.Cleanup(func() { GetDB().Exec(`DELETE FROM clients WHERE email = 'legacy-pg@example.com'`) })
	GetDB().Exec(`UPDATE clients SET wg_keep_alive = '15-25' WHERE email = 'legacy-pg@example.com'`)

	if err := migrateClientKeepAliveRangeSplit(); err != nil {
		t.Fatalf("split on postgres: %v", err)
	}
	var dataType string
	if err := GetDB().Raw(
		`SELECT data_type FROM information_schema.columns WHERE table_name = 'clients' AND column_name = 'wg_keep_alive'`,
	).Scan(&dataType).Error; err != nil {
		t.Fatalf("read data_type: %v", err)
	}
	if dataType != "integer" && dataType != "bigint" {
		t.Fatalf("data_type = %q, want integer (column narrowed back)", dataType)
	}
	var loaded model.ClientRecord
	if err := GetDB().First(&loaded, row.Id).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	if loaded.KeepAlive != "15-25" {
		t.Fatalf("KeepAlive = %q, want \"15-25\" (value preserved)", loaded.KeepAlive)
	}
}