// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// migrateAwgOutbounds creates the awg_outbounds table. Idempotent: GORM
// AutoMigrate is a no-op when the table already exists with the same columns.
func migrateAwgOutbounds() error {
	return db.AutoMigrate(&model.AwgOutbound{})
}