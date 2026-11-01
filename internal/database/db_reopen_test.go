package database

import (
	"path/filepath"
	"testing"
	"time"
)

// A replaced pool that stays open keeps its database file open; Windows then
// cannot delete or replace that file.
func TestInitDBClosesThePoolItReplaces(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("first InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	replaced, err := GetDB().DB()
	if err != nil {
		t.Fatalf("first pool: %v", err)
	}

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("second InitDB: %v", err)
	}
	if err := replaced.Ping(); err == nil || err.Error() != "sql: database is closed" {
		t.Fatalf("replaced pool Ping() = %v, want sql: database is closed", err)
	}
}

func TestCloseDBWaitsForBackgroundJobs(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	done := BeginBackgroundDB()
	closed := make(chan error, 1)
	go func() { closed <- CloseDB() }()
	select {
	case err := <-closed:
		done()
		t.Fatalf("closed before background job finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	done()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}
