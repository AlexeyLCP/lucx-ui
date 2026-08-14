// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strings"
	"testing"
)

// A DKMS rebuild runs for up to 45 minutes and is chatty; the retained tail
// must stay bounded however much it prints.
func TestBoundedTailKeepsOnlyTheTail(t *testing.T) {
	tail := &boundedTail{limit: 64}
	for range 100 {
		if _, err := tail.Write([]byte(strings.Repeat("x", 32) + "\n")); err != nil {
			t.Fatalf("Write = %v", err)
		}
	}
	if got := len(tail.String()); got > 64 {
		t.Fatalf("tail holds %d bytes, want at most 64", got)
	}
}

// The end of the output is the part worth keeping: a DKMS failure always
// explains itself on the last lines.
func TestBoundedTailKeepsTheNewestBytes(t *testing.T) {
	tail := &boundedTail{limit: 32}
	if _, err := tail.Write([]byte(strings.Repeat("old", 100) + "\nfatal: no headers\n")); err != nil {
		t.Fatalf("Write = %v", err)
	}
	if !strings.Contains(tail.String(), "fatal: no headers") {
		t.Fatalf("tail = %q, want it to end with the failure line", tail.String())
	}
}

func TestBoundedTailReportsFullLength(t *testing.T) {
	tail := &boundedTail{limit: 1 << 10}
	payload := []byte("dkms: building\nmodule.ko built\n")
	n, err := tail.Write(payload)
	if err != nil {
		t.Fatalf("Write = %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write = %d, want %d", n, len(payload))
	}
}

// Output arrives in arbitrary chunks; a line split across two writes must be
// logged once, joined, not twice in halves.
func TestBoundedTailHandlesSplitLines(t *testing.T) {
	tail := &boundedTail{limit: 1 << 10}
	for _, chunk := range []string{"dkms: ", "building ", "amneziawg\n"} {
		if _, err := tail.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write(%q) = %v", chunk, err)
		}
	}
	if want := "dkms: building amneziawg\n"; tail.String() != want {
		t.Fatalf("tail = %q, want %q", tail.String(), want)
	}
}
