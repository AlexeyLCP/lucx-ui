// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"bytes"
	"testing"
)

func TestTLSSession_I1IsAClientHelloCarryingTheSNI(t *testing.T) {
	for _, br := range []BrowserProfile{BrowserChrome, BrowserFirefox, BrowserSafari} {
		t.Run(string(br), func(t *testing.T) {
			set := tlsSession("example.com", br)
			chi := clientHelloInfo(t, materialise(t, set[0]))
			if chi == nil {
				t.Fatal("no TLS stack could parse I1 as a ClientHello")
			}
			if chi.ServerName != "example.com" {
				t.Fatalf("ServerName = %q, want example.com", chi.ServerName)
			}
			if len(chi.CipherSuites) == 0 {
				t.Fatal("ClientHello parsed with no cipher suites")
			}
		})
	}
}

// The holes are the point: a byte-identical burst every handshake is itself a
// fingerprint. The SNI must survive unchanged, since it is what the mimicry is.
func TestTLSSession_I1VariesPerSendButKeepsItsIdentity(t *testing.T) {
	set := tlsSession("example.com", BrowserChrome)
	a, b := materialise(t, set[0]), materialise(t, set[0])
	if len(a) != len(b) {
		t.Fatalf("materialised lengths differ: %d vs %d — holes must be fixed width", len(a), len(b))
	}
	if bytes.Equal(a, b) {
		t.Fatal("two sends produced identical bytes: I1 carries no <r> holes")
	}
	for n, raw := range [][]byte{a, b} {
		chi := clientHelloInfo(t, raw)
		if chi == nil {
			t.Fatalf("draw %d: redrawn I1 no longer parses (len %d)", n, len(raw))
		}
		if chi.ServerName != "example.com" {
			t.Fatalf("draw %d: SNI became %q", n, chi.ServerName)
		}
	}
}

// I2-I5 are encrypted records: an observer cannot verify them, so they are all
// hole and cost almost nothing against the netlink budget.
func TestTLSSession_ContinuationRecordsAreCheapAndVary(t *testing.T) {
	set := tlsSession("example.com", BrowserChrome)
	for n := 1; n < 5; n++ {
		if set[n].Len() == 0 {
			t.Fatalf("I%d is empty; a full set was asked for", n+1)
		}
		if set[n].Len() > 64 {
			t.Fatalf("I%d costs %d descriptor chars; a continuation record should be tiny", n+1, set[n].Len())
		}
		if bytes.Equal(materialise(t, set[n]), materialise(t, set[n])) {
			t.Fatalf("I%d does not vary between sends", n+1)
		}
	}
}
