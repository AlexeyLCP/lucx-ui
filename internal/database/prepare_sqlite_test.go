package database

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPrepareSQLiteForMigration(t *testing.T) {
	t.Run("rejects non-panel database", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "random.db")
		gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		if err := gdb.Exec("CREATE TABLE notes(id integer primary key, body text)").Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
		closeGorm(gdb)

		err = PrepareSQLiteForMigration(dbPath)
		if err == nil {
			t.Fatal("PrepareSQLiteForMigration accepted a non-panel database, want error")
		}
		if !strings.Contains(err.Error(), "not a 3x-ui panel database") {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), "not a 3x-ui panel database")
		}
	})

	t.Run("upgrades old panel schema", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "old.db")
		gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		if err := gdb.AutoMigrate(&model.User{}, &model.Setting{}, &model.Inbound{}); err != nil {
			t.Fatalf("automigrate legacy subset: %v", err)
		}
		closeGorm(gdb)

		if err := PrepareSQLiteForMigration(dbPath); err != nil {
			t.Fatalf("PrepareSQLiteForMigration rejected an old panel database: %v", err)
		}

		check, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatalf("reopen sqlite: %v", err)
		}
		defer closeGorm(check)
		sqlDB, err := check.DB()
		if err != nil {
			t.Fatalf("sql db: %v", err)
		}
		for _, table := range []string{"client_groups", "client_global_traffics", "outbound_subscriptions"} {
			if !sqliteTableExists(sqlDB, table) {
				t.Errorf("table %s was not created by the schema upgrade", table)
			}
		}
	})

	t.Run("upgrades legacy clients with triggers and keepalive values", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "legacy.db")
		gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatal(err)
		}
		if err := gdb.AutoMigrate(&model.User{}, &model.Setting{}, &model.Inbound{}); err != nil {
			t.Fatal(err)
		}
		for _, query := range []string{
			`CREATE TABLE clients (id integer primary key, email text, wg_keep_alive text)`,
			`INSERT INTO clients VALUES (1, 'empty', ''), (2, 'range', '15-25')`,
			`CREATE TABLE client_inbounds (id integer primary key, client_id integer, inbound_id integer)`,
			`CREATE TRIGGER lucx_shareonly_clients_ins AFTER INSERT ON client_inbounds BEGIN UPDATE clients SET email = email WHERE id = NEW.client_id; END`,
			`CREATE TRIGGER lucx_shareonly_clients_del AFTER DELETE ON client_inbounds BEGIN UPDATE clients SET email = email WHERE id = OLD.client_id; END`,
		} {
			if err := gdb.Exec(query).Error; err != nil {
				t.Fatal(err)
			}
		}
		closeGorm(gdb)
		if err := PrepareSQLiteForMigration(dbPath); err != nil {
			t.Fatal(err)
		}
		check, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
		if err != nil {
			t.Fatal(err)
		}
		defer closeGorm(check)
		var clients []model.ClientRecord
		if err := check.Order("id").Find(&clients).Error; err != nil {
			t.Fatal(err)
		}
		if len(clients) != 2 || clients[0].KeepAliveLegacy != 0 || clients[1].KeepAliveRange != "15-25" {
			t.Fatalf("unexpected clients: %+v", clients)
		}
	})
}
