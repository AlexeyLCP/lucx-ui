// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupImportSources copies the server conf, matched client files and extra
// sidecars (e.g. Amnezia clientsTable) into x-ui-backup/import-<unix>-<id>/
// BEFORE adopt overwrites or moves anything.
func BackupImportSources(c ImportCandidate) (string, error) {
	dir := filepath.Join(awgBackupDir(), importBackupName(c.ID, time.Now().Unix()))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	seen := map[string]struct{}{}
	copyOne := func(src string) error {
		src = strings.TrimSpace(src)
		if src == "" {
			return nil
		}
		if _, ok := seen[src]; ok {
			return nil
		}
		seen[src] = struct{}{}
		data, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		dst := uniqueBackupName(dir, filepath.Base(src))
		return os.WriteFile(dst, data, 0o600)
	}
	if err := copyOne(c.ConfPath); err != nil {
		return dir, fmt.Errorf("server conf %s: %w", c.ConfPath, err)
	}
	for _, k := range c.Keys {
		if err := copyOne(k.Path); err != nil {
			return dir, fmt.Errorf("client %s: %w", k.Path, err)
		}
	}
	for _, p := range c.ExtraPaths {
		if err := copyOne(p); err != nil {
			return dir, fmt.Errorf("extra %s: %w", p, err)
		}
	}
	return dir, nil
}

func importBackupName(id string, ts int64) string {
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '-'
		default:
			return r
		}
	}, id)
	if safe == "" {
		safe = "awg"
	}
	return fmt.Sprintf("import-%d-%s", ts, safe)
}

func uniqueBackupName(dir, base string) string {
	dst := filepath.Join(dir, base)
	if _, err := os.Stat(dst); err != nil {
		return dst
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(cand); err != nil {
			return cand
		}
	}
}
