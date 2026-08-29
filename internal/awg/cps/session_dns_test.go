// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"bytes"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

// parseQuery is the only assertion about a DNS packet that means anything: a
// real parser reads it back as the query it claims to be.
func parseQuery(t *testing.T, raw []byte) dnsmessage.Question {
	t.Helper()
	var p dnsmessage.Parser
	hdr, err := p.Start(raw)
	if err != nil {
		t.Fatalf("not a DNS message: %v", err)
	}
	if hdr.Response {
		t.Fatal("a client sends queries, not responses")
	}
	if !hdr.RecursionDesired {
		t.Fatal("RD is not set; a stub resolver always sets it")
	}
	q, err := p.Question()
	if err != nil {
		t.Fatalf("no question section: %v", err)
	}
	return q
}

func TestDNSSession_PacketsParseAsQueries(t *testing.T) {
	set := dnsSession("example.com", RegionWorld)
	for n, d := range set {
		if d.Len() == 0 {
			t.Fatalf("I%d is empty", n+1)
		}
		q := parseQuery(t, materialise(t, d))
		if q.Class != dnsmessage.ClassINET {
			t.Fatalf("I%d: class %v, want IN", n+1, q.Class)
		}
		if n == 0 && q.Name.String() != "example.com." {
			t.Fatalf("I1 asks for %q, want the requested domain", q.Name.String())
		}
	}
}

// The transaction id is the one field a resolver redraws per query, and the
// one an observer cannot check. Everything else identifies the query and must
// stay put.
func TestDNSSession_OnlyTheTransactionIDVaries(t *testing.T) {
	set := dnsSession("example.com", RegionWorld)
	a, b := materialise(t, set[0]), materialise(t, set[0])
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	if bytes.Equal(a[:2], b[:2]) {
		t.Fatal("the transaction id is frozen; every query replays one id")
	}
	if !bytes.Equal(a[2:], b[2:]) {
		t.Fatal("something past the transaction id varies between sends")
	}
	if parseQuery(t, a).Name.String() != parseQuery(t, b).Name.String() {
		t.Fatal("the queried name changed between sends")
	}
}
