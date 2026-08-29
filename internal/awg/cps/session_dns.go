// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import "bytes"

// dnsSession is a stub resolver's burst: the requested name, then the others a
// page load resolves alongside it. Unlike TLS or QUIC, several queries down one
// socket is what a resolver actually does, so the shape needed no redesign.
func dnsSession(domain string, region Region) [5]Descriptor {
	pool := DomainPool(ProfileDNS, region)
	set := [5]Descriptor{dnsQuery(domain)}
	for i := 1; i < 5; i++ {
		set[i] = dnsQuery(pool[rng.Intn(len(pool))])
	}
	return set
}

// dnsQuery renders one query with the transaction id as its only hole: a
// resolver redraws it per query and an observer cannot check it.
func dnsQuery(domain string) Descriptor {
	var b bytes.Buffer
	writeLen16(&b, 0x0100) // flags: RD
	// counts: 1 query, 0 answer, 0 authority, 1 additional (OPT)
	writeLen16(&b, 1)
	writeLen16(&b, 0)
	writeLen16(&b, 0)
	writeLen16(&b, 1)
	writeQName(&b, domain)
	// Weighted qtype: A 60%, AAAA 30%, MX 10%
	qtype := uint16(1)
	switch r := rng.Intn(100); {
	case r >= 90:
		qtype = 15
	case r >= 60:
		qtype = 28
	}
	writeLen16(&b, qtype)
	writeLen16(&b, 1) // IN
	// EDNS0 OPT RR: root name, type 0x0029, class = UDP size, TTL carries DO.
	b.WriteByte(0x00)
	writeLen16(&b, 0x0029)
	udpSize := 1232
	if rng.Intn(2) == 0 {
		udpSize = 4096
	}
	writeLen16(&b, uint16(udpSize))
	writeLen16(&b, 0x8000)
	writeLen16(&b, 0x0000)
	writeLen16(&b, 0) // rdlen
	return Descriptor{}.Rand(2).Lit(b.Bytes())
}
