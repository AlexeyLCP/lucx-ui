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

func TestModuleSHA(t *testing.T) {
	orig := moduleSHAPath
	t.Cleanup(func() { moduleSHAPath = orig })

	dir := t.TempDir()
	path := filepath.Join(dir, "marker")
	moduleSHAPath = path
	if got := moduleSHA(); got != "" {
		t.Fatalf("missing marker: %q", got)
	}

	full := "3c38e168beb7c60dec41dfe423d41555205a3dac"
	if err := os.WriteFile(path, []byte(full+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := moduleSHA(); got != "3c38e168beb7" {
		t.Fatalf("sha = %q", got)
	}

	if err := os.WriteFile(path, []byte("1.0.20260611\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := moduleSHA(); got != "1.0.20260611" {
		t.Fatalf("legacy marker = %q", got)
	}
}

func TestCollectHostStatus_NonLinux(t *testing.T) {
	// On Windows CI/dev host the probe must not panic and should report empty.
	hs := CollectHostStatus()
	if hs.Interfaces < 0 {
		t.Fatalf("interfaces negative: %+v", hs)
	}
}
