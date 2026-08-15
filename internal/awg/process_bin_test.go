// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAwgBinFallsBackToNameWhenMissing(t *testing.T) {
	got := awgBin("definitely-not-installed-awg-xyz")
	if got != "definitely-not-installed-awg-xyz" {
		t.Fatalf("awgBin(missing) = %q, want the bare name", got)
	}
}

func TestAwgBinFindsAbsoluteFallback(t *testing.T) {
	dir := t.TempDir()
	name := "awg-quick-testbin"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	fake := filepath.Join(dir, name)
	if err := os.WriteFile(fake, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	look := "awg-quick-testbin"
	got := awgBin(look)
	want, err := exec.LookPath(look)
	if err != nil {
		t.Fatalf("LookPath: %v", err)
	}
	if got != want {
		t.Fatalf("awgBin = %q, want LookPath %q", got, want)
	}
}
