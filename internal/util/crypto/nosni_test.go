// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

func domainCert(t *testing.T, name string) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func ipCert(t *testing.T, ips []net.IP, cn string) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: cn}, IPAddresses: ips,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// handshakeCN does a real TLS handshake and returns the certificate's CN.
func handshakeCN(t *testing.T, cfg *tls.Config, sni string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = tls.Server(conn, cfg).Handshake()
			conn.Close()
		}
	}()
	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{InsecureSkipVerify: true, ServerName: sni})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return conn.ConnectionState().PeerCertificates[0].Subject.CommonName
}

func TestHideCertWithoutSNI(t *testing.T) {
	cfg := &tls.Config{Certificates: []tls.Certificate{domainCert(t, "sub.example.com")}}
	if got := handshakeCN(t, cfg, ""); got != "sub.example.com" {
		t.Fatalf("precondition: stock config shows %q", got)
	}
	if err := HideCertWithoutSNI(cfg); err != nil {
		t.Fatal(err)
	}
	for sni, want := range map[string]string{
		"":                "localhost", // a visit by bare IP: no domain leaked
		"sub.example.com": "sub.example.com",
		"other.example":   "sub.example.com", // unknown name: the real one, as before
	} {
		if got := handshakeCN(t, cfg, sni); got != want {
			t.Errorf("SNI %q: certificate %q, want %q", sni, got, want)
		}
	}
}

// A certificate carrying the visited IP in SAN (a Let's Encrypt cert issued
// to the IP) must survive the no-SNI hide: the visit is by that very IP, the
// operator configured the cert for it. Tester's report lucx.352: .350/.351
// served the self-signed localhost pair on every bare-IP visit of the panel.
func TestHideCertWithoutSNIKeepsIPCert(t *testing.T) {
	cfg := &tls.Config{Certificates: []tls.Certificate{
		ipCert(t, []net.IP{net.ParseIP("127.0.0.1")}, "127.0.0.1"),
	}}
	if err := HideCertWithoutSNI(cfg); err != nil {
		t.Fatal(err)
	}
	for sni, want := range map[string]string{
		"":        "127.0.0.1", // no SNI, the visit is by the covered IP
		"1.2.3.4": "127.0.0.1", // IP-literal SNI to an uncovered IP: hidden
	} {
		if got := handshakeCN(t, cfg, sni); got != want {
			t.Errorf("SNI %q: certificate %q, want %q", sni, got, want)
		}
	}
}

// A cert whose SAN names IPs but not the visiting one stays hidden; a hello
// carrying a covered IP literal picks the real cert. stdlib clients drop the
// SNI extension for IP literals, so the IP-name case is exercised on the
// closure directly (a hello with no Conn, like probes that send SNI=IP).
func TestHideCertWithoutSNIMismatchedIP(t *testing.T) {
	cfg := &tls.Config{Certificates: []tls.Certificate{
		ipCert(t, []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("203.0.113.5")}, "mixed"),
	}}
	if err := HideCertWithoutSNI(cfg); err != nil {
		t.Fatal(err)
	}
	// Real handshakes: every dial lands on 127.0.0.1, which the cert does
	// not cover — always hidden.
	for sni := range map[string]string{"": "", "1.2.3.4": "", "9.9.9.9": ""} {
		if got := handshakeCN(t, cfg, sni); got != "localhost" {
			t.Errorf("SNI %q: certificate %q, want localhost", sni, got)
		}
	}
	for sni, want := range map[string]string{
		"203.0.113.5": "mixed",     // covered IP
		"1.2.3.4":     "mixed",     // covered IP
		"8.8.8.8":     "localhost", // uncovered IP
		"":            "localhost", // no name and no conn — nothing to match
	} {
		c, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: sni})
		if err != nil {
			t.Fatalf("SNI %q: %v", sni, err)
		}
		leaf, perr := x509.ParseCertificate(c.Certificate[0])
		if perr != nil {
			t.Fatal(perr)
		}
		if got := leaf.Subject.CommonName; got != want {
			t.Errorf("SNI %q: certificate %q, want %q", sni, got, want)
		}
	}
}
