// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"
)

// clientHelloInfo feeds raw bytes to a real TLS server and returns what it
// parsed. A hand-rolled ClientHello no TLS stack accepts is not mimicry, so
// this is the only assertion about these packets that means anything.
func clientHelloInfo(t *testing.T, raw []byte) *tls.ClientHelloInfo {
	t.Helper()
	client, server := net.Pipe()
	var got *tls.ClientHelloInfo
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer server.Close()
		cfg := &tls.Config{GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
			got = chi
			return nil, errors.New("captured")
		}}
		_ = tls.Server(server, cfg).Handshake()
	}()
	_ = client.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, _ = client.Write(raw)
	_ = client.Close()
	<-done
	return got
}

// Chrome's two GREASE extension types are drawn independently, so a collision
// is a duplicate extension type — rejected outright, on roughly 6% of draws.
// 200 rounds puts the odds of missing a regression near zero.
func TestTLSPacket_ParsesAsAClientHelloEveryDraw(t *testing.T) {
	for _, br := range []BrowserProfile{BrowserChrome, BrowserFirefox, BrowserSafari} {
		t.Run(string(br), func(t *testing.T) {
			for i := 0; i < 200; i++ {
				chi := clientHelloInfo(t, materialise(t, tlsSession("example.com", br)[0]))
				if chi == nil {
					t.Fatalf("draw %d: no TLS stack could parse this ClientHello", i)
				}
				if chi.ServerName != "example.com" {
					t.Fatalf("draw %d: ServerName = %q — the SNI is the whole point of the mimicry", i, chi.ServerName)
				}
			}
		})
	}
}
