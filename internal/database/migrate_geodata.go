// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"errors"
	"log"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx"

	"gorm.io/gorm"
)

func migrateLucxGeodataAssets() error {
	var setting model.Setting
	err := db.Where("key = ?", "xrayTemplateConfig").First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	updated, changed, err := lucx.ApplyGeodata(setting.Value)
	if err != nil {
		log.Printf("[LUCX] geodata assets: skip (invalid xrayTemplateConfig): %v", err)
		return nil
	}
	if !changed {
		return nil
	}
	log.Printf("[LUCX] geodata assets: seeded IR/RU/ROSCOM into xrayTemplateConfig")
	return db.Model(&model.Setting{}).Where("key = ?", "xrayTemplateConfig").Update("value", updated).Error
}
