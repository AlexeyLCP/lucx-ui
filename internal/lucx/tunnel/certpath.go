// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const maxCertFileBytes = 1 << 20

// isWindowsLocalAbs lexically checks a Windows drive-letter absolute path.
// filepath.IsAbs alone is not enough on Windows: UNC (\\server\share),
// device (\\.\pipe\...) and volume-GUID (\\?\...) paths all pass it, and an
// os.Stat then reaches the network or a named-pipe server. NTFS streams
// (a second colon) and reserved device names (NUL, CON, COM1, ...) are
// rejected as well. The check is purely shape-based (no filepath.IsAbs —
// that call is OS-dependent) so tests can run on any OS.
func isWindowsLocalAbs(path string) bool {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return false
	}
	if len(path) < 3 || path[1] != ':' || (path[2] != '\\' && path[2] != '/') {
		return false
	}
	if strings.ContainsAny(path[3:], `:`) {
		return false
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if isWindowsDeviceName(part) {
			return false
		}
	}
	return true
}

// isWindowsDeviceName matches a Win32 device name whatever its extension; Win32 also
// drops spaces before the dot ("nul .pem") and reserves COM0/LPT0 and COM¹-COM³.
func isWindowsDeviceName(part string) bool {
	base, _, _ := strings.Cut(part, ".")
	base = strings.ToUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	for _, prefix := range []string{"COM", "LPT"} {
		if n, ok := strings.CutPrefix(base, prefix); ok && len([]rune(n)) == 1 && strings.ContainsAny(n, "0123456789¹²³") {
			return true
		}
	}
	return false
}

// rejectLocalPath refuses operator-supplied cert and site paths that are not a
// local absolute location. Relative paths, "..", and control characters must
// not reach os.ReadFile or a generated config.
func rejectLocalPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is empty")
	}
	if strings.ContainsAny(path, "\x00\n\r") {
		return errors.New("path contains a control character")
	}
	if runtime.GOOS == "windows" && !isWindowsLocalAbs(path) {
		return errors.New("path must be an absolute local drive-letter path")
	}
	if !filepath.IsAbs(path) {
		return errors.New("path must be absolute")
	}
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return errors.New("path must not contain ..")
		}
	}
	return nil
}

func readRegularFile(path string, limit int64) ([]byte, error) {
	path = strings.TrimSpace(path)
	if err := rejectLocalPath(path); err != nil {
		return nil, err
	}
	// Stat first so a device or FIFO is refused unopened, then re-check the OPENED
	// handle: the file may be swapped between the Stat and the open (TOCTOU).
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if err := checkRegularFile(st, limit); err != nil {
		return nil, err
	}
	f, err := openRegularFileNonBlock(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	if st, err = f.Stat(); err != nil {
		return nil, err
	}
	if err := checkRegularFile(st, limit); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return b, nil
}

// checkRegularFile also refuses an empty file: no certificate or key is empty, and a
// zero-size /proc file is pollable, so its read could wait forever.
func checkRegularFile(st os.FileInfo, limit int64) error {
	if !st.Mode().IsRegular() {
		return errors.New("not a regular file")
	}
	if st.Size() == 0 {
		return errors.New("file is empty")
	}
	if st.Size() > limit {
		return fmt.Errorf("file exceeds %d bytes", limit)
	}
	return nil
}
