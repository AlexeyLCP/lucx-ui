// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package cleanip probes random Cloudflare edge addresses for reachability
// and latency so an operator can pick a healthy client-side address. It is a
// read-only tool: it never rewrites configs (Rule 0) — the operator copies
// what it finds.
package cleanip

import (
	"crypto/tls"
	"errors"
	"math/rand/v2"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"
)

// Ranges are Cloudflare's published public IPv4 blocks.
var Ranges = []netip.Prefix{
	netip.MustParsePrefix("103.21.244.0/22"),
	netip.MustParsePrefix("103.22.200.0/22"),
	netip.MustParsePrefix("103.31.4.0/22"),
	netip.MustParsePrefix("104.16.0.0/13"),
	netip.MustParsePrefix("104.24.0.0/14"),
	netip.MustParsePrefix("108.162.192.0/18"),
	netip.MustParsePrefix("131.0.72.0/22"),
	netip.MustParsePrefix("141.101.64.0/18"),
	netip.MustParsePrefix("162.158.0.0/15"),
	netip.MustParsePrefix("172.64.0.0/13"),
	netip.MustParsePrefix("173.245.48.0/20"),
	netip.MustParsePrefix("188.114.96.0/20"),
	netip.MustParsePrefix("190.93.240.0/20"),
	netip.MustParsePrefix("197.234.240.0/22"),
	netip.MustParsePrefix("198.41.128.0/17"),
}

// scanWorkers caps concurrent probes of one scan batch.
const scanWorkers = 32

// Params describes one scan batch. Bounds are enforced by the web service.
type Params struct {
	Port      int    `json:"port"`
	Count     int    `json:"count"`
	Sample    int    `json:"sample"`
	TimeoutMs int    `json:"timeoutMs"`
	TlsCheck  bool   `json:"tlsCheck"`
	Sni       string `json:"sni"`
}

// Result is one healthy edge address.
type Result struct {
	Address   string `json:"address"`
	Port      int    `json:"port"`
	LatencyMs int64  `json:"latencyMs"`
	TlsOk     bool   `json:"tlsOk"`
}

// tcpDial and tlsHandshake are vars so tests fake the network.
var tcpDial = func(ap netip.AddrPort, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", ap.String(), timeout)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start), nil
}

var tlsHandshake = func(ap netip.AddrPort, sni string, timeout time.Duration) error {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", ap.String(), &tls.Config{ServerName: sni})
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// sampleAddrs draws n random AddrPorts from the published ranges. Var, so
// tests feed a fixed batch of addresses.
var sampleAddrs = func(n, port int) []netip.AddrPort {
	seen := map[netip.Addr]struct{}{}
	addrs := make([]netip.AddrPort, 0, n)
	guard := 0
	for len(addrs) < n && guard < n*10 {
		guard++
		p := Ranges[rand.IntN(len(Ranges))]
		hostBits := p.Addr().BitLen() - p.Bits()
		off := rand.Uint64() & ((1 << hostBits) - 1)
		addr := netip.AddrFrom4(uint32Add(p.Addr().As4(), uint32(off)))
		if _, dup := seen[addr]; dup {
			continue
		}
		seen[addr] = struct{}{}
		addrs = append(addrs, netip.AddrPortFrom(addr, uint16(port)))
	}
	return addrs
}

func uint32Add(b [4]byte, off uint32) [4]byte {
	v := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) + off
	return [4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// Scan probes Sample addresses and returns at most Count healthy ones,
// TLS-verified first, then by latency. TlsCheck drops addresses whose TLS
// handshake against Sni fails — they are useless as edges.
func Scan(p Params) ([]Result, error) {
	if p.Port <= 0 || p.Port > 65535 {
		return nil, errors.New("invalid port")
	}
	if p.Count <= 0 || p.Sample <= 0 {
		return nil, errors.New("count and sample must be positive")
	}
	timeout := time.Duration(p.TimeoutMs) * time.Millisecond
	if p.TlsCheck && p.Sni == "" {
		return nil, errors.New("tlsCheck requires an SNI")
	}

	addrs := sampleAddrs(p.Sample, p.Port)
	jobs := make(chan netip.AddrPort)
	results := make([]Result, 0, len(addrs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := scanWorkers
	if workers > len(addrs) {
		workers = len(addrs)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ap := range jobs {
				latency, err := tcpDial(ap, timeout)
				if err != nil {
					continue
				}
				r := Result{Address: ap.Addr().String(), Port: int(ap.Port()), LatencyMs: latency.Milliseconds()}
				if p.TlsCheck {
					if err := tlsHandshake(ap, p.Sni, timeout); err != nil {
						continue
					}
					r.TlsOk = true
				}
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			}
		}()
	}
	for _, ap := range addrs {
		jobs <- ap
	}
	close(jobs)
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].TlsOk != results[j].TlsOk {
			return results[i].TlsOk
		}
		return results[i].LatencyMs < results[j].LatencyMs
	})
	if len(results) > p.Count {
		results = results[:p.Count]
	}
	return results, nil
}
