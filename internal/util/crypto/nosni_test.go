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
