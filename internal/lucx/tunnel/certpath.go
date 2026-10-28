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
// rejected as well. Purely lexical so tests can run on any OS.
func isWindowsLocalAbs(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
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
		if part == "" {
			continue
		}
		base := strings.ToUpper(part)
		if i := strings.IndexByte(base, '.'); i > 0 {
			base = base[:i]
		}
		switch base {
		case "CON", "PRN", "AUX", "NUL",
			"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
			"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
			return false
		}
	}
	return true
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
		return errors.New("path must be a local drive-letter path")
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
	if err := rejectLocalPath(path); err != nil {
		return nil, err
	}
	// Open first, then re-check type and size on the OPENED handle: the file
	// may be swapped between a Stat and the read (TOCTOU).
	f, err := openRegularFileNonBlock(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if st.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
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
