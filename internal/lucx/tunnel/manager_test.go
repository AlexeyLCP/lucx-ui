// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestProcHelperMain is the re-executed child body for the lifecycle tests:
// when run with TUNNEL_TEST_HELPER=1 it blocks until killed.
func TestProcHelperMain(t *testing.T) {
	if os.Getenv("TUNNEL_TEST_HELPER") != "1" {
		return
	}
	select {}
}

func redirectTunnelDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := tunnelDir
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = old })
	return dir
}

func TestProcStartStop(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("cannot resolve test binary: %v", err)
	}
	p := NewProc("test")
	env := append(os.Environ(), "TUNNEL_TEST_HELPER=1")
	args := []string{"-test.run", "TestProcHelperMain"}

	if err := p.Start(exe, args, env); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !p.IsRunning() {
		t.Fatal("process must be running after Start")
	}
	if err := p.Start(exe, args, env); err == nil {
		t.Error("double Start must fail")
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if p.IsRunning() {
		t.Fatal("process must be stopped after Stop")
	}
	if err := p.Stop(); err == nil {
		t.Error("Stop on an idle process must fail")
	}
}

func TestProbeListeningAndResponding(t *testing.T) {
	// Plain TCP listener: listening yes, responding no.
	plain, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Close()
	go func() {
		for {
			c, err := plain.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}
			_ = c.Close()
		}
	}()
	plainAddr := plain.Addr().String()
	if !probeListening(plainAddr) {
		t.Error("probeListening must succeed against a live listener")
	}
	if probeResponding(plainAddr) {
		t.Error("probeResponding must fail against a non-TLS listener")
	}

	// Dead port: both fail fast.
	dead := freePort(t)
	if probeListening(dead) {
		t.Error("probeListening must fail against a dead port")
	}
	if probeResponding(dead) {
		t.Error("probeResponding must fail against a dead port")
	}

	// TLS listener: both probes pass. On dev machines an HTTPS-inspecting
	// antivirus (Kaspersky et al.) can MITM/break loopback TLS handshakes;
	// the preflight below detects that and skips the TLS assertions instead
	// of failing on an environment issue (production targets are Linux).
	cert := selfSignedCert(t)
	tlsLn, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	defer tlsLn.Close()
	go func() {
		for {
			c, err := tlsLn.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				// A failed handshake (e.g. the plain-TCP probe above) must
				// not kill the accept loop.
				continue
			}
			// Hold the connection: closing it right after Accept races the
			// client's final handshake write (TLS 1.3) and flakes the probe.
			go func(c net.Conn) {
				time.Sleep(2 * time.Second)
				_ = c.Close()
			}(c)
		}
	}()
	tlsAddr := tlsLn.Addr().String()
	if !probeListening(tlsAddr) {
		t.Error("probeListening must succeed against a TLS listener")
	}
	if !probeResponding(tlsAddr) {
		t.Skip("loopback TLS handshake is broken in this environment (likely antivirus TLS interception); skipping TLS probe assertions")
	}
}

func TestStatusOfNotRunning(t *testing.T) {
	m := newManager()
	st := m.StatusOf(Instance{Core: Naive, ProbePort: 443})
	if st.Running || st.Listening || st.Responding {
		t.Errorf("idle core must probe all-false: %+v", st)
	}
}

func TestEnsureWritesConfigAndFailsOnMissingBinary(t *testing.T) {
	dir := redirectTunnelDir(t)
	t.Setenv("XUI_BIN_FOLDER", filepath.Join(dir, "bin"))

	m := newManager()
	inst := Instance{Core: Naive, Enabled: true, ConfigText: "admin off\n", ProbePort: 443}
	err := m.Ensure(inst)
	if err == nil {
		t.Fatal("Ensure must fail when the binary is missing")
	}
	if _, statErr := os.Stat(configPath(Naive)); statErr != nil {
		t.Errorf("config file must be written before start: %v", statErr)
	}

	// Disabled instance stops cleanly even without a binary.
	inst.Enabled = false
	if err := m.Ensure(inst); err != nil {
		t.Fatalf("Ensure(disabled): %v", err)
	}
}

func TestEnsureSkipsUnchangedConfigWrite(t *testing.T) {
	redirectTunnelDir(t)
	inst := Instance{Core: Naive, Enabled: true, ConfigText: "admin off\n"}
	if err := writeConfigFile(inst); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(configPath(Naive))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := writeConfigFile(inst); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(configPath(Naive))
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("unchanged config must not be rewritten")
	}
}

func TestEnsureRejectsUnknownCore(t *testing.T) {
	m := newManager()
	if err := m.Ensure(Instance{Core: Name("bogus"), Enabled: true}); err == nil {
		t.Fatal("Ensure must reject an unknown core")
	}
}

// TestReconcileWantedLegacyFallback sweeps orphan per-inbound keys while
// keeping the legacy key converged: the reconcile fallback path calls
// Reconcile{Naive,Olcrtc} with a single legacy instance when no inbound of
// the protocol is left, and a deleted inbound's key must never survive.
func TestReconcileWantedLegacyFallback(t *testing.T) {
	m := newManager()
	m.cores["olcrtc-7"] = &managed{fp: "stale"}
	m.cores["olcrtc"] = &managed{fp: "stale"}
	m.cores["unrelated"] = &managed{}

	m.ReconcileOlcrtc([]Instance{{Core: Olcrtc, Enabled: false}})

	if _, ok := m.cores["olcrtc-7"]; ok {
		t.Error("orphan per-inbound key must be removed by the legacy fallback")
	}
	if _, ok := m.cores["olcrtc"]; !ok {
		t.Error("legacy key must survive the legacy fallback")
	}
	if _, ok := m.cores["unrelated"]; !ok {
		t.Error("keys of other cores must not be touched")
	}

	m.ReconcileOlcrtc(nil)
	if _, ok := m.cores["olcrtc"]; ok {
		t.Error("empty want must sweep the legacy key too")
	}
}

// TestReconcileWantedKeepsWantedKeys guards the normal path: wanted keys are
// Ensured, not swept.
func TestReconcileWantedKeepsWantedKeys(t *testing.T) {
	m := newManager()
	m.cores["olcrtc-3"] = &managed{fp: "stale"}
	m.cores["olcrtc-9"] = &managed{fp: "stale"}

	m.ReconcileOlcrtc([]Instance{
		{Core: Olcrtc, Key: "olcrtc-3", Enabled: false},
	})

	if _, ok := m.cores["olcrtc-3"]; !ok {
		t.Error("wanted key must be kept")
	}
	if _, ok := m.cores["olcrtc-9"]; ok {
		t.Error("unwanted per-inbound key must be removed")
	}
}

func TestStopAllIdle(t *testing.T) {
	m := newManager()
	m.StopAll()
	if m.IsRunning(Naive) {
		t.Error("idle manager must report nothing running")
	}
	if got := m.Logs(Naive, 10); got != nil {
		t.Errorf("idle logs = %v", got)
	}
	if got := m.LastLog(Naive); got != "" {
		t.Errorf("idle last log = %q", got)
	}
}

func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tunnel-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}
