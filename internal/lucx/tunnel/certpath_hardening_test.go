// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

// Hardening follow-up for the private cert-path report (GHSA-577x): Windows
// non-local paths (UNC, device, NTFS streams) must be refused, the type check
// must happen on the OPENED handle (TOCTOU), FIFO opens must not block, the
// keypair parses from already-read bytes, and anytls/trusttunnel log why they
// disabled themselves.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// testSelfSignedPair mints one self-signed cert/key under dir and returns
// their paths plus the PEM bytes.
func certpathTestPair(t *testing.T, dir, san string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "certpath.test"},
		DNSNames:     []string{san},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func TestReadRegularFileReadsTheValidatedPath(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := certpathTestPair(t, dir, "t.test")
	cert, key, err := validatePEMCertBytes("anytls", certPath, keyPath, "t.test")
	if err != nil {
		t.Fatalf("valid pair rejected: %v", err)
	}
	if len(cert) == 0 || len(key) == 0 || !strings.Contains(string(key), "PRIVATE KEY") {
		t.Fatal("validated bytes missing")
	}
	if _, err := readRegularFile(filepath.Join(dir, "no.pem"), maxCertFileBytes); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestOpenCertFileDoesNotBlockOnFIFO(t *testing.T) {
	if !isPipesAvailable() {
		t.Skip("FIFOs unavailable on this platform")
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "cert.pem")
	if err := makeFIFO(fifo); err != nil {
		t.Skipf("mkfifo failed: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- readRegularFileAsyncErr(fifo, 1<<20)
		close(done)
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted as a cert file")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readRegularFile blocked on a FIFO — O_NONBLOCK missing")
	}
}

func TestLocalPath(t *testing.T) {
	if rejectLocalPath(filepath.Join(t.TempDir(), "cert.pem")) != nil {
		t.Fatal("local absolute cert path rejected")
	}
}

func TestIsWindowsLocalAbsLexical(t *testing.T) {
	// Runs on every OS: the check is lexical, the runtime gate lives in
	// rejectLocalPath. A test built with GOOS=windows additionally asserts the
	// real path (TestRejectsNonLocalPathsOnWindows).
	accept := []string{`C:\etc\x-ui\cert.pem`, `C:/etc/x-ui/cert.pem`, `c:\x-ui\NAMED.pem`, `C:\etc\COM10.pem`}
	for _, p := range accept {
		// NUL.pem-like device names are refused even though the drive shape
		// is right; strip device names first to assert the shape logic only.
		if strings.Contains(strings.ToUpper(p), "NUL") {
			continue
		}
		if !isWindowsLocalAbs(p) {
			t.Errorf("lexically local %q rejected", p)
		}
	}
	refuse := []string{
		`\\server\share\cert.pem`, // UNC
		`\\.\pipe\x-ui`,           // named pipe
		`\\?\C:\etc\cert.pem`,     // device path form
		`C:\etc\cert.pem:stream`,  // NTFS stream
		`C:\etc\NUL.pem`,          // reserved device name
		`C:\etc\com1\cert.pem`,    // reserved name as a directory
		`C:\etc\nul .pem`,         // Win32 drops the space before the dot
		`C:\etc\COM0.pem`,         // 0 and superscript digits are reserved too
		`C:\etc\LPT0.pem`,
		`C:\etc\COM¹.pem`,
		`C:\etc\LPT³.pem`,
		`C:\etc\CONIN$.pem`, // console devices
		`C:\etc\CONOUT$.pem`,
		`C:relative.pem`,      // drive-relative
		`/home/x-ui/cert.pem`, // POSIX form, not windows-local
	}
	for _, p := range refuse {
		if isWindowsLocalAbs(p) {
			t.Errorf("non-local %q accepted", p)
		}
	}
}

// --- platform helpers for the FIFO probe ---

func isPipesAvailable() bool {
	return runtime.GOOS != "windows" // POSIX has FIFOs, Windows has none
}

func readRegularFileAsyncErr(path string, limit int64) error {
	_, err := readRegularFile(path, limit)
	return err
}

// TestRejectsNonLocalPathsOnWindows is the GOOS=windows gate: UNC, \\.\pipe,
// NTFS-stream and device-named paths must never reach os.Stat (the network or
// the pipe server). Reporter-supplied scenario, mirrored locally.
func TestRejectsNonLocalPathsOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only scenario")
	}
	refuse := []string{
		`\\server\share\cert.pem`,
		`\\.\pipe\x-ui`,
		`C:\etc\cert.pem:stream`,
		`C:\etc\NUL.pem`,
	}
	for _, p := range refuse {
		if _, err := readRegularFile(p, maxCertFileBytes); err == nil {
			t.Errorf("non-local path %q accepted", p)
		}
	}
}

// The check trims the path, so the open must use the trimmed path too: " /abs/c.pem"
// would otherwise pass the check and open a cwd-relative file.
func TestReadRegularFileOpensTheCheckedPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.pem")
	if err := os.WriteFile(p, []byte("pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := readRegularFile(" "+p+" ", maxCertFileBytes)
	if err != nil || string(b) != "pem" {
		t.Fatalf("readRegularFile(padded) = %q, %v; want \"pem\", nil", b, err)
	}
}

// No certificate or key is empty, while a read of a zero-size /proc file can wait forever.
func TestEmptyFileRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.pem")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegularFile(p, maxCertFileBytes); err == nil || err.Error() != "file is empty" {
		t.Fatalf("readRegularFile(empty) = %v, want \"file is empty\"", err)
	}
}

// The site check must name the real reason: a relative dir is not a ".." problem.
func TestRequireIndexHTMLNamesTheReason(t *testing.T) {
	want := "tproxy: site directory: path must be absolute"
	if runtime.GOOS == "windows" {
		want = "tproxy: site directory: path must be an absolute local drive-letter path"
	}
	if err := RequireIndexHTML("site"); err == nil || err.Error() != want {
		t.Fatalf("RequireIndexHTML(\"site\") = %v, want %q", err, want)
	}
}
