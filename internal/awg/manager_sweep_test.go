// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempConfigDir repoints awgConfigDir (and thus awgBackupDir) at a temp
// dir for the duration of a test, restoring the original on cleanup.
func withTempConfigDir(t *testing.T) string {
	t.Helper()
	orig := awgConfigDir
	dir := t.TempDir()
	awgConfigDir = dir
	t.Cleanup(func() { awgConfigDir = orig })
	return dir
}

func writeConf(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(awgConfigDir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// TestSweepOrphanInboundConfigs_BackupsMarkedOnly is the lucx.67 regression test
// for "LucX-UI deletes foreign AWG configs". Only configs carrying the x-ui
// ownership marker are swept, and they are moved to the backup dir rather than
// deleted. Unmarked configs (e.g. WGDashboard's awg0.conf, which shares the
// awg{N}.conf naming) and configs whose id is still wanted are left untouched.
func TestSweepOrphanInboundConfigs_BackupsMarkedOnly(t *testing.T) {
	dir := withTempConfigDir(t)

	// Marked orphan (LucX-UI created it, inbound since deleted) -> backed up.
	markedOrphan := writeConf(t, "awg7.conf", xuiManagedMarker+"\n[Interface]\nPrivateKey = x\n")
	// Unmarked orphan (foreign, e.g. WGDashboard) -> must be left alone.
	foreignOrphan := writeConf(t, "awg0.conf", "[Interface]\nPrivateKey = foreign\n")
	// Wanted config (id still in want) -> must be left alone even though marked.
	wantedConf := writeConf(t, "awg3.conf", xuiManagedMarker+"\n[Interface]\nPrivateKey = y\n")

	want := map[int]struct{}{3: {}}
	sweepOrphanInboundConfigs(want)

	// Marked orphan must be gone from the config dir but present in the backup dir.
	if _, err := os.Stat(markedOrphan); !os.IsNotExist(err) {
		t.Errorf("marked orphan awg7.conf should have been moved out of %s", dir)
	}
	backupEntries, err := os.ReadDir(awgBackupDir())
	if err != nil {
		t.Fatalf("backup dir not created: %v", err)
	}
	foundBackup := false
	for _, e := range backupEntries {
		if strings.HasPrefix(e.Name(), "awg7.conf.") {
			foundBackup = true
		}
	}
	if !foundBackup {
		t.Errorf("marked orphan awg7.conf not found in backup dir %s", awgBackupDir())
	}

	// Foreign (unmarked) orphan must still be in place, unmodified.
	data, err := os.ReadFile(foreignOrphan)
	if err != nil {
		t.Fatalf("foreign orphan awg0.conf was removed: %v", err)
	}
	if !strings.Contains(string(data), "PrivateKey = foreign") {
		t.Errorf("foreign orphan awg0.conf was modified:\n%s", data)
	}

	// Wanted config must still be in place.
	if _, err := os.Stat(wantedConf); err != nil {
		t.Errorf("wanted config awg3.conf was removed: %v", err)
	}
}

// TestConfigIsManaged verifies the marker detection, including the
// missing-file and no-marker cases that must report false.
func TestConfigIsManaged(t *testing.T) {
	withTempConfigDir(t)
	marked := writeConf(t, "awg1.conf", xuiManagedMarker+"\n[Interface]\n")
	unmarked := writeConf(t, "awg2.conf", "[Interface]\nPrivateKey = x\n")

	if !configIsManaged(marked) {
		t.Errorf("configIsManaged must be true for a marked config")
	}
	if configIsManaged(unmarked) {
		t.Errorf("configIsManaged must be false for an unmarked config")
	}
	if configIsManaged(filepath.Join(awgConfigDir, "does-not-exist.conf")) {
		t.Errorf("configIsManaged must be false for a missing file")
	}
}

func TestStrayInterfaceIsOurs(t *testing.T) {
	withTempConfigDir(t)
	writeConf(t, "awg5.conf", xuiManagedMarker+"\n[Interface]\n")
	writeConf(t, "awg0.conf", "[Interface]\nPrivateKey = foreign\n")

	if !strayInterfaceIsOurs("awg5") {
		t.Fatal("marked awg5.conf must be treated as ours")
	}
	if strayInterfaceIsOurs("awg0") {
		t.Fatal("unmarked awg0.conf must be left alone")
	}
	if strayInterfaceIsOurs("awg1") {
		t.Fatal("missing conf must be treated as foreign")
	}
}

// TestBackupConfigFile_MoveAndTimestamp confirms the file is moved into the
// backup dir under a timestamped name, and that the source is gone.
func TestBackupConfigFile_MoveAndTimestamp(t *testing.T) {
	withTempConfigDir(t)
	src := writeConf(t, "awg9.conf", xuiManagedMarker+"\n[Interface]\n")

	if err := backupConfigFile(src); err != nil {
		t.Fatalf("backupConfigFile: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should be gone after backup")
	}
	entries, err := os.ReadDir(awgBackupDir())
	if err != nil {
		t.Fatalf("backup dir not created: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "awg9.conf.") {
			found = true
		}
	}
	if !found {
		t.Errorf("backup file awg9.conf.<ts> not found in %s", awgBackupDir())
	}
}
