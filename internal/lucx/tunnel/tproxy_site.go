// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const tproxySiteZipMax = 20 << 20

func RequireIndexHTML(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("tproxy: site directory is empty")
	}
	if err := rejectLocalPath(dir); err != nil {
		return fmt.Errorf("tproxy: site directory: %w", err)
	}
	index := filepath.Join(dir, "index.html")
	if !strings.HasPrefix(filepath.Clean(index), filepath.Clean(dir)+string(os.PathSeparator)) {
		return errors.New("tproxy: site directory must be an absolute path without ..")
	}
	st, err := os.Stat(index)
	if err != nil || st.IsDir() {
		return errors.New("tproxy: site needs index.html")
	}
	return nil
}

// ExtractTproxySiteZip unpacks zipBytes into dest, rejecting zip-slip and
// symlinks. dest is replaced. index.html must be present at the root or in a
// single top-level folder.
func ExtractTproxySiteZip(dest string, zipBytes []byte) error {
	if len(zipBytes) == 0 || int64(len(zipBytes)) > tproxySiteZipMax {
		return errors.New("tproxy: site zip must be 1 B .. 20 MB")
	}
	r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dest), ".site-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := extractSiteZip(stage, r); err != nil {
		return err
	}
	return ReplaceSiteDir(dest, stage)
}

func extractSiteZip(dest string, r *zip.Reader) error {
	var total int64
	for _, f := range r.File {
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			continue
		}
		name := filepath.ToSlash(f.Name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			return errors.New("tproxy: zip path escapes destination")
		}
		out := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(out, dest+string(os.PathSeparator)) && out != dest {
			return errors.New("tproxy: zip path escapes destination")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rc, tproxySiteZipMax+1))
		rc.Close()
		if err != nil {
			return err
		}
		total += int64(len(data))
		if total > tproxySiteZipMax {
			return errors.New("tproxy: extracted site exceeds 20 MB")
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
	}
	if RequireIndexHTML(dest) == nil {
		return nil
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		nested := filepath.Join(dest, entries[0].Name())
		if RequireIndexHTML(nested) == nil {
			tmp := dest + ".flat"
			_ = os.RemoveAll(tmp)
			if err := os.Rename(nested, tmp); err != nil {
				return err
			}
			_ = os.RemoveAll(dest)
			return os.Rename(tmp, dest)
		}
	}
	return errors.New("tproxy: zip needs index.html")
}

func ReplaceSiteDir(dest, stage string) error {
	backup, err := os.MkdirTemp(filepath.Dir(dest), ".site-previous-")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	hadOld := false
	if err := os.Rename(dest, backup); err == nil {
		hadOld = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stage, dest); err != nil {
		if hadOld {
			if restoreErr := os.Rename(backup, dest); restoreErr != nil {
				return fmt.Errorf("replace site: %w; previous site remains at %s: %w", err, backup, restoreErr)
			}
		}
		return err
	}
	if hadOld {
		return os.RemoveAll(backup)
	}
	return nil
}

func ListTproxySite(id int) []string {
	return ListSiteFiles(TproxySiteDir(id))
}

func ListSiteFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		if len(out) >= 40 {
			return filepath.SkipAll
		}
		return nil
	})
	return out
}
