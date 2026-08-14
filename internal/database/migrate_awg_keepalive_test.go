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

func TestMigrateClientKeepAliveColumnType_SQLiteNoOp(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	if err := migrateClientKeepAliveColumnType(); err != nil {
		t.Fatalf("migrate on sqlite: %v", err)
	}
	if !GetDB().Migrator().HasColumn(&model.ClientRecord{}, "wg_keep_alive") {
		t.Fatal("wg_keep_alive column missing after migration")
	}
}

func TestClientKeepAlive_ReadsLegacyIntegerColumn(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

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
	if loaded.KeepAlive.Int() != 25 {
		t.Fatalf("KeepAlive.Int() = %d, want 25", loaded.KeepAlive.Int())
	}

	if err := GetDB().Table("legacy_clients").Where("id = ?", 1).
		Update("wg_keep_alive", model.KeepAliveValue("15-25")).Error; err != nil {
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

func TestMigrateClientKeepAliveColumnType_Postgres(t *testing.T) {
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
	if err := GetDB().Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive DROP DEFAULT`).Error; err != nil {
		t.Fatalf("drop default: %v", err)
	}
	if err := GetDB().Exec(
		`ALTER TABLE clients ALTER COLUMN wg_keep_alive TYPE bigint USING COALESCE(NULLIF(split_part(wg_keep_alive::text, '-', 1), ''), '0')::bigint`,
	).Error; err != nil {
		t.Fatalf("narrow column to bigint: %v", err)
	}

	if err := migrateClientKeepAliveColumnType(); err != nil {
		t.Fatalf("migrate on postgres: %v", err)
	}
	var dataType string
	if err := GetDB().Raw(
		`SELECT data_type FROM information_schema.columns WHERE table_name = 'clients' AND column_name = 'wg_keep_alive'`,
	).Scan(&dataType).Error; err != nil {
		t.Fatalf("read data_type: %v", err)
	}
	if dataType != "text" {
		t.Fatalf("data_type = %q, want \"text\"", dataType)
	}

	row := &model.ClientRecord{Email: "legacy-pg@example.com", KeepAlive: "15-25"}
	if err := GetDB().Create(row).Error; err != nil {
		t.Fatalf("create client with range keepalive: %v", err)
	}
	t.Cleanup(func() { GetDB().Exec(`DELETE FROM clients WHERE email = 'legacy-pg@example.com'`) })
	var loaded model.ClientRecord
	if err := GetDB().First(&loaded, row.Id).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	if loaded.KeepAlive != "15-25" {
		t.Fatalf("KeepAlive = %q, want \"15-25\"", loaded.KeepAlive)
	}

	if err := migrateClientKeepAliveColumnType(); err != nil {
		t.Fatalf("migrate on postgres (2nd): %v", err)
	}
	if err := GetDB().Raw(
		`SELECT data_type FROM information_schema.columns WHERE table_name = 'clients' AND column_name = 'wg_keep_alive'`,
	).Scan(&dataType).Error; err != nil {
		t.Fatalf("read data_type after 2nd run: %v", err)
	}
	if dataType != "text" {
		t.Fatalf("data_type after 2nd run = %q, want \"text\" (idempotent)", dataType)
	}
}
