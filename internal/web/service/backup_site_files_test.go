// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	ormlogger "gorm.io/gorm/logger"
)

func initBackupSiteTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	t.Setenv("XUI_BIN_FOLDER", filepath.Join(dbDir, "bin"))
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
}

func seedSiteInbound(t *testing.T, protocol model.Protocol) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		Remark:   string(protocol) + "-ib",
		Tag:      string(protocol) + "in-" + fmt.Sprint(siteInboundSerial),
		Protocol: protocol,
		Port:     443,
		Enable:   true,
	}
	siteInboundSerial++
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	return ib
}

var siteInboundSerial = 1

func writeSiteFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openBackup(t *testing.T, path string) *gorm.DB {
	t.Helper()
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: ormlogger.Discard})
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := g.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return g
}

func siteRowsReady(t *testing.T, g *gorm.DB) {
	t.Helper()
	if err := g.AutoMigrate(&backupSiteFile{}); err != nil {
		t.Fatalf("migrate site files table: %v", err)
	}
	if err := g.Create(&[]backupSiteFile{
		{Dir: filepath.Base(tunnel.CoverSiteDir(1)), Path: "index.html", Data: []byte("<h1>cover</h1>")},
		{Dir: filepath.Base(tunnel.CoverSiteDir(1)), Path: "assets/app.css", Data: []byte("body{}")},
	}).Error; err != nil {
		t.Fatalf("seed rows: %v", err)
	}
}

func TestEmbedBackupSiteFiles(t *testing.T) {
	initBackupSiteTestDB(t)
	coverInbound := seedSiteInbound(t, model.Cover)
	tproxyInbound := seedSiteInbound(t, model.Tproxy)

	writeSiteFile(t, tunnel.CoverSiteDir(coverInbound.Id), "index.html", "<h1>cover</h1>")
	writeSiteFile(t, tunnel.CoverSiteDir(coverInbound.Id), "assets/app.css", "body{}")
	writeSiteFile(t, tunnel.TproxySiteDir(tproxyInbound.Id), "index.html", "<h1>tproxy</h1>")

	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := embedBackupSiteFiles(backupPath); err != nil {
		t.Fatalf("embed: %v", err)
	}

	g := openBackup(t, backupPath)
	var count int64
	if err := g.Table(backupSiteFilesTable).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 embedded files, got %d", count)
	}
	var row backupSiteFile
	base := filepath.Base(tunnel.CoverSiteDir(coverInbound.Id))
	if err := g.Where("dir = ? AND path = ?", base, "assets/app.css").First(&row).Error; err != nil {
		t.Fatalf("nested file row: %v", err)
	}
	if string(row.Data) != "body{}" {
		t.Fatalf("nested file data changed: %q", row.Data)
	}
	// A cover inbound with no uploaded site must not embed anything.
	if err := database.GetDB().Delete(&model.Inbound{}, coverInbound.Id).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	os.RemoveAll(tunnel.CoverSiteDir(coverInbound.Id))
	if err := embedBackupSiteFiles(backupPath); err != nil {
		t.Fatalf("re-embed: %v", err)
	}
	g2 := openBackup(t, backupPath)
	g2.Table(backupSiteFilesTable).Count(&count)
	if count != 1 {
		t.Fatalf("stale cover files leaked into the backup: %d rows", count)
	}
}

func TestRestoreBackupSiteFiles(t *testing.T) {
	initBackupSiteTestDB(t)
	coverInbound := seedSiteInbound(t, model.Cover)
	tproxyInbound := seedSiteInbound(t, model.Tproxy)

	g := database.GetDB()
	siteRowsReady(t, g)
	if err := g.Create(&[]backupSiteFile{
		{Dir: filepath.Base(tunnel.TproxySiteDir(tproxyInbound.Id)), Path: "index.html", Data: []byte("<h1>tproxy</h1>")},
		{Dir: "cover-999-site", Path: "index.html", Data: []byte("stale")},
	}).Error; err != nil {
		t.Fatalf("seed extra rows: %v", err)
	}

	if err := restoreBackupSiteFiles(); err != nil {
		t.Fatalf("restore: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tunnel.CoverSiteDir(coverInbound.Id), "index.html"))
	if err != nil || string(got) != "<h1>cover</h1>" {
		t.Fatalf("cover site not restored: %v %q", err, got)
	}
	nested, err := os.ReadFile(filepath.Join(tunnel.CoverSiteDir(coverInbound.Id), "assets", "app.css"))
	if err != nil || string(nested) != "body{}" {
		t.Fatalf("nested file not restored: %v %q", err, nested)
	}
	got, err = os.ReadFile(filepath.Join(tunnel.TproxySiteDir(tproxyInbound.Id), "index.html"))
	if err != nil || string(got) != "<h1>tproxy</h1>" {
		t.Fatalf("tproxy site not restored: %v %q", err, got)
	}
	if _, err := os.Stat(tunnel.CoverSiteDir(999)); !os.IsNotExist(err) {
		t.Fatal("stale dir materialized for a foreign row")
	}

	var count int64
	g.Table(backupSiteFilesTable).Count(&count)
	if count != 0 {
		t.Fatalf("table not dropped from the live db: %d rows left", count)
	}
}

func TestRestoreBackupSiteFiles_RejectsTraversal(t *testing.T) {
	initBackupSiteTestDB(t)
	coverInbound := seedSiteInbound(t, model.Cover)

	g := database.GetDB()
	if err := g.AutoMigrate(&backupSiteFile{}); err != nil {
		t.Fatal(err)
	}
	if err := g.Create(&backupSiteFile{
		Dir:  filepath.Base(tunnel.CoverSiteDir(coverInbound.Id)),
		Path: "../escape.html",
		Data: []byte("evil"),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := restoreBackupSiteFiles(); err == nil {
		t.Fatal("traversal row accepted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(tunnel.CoverSiteDir(coverInbound.Id)), "escape.html")); !os.IsNotExist(err) {
		t.Fatal("escaped file written outside the site dir")
	}
	var count int64
	g.Table(backupSiteFilesTable).Count(&count)
	if count != 1 {
		t.Fatalf("data dropped after a failed restore: %d rows left", count)
	}
}

// A pre-feature backup (no table) restores without touching the disk.
func TestRestoreBackupSiteFiles_NoTable(t *testing.T) {
	initBackupSiteTestDB(t)
	if err := restoreBackupSiteFiles(); err != nil {
		t.Fatalf("no-table restore errored: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(os.Getenv("XUI_BIN_FOLDER"), "tunnel"))
	if len(entries) != 0 {
		t.Fatalf("wrote stray dirs: %v", entries)
	}
}
