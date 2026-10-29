// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	ormlogger "gorm.io/gorm/logger"
)

// Backup = one SQLite file (GetDb). The uploaded cover / tproxy masking sites
// live only on disk, so a restored backup used to drop them. Instead of
// changing the artifact format, the site files are embedded into the backup
// copy in a dedicated table and are written back to disk on import. The live
// database never keeps the table: embed rebuilds it in the backup copy, restore
// drops it after extraction. Backups without the table (vanilla panel, older
// LucX) restore exactly as before. PostgreSQL flows are untouched: the table
// never exists in a live DB, pg_dump/pg_restore never carry it, and MigrateData
// copies only its known models into PostgreSQL.
const backupSiteFilesTable = "lucx_backup_site_files"

const (
	// Same ceiling as the tproxy site zip upload.
	backupSiteFileMax = 20 << 20
	// Safety ceiling over all sites of one backup.
	backupSiteFilesTotalMax = 64 << 20
)

// backupSiteFile is one file of an uploaded masking site inside the backup.
type backupSiteFile struct {
	// Dir is the site directory base name ("cover-3-site", "tproxy-5-site").
	Dir  string `gorm:"column:dir;primaryKey"`
	Path string `gorm:"column:path;primaryKey"`
	Data []byte `gorm:"column:data"`
}

func (backupSiteFile) TableName() string { return backupSiteFilesTable }

// siteSourceDir maps a site-serving inbound to its on-disk site directory.
func siteSourceDir(ib *model.Inbound) (string, bool) {
	if ib == nil {
		return "", false
	}
	switch ib.Protocol {
	case model.Cover:
		return tunnel.CoverSiteDir(ib.Id), true
	case model.Tproxy:
		return tunnel.TproxySiteDir(ib.Id), true
	default:
		return "", false
	}
}

// siteInbounds lists the site-serving inbounds of the given database.
func siteInbounds(db *gorm.DB) ([]*model.Inbound, error) {
	var inbounds []*model.Inbound
	err := db.Select("id, protocol").
		Where("protocol IN ?", []model.Protocol{model.Cover, model.Tproxy}).
		Find(&inbounds).Error
	return inbounds, err
}

// openSQLiteCopy opens a standalone backup file away from the live pool.
func openSQLiteCopy(path string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(path), &gorm.Config{Logger: ormlogger.Discard})
}

// embedBackupSiteFiles writes the current site files into the backup copy at
// path. The table is dropped first, so a live DB that still carries an old row
// set cannot leak stale blobs into the backup.
func embedBackupSiteFiles(backupPath string) error {
	rows, err := collectSiteFileRows()
	if err != nil {
		return err
	}
	g, err := openSQLiteCopy(backupPath)
	if err != nil {
		return err
	}
	defer func() {
		if sqlDB, dbErr := g.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	}()
	if err := g.Migrator().DropTable(backupSiteFilesTable); err != nil {
		logger.Debugf("backup: site files table not present: %v", err)
	}
	if total := totalSiteFileBytes(rows); total > backupSiteFilesTotalMax {
		return fmt.Errorf("site files total %d bytes, backup ceiling is %d", total, backupSiteFilesTotalMax)
	}
	if len(rows) == 0 {
		return nil
	}
	if err := g.AutoMigrate(&backupSiteFile{}); err != nil {
		return err
	}
	return g.CreateInBatches(rows, 50).Error
}

func totalSiteFileBytes(rows []backupSiteFile) int {
	total := 0
	for _, r := range rows {
		total += len(r.Data)
	}
	return total
}

// collectSiteFileRows walks every site-serving inbound's dir. A missing dir
// (site never uploaded) is skipped, not an error.
func collectSiteFileRows() ([]backupSiteFile, error) {
	rows := []backupSiteFile{}
	total := 0
	inbounds, err := siteInbounds(database.GetDB())
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, ib := range inbounds {
		dir, ok := siteSourceDir(ib)
		if !ok || seen[dir] {
			continue
		}
		seen[dir] = true
		var walkErr error
		rows, total, walkErr = walkOneSiteDir(dir, rows, total)
		if walkErr != nil {
			return nil, walkErr
		}
	}
	if total > backupSiteFilesTotalMax {
		return nil, fmt.Errorf("site files total %d bytes, backup ceiling is %d", total, backupSiteFilesTotalMax)
	}
	return rows, nil
}

func walkOneSiteDir(dir string, rows []backupSiteFile, total int) ([]backupSiteFile, int, error) {
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.Type()&fs.ModeSymlink != 0 || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > backupSiteFileMax {
			return fmt.Errorf("site file %s exceeds backup limit", p)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		total += len(data)
		rows = append(rows, backupSiteFile{
			Dir:  filepath.Base(dir),
			Path: filepath.ToSlash(rel),
			Data: data,
		})
		return nil
	})
	return rows, total, err
}

var errBackupSitePath = errors.New("site file path escapes its directory")

// restoreBackupSiteFiles writes embedded site files back to disk after a
// database import and drops the table from the now-live database.
func restoreBackupSiteFiles() error {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(backupSiteFilesTable) {
		return nil
	}
	var rows []backupSiteFile
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		if err := db.Migrator().DropTable(backupSiteFilesTable); err != nil {
			logger.Warningf("Import: drop %s: %v", backupSiteFilesTable, err)
		}
		return nil
	}
	// Only dirs whose inbound the restored database actually carries are
	// materialized; rows for foreign/absent inbounds would be stale orphans.
	targets := map[string]string{}
	siteInbounds, err := siteInbounds(db)
	if err != nil {
		logger.Warningf("Import: site restore skipped, cannot read inbounds: %v", err)
		return err
	}
	for _, ib := range siteInbounds {
		dir, ok := siteSourceDir(ib)
		if ok {
			targets[filepath.Base(dir)] = dir
		}
	}
	byDir := map[string][]backupSiteFile{}
	for _, r := range rows {
		byDir[r.Dir] = append(byDir[r.Dir], r)
	}
	var firstErr error
	for base, group := range byDir {
		target, ok := targets[base]
		if !ok {
			continue
		}
		if err := writeSiteFiles(target, group); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		// Dropped only on success, so a failed write keeps the data in the DB.
		if err := db.Migrator().DropTable(backupSiteFilesTable); err != nil {
			logger.Warningf("Import: drop %s: %v", backupSiteFilesTable, err)
		}
	}
	return firstErr
}

// writeSiteFiles replaces the target dir with the backup's file set. Path
// validation runs before the wipe so a corrupted backup cannot delete an
// existing dir and then fail halfway through. The check never sees a Clean()
// form: Clean collapses "/../x" into "/x" and hides the traversal.
func writeSiteFiles(target string, group []backupSiteFile) error {
	for _, f := range group {
		if err := validateBackupSitePath(filepath.ToSlash(f.Path)); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".site-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, f := range group {
		out := filepath.Join(stage, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, f.Data, 0o644); err != nil {
			return err
		}
	}
	if err := tunnel.ReplaceSiteDir(target, stage); err != nil {
		return err
	}
	logger.Infof("backup: restored %d site file(s) into %s", len(group), target)
	return nil
}

func validateBackupSitePath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" || p == "." ||
		strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) ||
		strings.Contains(p, "..") || strings.Contains(p, `\`) {
		return fmt.Errorf("%w: %q", errBackupSitePath, p)
	}
	return nil
}
