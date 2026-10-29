// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"log"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// migrateClientKeepAliveRangeSplit adapts pre-lucx.329 LucX databases to the
// upstream v3.11.0 storage scheme. Those rows once stored AWG ranges
// ("15-25") in wg_keep_alive (a TEXT column after our old widen migration);
// the v3.11 ClientRecord reads that column into a plain int and would fail
// on every range row. The split moves each stored VALUE — no value changes,
// only the column it lives in (Rule 0: values preserved):
//
//	numeric ("25")      -> stays in wg_keep_alive (readable as int)
//	range ("15-25")/zero-range ("0-0","" already empty) -> wg_keep_alive_range
//
// Idempotent: numeric/range/empty inputs are classified, only non-numeric
// values move.
func migrateClientKeepAliveRangeSplit() error {
	migrator := db.Migrator()
	if !migrator.HasTable(&model.ClientRecord{}) || !migrator.HasColumn(&model.ClientRecord{}, "wg_keep_alive") {
		return nil
	}
	if !migrator.HasColumn(&model.ClientRecord{}, "wg_keep_alive_range") {
		return logAndReturn("wg_keep_alive_range column missing before the range split")
	}
	var rows []struct {
		Id     int64  `gorm:"column:id"`
		Column string `gorm:"column:v"`
	}
	if err := db.Raw(`SELECT id, wg_keep_alive AS v FROM clients`).Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		value := strings.TrimSpace(r.Column)
		if value == "" || isNumericKeepAlive(value) {
			continue
		}
		if err := db.Exec(`UPDATE clients SET wg_keep_alive = '0', wg_keep_alive_range = ? WHERE id = ?`, value, r.Id).Error; err != nil {
			return err
		}
		log.Printf("[LUCX-AWG] migration: client %d keepAlive %q moved to wg_keep_alive_range", r.Id, value)
	}
	// Postgres: the old widen made the column text; upstream v3.11 wants int
	// back. Numeric-only content is guaranteed by the split above.
	if IsPostgres() {
		var dataType string
		if err := db.Raw(
			`SELECT data_type FROM information_schema.columns WHERE table_name = 'clients' AND column_name = 'wg_keep_alive'`,
		).Scan(&dataType).Error; err != nil {
			return err
		}
		if dataType == "text" || dataType == "character varying" {
			if err := db.Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive DROP DEFAULT`).Error; err != nil {
				return err
			}
			if err := db.Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive TYPE integer USING wg_keep_alive::integer`).Error; err != nil {
				return err
			}
			if err := db.Exec(`ALTER TABLE clients ALTER COLUMN wg_keep_alive SET DEFAULT 0`).Error; err != nil {
				return err
			}
			log.Printf("[LUCX-AWG] migration: clients.wg_keep_alive back to integer (%s -> integer)", dataType)
		}
	}
	return nil
}

func isNumericKeepAlive(value string) bool {
	n, err := strconv.Atoi(value)
	return err == nil && strconv.Itoa(n) == value
}

func logAndReturn(msg string) error {
	log.Printf("[LUCX-AWG] migration: %s", msg)
	return nil
}
