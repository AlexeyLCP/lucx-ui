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
// by bare IP), or no usable host (an IP literal as SNI), with a throwaway
// self-signed "localhost" certificate — unless the operator's own certificate
// names the visited IP in SAN (a Let's Encrypt cert issued to the IP): it is
// explicitly meant for that visit and leaks nothing beyond it. Otherwise the
// address would read the real domain from the certificate and could tie the
// address to it. Hellos that name a host still get cfg.Certificates.
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
	// The IP SANs are parsed once: every no-SNI hello re-checks coverage.
	certIPs := make([][]net.IP, len(certs))
	for i := range certs {
		if leaf, perr := x509.ParseCertificate(certs[i].Certificate[0]); perr == nil {
			certIPs[i] = leaf.IPAddresses
		}
	}
	cfg.Certificates = nil
	inner := cfg.GetCertificate
	pickReal := func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
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
	cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		name := strings.TrimSpace(hello.ServerName)
		if name != "" && net.ParseIP(strings.Trim(name, "[]")) == nil {
			return pickReal(hello)
		}
		if ip := visitingIP(hello); ip != nil {
			for i := range certIPs {
				for _, cand := range certIPs[i] {
					if cand.Equal(ip) {
						return pickReal(hello)
					}
				}
			}
		}
		return generic, nil
	}
	return nil
}

// visitingIP is the IP the hello is actually addressed to: an IP-literal SNI
// when present, otherwise the listener's local address of the underlying
// connection (a no-SNI hello is how browsers address a server by bare IP).
func visitingIP(hello *tls.ClientHelloInfo) net.IP {
	if ip := net.ParseIP(strings.TrimSpace(hello.ServerName)); ip != nil {
		return ip
	}
	if hello.Conn == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(hello.Conn.LocalAddr().String())
	if err != nil {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	return ip
}
