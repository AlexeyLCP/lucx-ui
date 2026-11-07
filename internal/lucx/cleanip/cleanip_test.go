// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cleanip

import (
	"errors"
	"net/netip"
	"testing"
	"time"
)

func TestSampleAddrs(t *testing.T) {
	orig := sampleAddrs
	t.Cleanup(func() { sampleAddrs = orig })
	addrs := sampleAddrs(60, 443)
	if len(addrs) != 60 {
		t.Fatalf("sampleAddrs(60) = %d addresses", len(addrs))
	}
	for _, ap := range addrs {
		if ap.Port() != 443 {
			t.Fatalf("wrong port in %v", ap)
		}
		inRange := false
		for _, r := range Ranges {
			if r.Contains(ap.Addr()) {
				inRange = true
				break
			}
		}
		if !inRange {
			t.Fatalf("%v is outside every Cloudflare range", ap)
		}
	}
}

func newAddrPort(s string) netip.AddrPort {
	return netip.AddrPortFrom(netip.MustParseAddr(s), 443)
}

func fakeDialers(lats map[netip.Addr]time.Duration, tlsFails map[netip.Addr]bool) {
	tcpDial = func(ap netip.AddrPort, _ time.Duration) (time.Duration, error) {
		lat, ok := lats[ap.Addr()]
		if !ok {
			return 0, errors.New("unreachable")
		}
		return lat, nil
	}
	tlsHandshake = func(ap netip.AddrPort, _ string, _ time.Duration) error {
		if tlsFails[ap.Addr()] {
			return errors.New("tls handshake failed")
		}
		return nil
	}
}

func restoreDialers(origTcp func(netip.AddrPort, time.Duration) (time.Duration, error), origTls func(netip.AddrPort, string, time.Duration) error) {
	tcpDial = origTcp
	tlsHandshake = origTls
}

func TestScanRanksAndFilters(t *testing.T) {
	origTcp, origTls, origSample := tcpDial, tlsHandshake, sampleAddrs
	defer restoreDialers(origTcp, origTls)
	t.Cleanup(func() { sampleAddrs = origSample })

	port := []netip.AddrPort{
		newAddrPort("104.16.0.1"), newAddrPort("104.16.0.2"),
		newAddrPort("104.16.0.3"), newAddrPort("8.8.8.8"), newAddrPort("8.8.4.4"),
	}
	sampleAddrs = func(int, int) []netip.AddrPort { return append([]netip.AddrPort(nil), port...) }

	ip := func(s string) netip.Addr { return netip.MustParseAddr(s) }
	fakeDialers(
		map[netip.Addr]time.Duration{
			ip("8.8.8.8"):    40 * time.Millisecond,
			ip("8.8.4.4"):    40 * time.Millisecond,
			ip("104.16.0.1"): 120 * time.Millisecond,
			ip("104.16.0.2"): 10 * time.Millisecond,
			ip("104.16.0.3"): 200 * time.Millisecond,
		},
		map[netip.Addr]bool{
			ip("8.8.4.4"): true, ip("8.8.8.8"): true, ip("104.16.0.3"): true,
		},
	)

	res, err := Scan(Params{Port: 443, Count: 10, Sample: 5, TimeoutMs: 1000, TlsCheck: true, Sni: "www.cloudflare.com"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("TLS-failed and unreachable addresses must be dropped, got %v", res)
	}
	if res[0].Address != "104.16.0.2" || res[0].LatencyMs != 10 {
		t.Fatalf("fastest first: %v", res)
	}
	if res[1].Address != "104.16.0.1" || !res[1].TlsOk {
		t.Fatalf("second result: %v", res)
	}
}

func TestScanCountBounds(t *testing.T) {
	origTcp, origTls, origSample := tcpDial, tlsHandshake, sampleAddrs
	defer restoreDialers(origTcp, origTls)
	t.Cleanup(func() { sampleAddrs = origSample })
	sampleAddrs = func(int, int) []netip.AddrPort {
		return []netip.AddrPort{newAddrPort("104.16.0.1")}
	}
	fakeDialers(map[netip.Addr]time.Duration{
		netip.MustParseAddr("104.16.0.1"): 1 * time.Millisecond,
	}, nil)

	if _, err := Scan(Params{Port: 0, Count: 1, Sample: 1, TimeoutMs: 100}); err == nil {
		t.Fatal("port 0 must fail")
	}
	if _, err := Scan(Params{Port: 443, Count: 0, Sample: 1, TimeoutMs: 100}); err == nil {
		t.Fatal("count 0 must fail")
	}
	if _, err := Scan(Params{Port: 443, Count: 1, Sample: 0, TimeoutMs: 100}); err == nil {
		t.Fatal("sample 0 must fail")
	}
	if _, err := Scan(Params{Port: 443, Count: 1, Sample: 1, TimeoutMs: 100, TlsCheck: true, Sni: ""}); err == nil {
		t.Fatal("tlsCheck without SNI must fail")
	}

	res, err := Scan(Params{Port: 443, Count: 5, Sample: 1, TimeoutMs: 100, TlsCheck: false})
	if err != nil || len(res) != 1 {
		t.Fatalf("plain scan: %v %v", res, err)
	}
	if res[0].TlsOk {
		t.Fatalf("no TLS check — TlsOk must stay false: %v", res)
	}
}
