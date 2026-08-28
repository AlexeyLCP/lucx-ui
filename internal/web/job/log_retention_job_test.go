// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func seedFile(t *testing.T, dir, name string, age time.Duration, now time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("log line\n"), 0o644); err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
	mtime := now.Add(-age)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", name, err)
	}
	return path
}

func TestPruneLogFilesOlderThan_DeletesOnlyStaleFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	oldRotated := seedFile(t, dir, "3xui.log-2026-07-01T00-00-00.000.gz", 30*24*time.Hour, now)
	oldCrash := seedFile(t, dir, "core_crash_1754006400.log", 20*24*time.Hour, now)
	freshFile := seedFile(t, dir, "access.log", time.Hour, now)
	protectedActive := seedFile(t, dir, "3xui.log", 40*24*time.Hour, now)
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	protected := map[string]bool{protectedActive: true}
	removed, err := pruneLogFilesOlderThan(dir, 14, now, protected)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	for _, gone := range []string{oldRotated, oldCrash} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s should be deleted, stat err = %v", gone, err)
		}
	}
	for _, kept := range []string{freshFile, protectedActive} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("%s should survive, stat err = %v", kept, err)
		}
	}
}

func TestPruneLogFilesOlderThan_MissingDirIsNotAnError(t *testing.T) {
	removed, err := pruneLogFilesOlderThan(filepath.Join(t.TempDir(), "no-such-dir"), 7, time.Now(), nil)
	if err != nil {
		t.Fatalf("prune missing dir: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
}

func TestProtectedLogPaths_IncludesActiveChain(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("XUI_LOG_FOLDER", logDir)
	writeLogConfig(t, filepath.Join(logDir, "access.log"), filepath.Join(logDir, "error.log"))

	protected := protectedLogPaths()
	for _, name := range []string{"3xui.log", "3xipl.log", "3xipl-banned.log", "3xipl-banned.prev.log", "access.log", "error.log"} {
		abs, err := filepath.Abs(filepath.Join(logDir, name))
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		if !protected[abs] {
			t.Errorf("protected set missing %s", name)
		}
	}
}
