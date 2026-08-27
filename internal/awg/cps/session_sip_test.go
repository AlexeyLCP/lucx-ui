// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"strings"
	"testing"
)

func headerLine(t *testing.T, msg, name string) string {
	t.Helper()
	for _, line := range strings.Split(msg, "\r\n") {
		if strings.HasPrefix(line, name) {
			return line
		}
	}
	t.Fatalf("no %q header in:\n%s", name, msg)
	return ""
}

// SIP is plaintext all the way, so almost everything in it is verifiable and
// must stay literal. The registration sequence is what makes it a session.
func TestSIPSession_IsARegistrationSequence(t *testing.T) {
	set := sipSession("sip.example.com")
	i1 := string(materialise(t, set[0]))
	i2 := string(materialise(t, set[1]))
	i3 := string(materialise(t, set[2]))

	if !strings.HasPrefix(i1, "REGISTER sip:") {
		t.Fatalf("I1 is not a REGISTER: %.40q", i1)
	}
	if !strings.HasSuffix(i1, "\r\n\r\n") {
		t.Fatal("I1 does not end with a blank line")
	}
	if !strings.Contains(i2, "Authorization: Digest ") {
		t.Fatal("I2 must be the authenticated retry")
	}
	if !strings.HasPrefix(i3, "OPTIONS sip:") {
		t.Fatalf("I3 is not an OPTIONS ping: %.40q", i3)
	}
	for n, msg := range map[string]string{"I2": i2, "I3": i3} {
		if got, want := headerLine(t, msg, "Call-ID:"), headerLine(t, i1, "Call-ID:"); got != want {
			t.Fatalf("%s belongs to another dialog: %q vs %q", n, got, want)
		}
	}
	if a, b := headerLine(t, i1, "CSeq:"), headerLine(t, i2, "CSeq:"); a == b {
		t.Fatalf("the retry reuses CSeq %q", a)
	}
}

// RFC 5626 keepalive: four bytes, and nothing else.
func TestSIPSession_KeepalivesAreBareCRLF(t *testing.T) {
	set := sipSession("sip.example.com")
	for _, n := range []int{3, 4} {
		if got := string(materialise(t, set[n])); got != "\r\n\r\n" {
			t.Fatalf("I%d = %q, want a bare double CRLF", n+1, got)
		}
	}
}

// The branch identifies a transaction and is expected to differ every time;
// the Call-ID identifies the dialog and must not.
func TestSIPSession_BranchVariesCallIDDoesNot(t *testing.T) {
	set := sipSession("sip.example.com")
	a, b := string(materialise(t, set[0])), string(materialise(t, set[0]))
	if headerLine(t, a, "Via:") == headerLine(t, b, "Via:") {
		t.Fatal("the Via branch is frozen; every send replays one transaction id")
	}
	if headerLine(t, a, "Call-ID:") != headerLine(t, b, "Call-ID:") {
		t.Fatal("the Call-ID changed between sends; I1 and I2 would stop being one dialog")
	}
}
