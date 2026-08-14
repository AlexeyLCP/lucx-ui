// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"log"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func migrateClientKeepAliveColumnType() error {
	if !IsPostgres() {
		return nil
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.ClientRecord{}) {
		return nil
	}
	if !migrator.HasColumn(&model.ClientRecord{}, "wg_keep_alive") {
		return nil
	}
	var dataType string
	if err := db.Raw(
		`SELECT data_type FROM information_schema.columns WHERE table_name = 'clients' AND column_name = 'wg_keep_alive'`,
	).Scan(&dataType).Error; err != nil {
		return err
	}
	if dataType == "text" || dataType == "character varying" {
		return nil
	}
	if err := db.Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive DROP DEFAULT`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive TYPE text USING wg_keep_alive::text`).Error; err != nil {
		return err
	}
	if err := db.Exec(`UPDATE clients SET wg_keep_alive = '0' WHERE wg_keep_alive IS NULL`).Error; err != nil {
		return err
	}
	if err := db.Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive SET DEFAULT '0'`).Error; err != nil {
		return err
	}
	log.Printf("[LUCX-AWG] migration: converted clients.wg_keep_alive from %s to text", dataType)
	return nil
}
