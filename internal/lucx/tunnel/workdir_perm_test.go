// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build unix

package tunnel

import (
	"os"
	"path/filepath"
	"testing"
)

// A fresh install creates an AnyTLS data dir before anything else exists
// under bin/: the shared tunnel dir must still come out enterable by the
// routed sidecar's own uid, while the data dir stays private.
func TestMkdirPrivateKeepsWorkDirReachable(t *testing.T) {
	root := t.TempDir()
	saved := tunnelDir
	tunnelDir = func() string { return filepath.Join(root, "bin", "tunnel") }
	t.Cleanup(func() { tunnelDir = saved })

	data := filepath.Join(workDir(), "anytls-19-data")
	if err := mkdirPrivate(data); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(workDir()); st.Mode().Perm() != 0o755 {
		t.Fatalf("workDir mode = %v, want 0755", st.Mode().Perm())
	}
	if st, _ := os.Stat(data); st.Mode().Perm() != 0o700 {
		t.Fatalf("data dir mode = %v, want 0700", st.Mode().Perm())
	}

	// A server where an older build already made it 0700 is repaired.
	if err := os.Chmod(workDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorkDirTraversable(); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(workDir()); st.Mode().Perm() != 0o711 {
		t.Fatalf("repaired mode = %v, want 0711 (enter, no listing)", st.Mode().Perm())
	}
}
