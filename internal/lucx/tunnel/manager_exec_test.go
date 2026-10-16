// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"
)

type execBitsInfo struct{ mode os.FileMode }

func (f execBitsInfo) Name() string       { return "bin" }
func (f execBitsInfo) Size() int64        { return 1 }
func (f execBitsInfo) Mode() os.FileMode  { return f.mode }
func (f execBitsInfo) ModTime() time.Time { return time.Time{} }
func (f execBitsInfo) IsDir() bool        { return false }
func (f execBitsInfo) Sys() any           { return nil }

var _ fs.FileInfo = execBitsInfo{}

func TestEnsureExecutable(t *testing.T) {
	chmodErr := errors.New("chmod: operation not permitted")
	cases := []struct {
		name       string
		mode       os.FileMode
		chmodFails bool
		wantErr    error
	}{
		// Regression for the tester flood: an exec-flagged binary owned by a
		// foreign uid (release tar, tar --same-owner) must start without a
		// chmod attempt, even though the sandbox cannot chmod it.
		{"executable skips chmod", 0o755, true, nil},
		{"user-exec skips chmod", 0o744, true, nil},
		{"no exec bits needs chmod", 0o644, false, nil},
		{"chmod failure is fatal when needed", 0o644, true, chmodErr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := ensureExecutable("bin", execBitsInfo{mode: tc.mode}, func(string, os.FileMode) error {
				calls++
				if tc.chmodFails {
					return chmodErr
				}
				return nil
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			want := 0
			if tc.mode.Perm()&0o111 == 0 {
				want = 1
			}
			if calls != want {
				t.Fatalf("chmod calls = %d, want %d", calls, want)
			}
		})
	}
}
