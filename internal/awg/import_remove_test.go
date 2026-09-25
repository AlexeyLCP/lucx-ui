// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveImportCandidateMovesConf(t *testing.T) {
	dir := t.TempDir()
	orig := awgConfigDir
	awgConfigDir = dir
	t.Cleanup(func() { awgConfigDir = orig })
	conf := filepath.Join(dir, "awg0.conf")
	if err := os.WriteFile(conf, []byte("[Interface]\nPrivateKey = a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveImportCandidate(ImportCandidate{
		Source:   ImportSourceConf,
		Ifname:   "awg0",
		ConfPath: conf,
		Backend:  "userspace",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(conf); !os.IsNotExist(err) {
		t.Fatal("conf still in scan dir")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "x-ui-backup"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("backup: %v %v", entries, err)
	}
}

func TestRemoveImportCandidateRefusesManaged(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "awg1.conf")
	body := xuiManagedMarker + "\n[Interface]\nPrivateKey = a\n"
	if err := os.WriteFile(conf, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveImportCandidate(ImportCandidate{Source: ImportSourceConf, ConfPath: conf}); err == nil {
		t.Fatal("expected refuse")
	}
	if _, err := os.Stat(conf); err != nil {
		t.Fatal("managed conf was removed")
	}
}
