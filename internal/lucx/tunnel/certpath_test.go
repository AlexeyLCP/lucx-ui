// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectLocalPath(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "cert.pem")
	if err := rejectLocalPath(abs); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"cert.pem",
		abs + "\nroot * /etc",
		abs + "\x00",
		abs + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "etc",
	} {
		if err := rejectLocalPath(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestValidateCertFilesRejectsUnsafePath(t *testing.T) {
	err := ValidateCertFiles("rel.pem", filepath.Join(t.TempDir(), "k.pem"), "vpn.example.com")
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative cert path: %v", err)
	}
	raw := filepath.Join(t.TempDir(), "sub") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "x.pem"
	err = ValidateCertFiles(raw, filepath.Join(t.TempDir(), "k.pem"), "vpn.example.com")
	if err == nil || !strings.Contains(err.Error(), "..") {
		t.Fatalf("dotdot cert path: %v", err)
	}
}

func TestRequireIndexHTMLRejectsUnsafeDir(t *testing.T) {
	if err := RequireIndexHTML("site"); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative site dir: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "site") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "other"
	if err := RequireIndexHTML(dir); err == nil || !strings.Contains(err.Error(), "..") {
		t.Fatalf("dotdot site dir: %v", err)
	}
}
