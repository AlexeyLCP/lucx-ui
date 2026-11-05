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
	"strings"
	"time"
)

// HideCertWithoutSNI makes cfg answer a TLS hello that names no host (a visit
// by bare IP, or an IP literal as the name) with a throwaway self-signed
// "localhost" certificate. Otherwise any scanner of the IP reads the real
// domain from the certificate and can tie the address to it. Hellos that
// name a host still get cfg.Certificates.
func HideCertWithoutSNI(cfg *tls.Config) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 63))
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    now.Add(-24 * time.Hour),
		NotAfter:     now.AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return err
	}
	generic := &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	// crypto/tls consults GetCertificate for a hello without SNI only when
	// Certificates is empty, so the real certificates move behind it.
	certs := cfg.Certificates
	cfg.Certificates = nil
	inner := cfg.GetCertificate
	cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		name := strings.TrimSpace(hello.ServerName)
		if name == "" || net.ParseIP(strings.Trim(name, "[]")) != nil {
			return generic, nil
		}
		if inner != nil {
			if c, err := inner(hello); c != nil || err != nil {
				return c, err
			}
		}
		for i := range certs {
			if hello.SupportsCertificate(&certs[i]) == nil {
				return &certs[i], nil
			}
		}
		if len(certs) > 0 {
			return &certs[0], nil // like crypto/tls: the first one by default
		}
		return generic, nil
	}
	return nil
}
