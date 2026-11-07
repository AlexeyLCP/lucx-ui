// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import "github.com/mhsanaei/3x-ui/v3/internal/lucx/cleanip"

// CleanIpService runs Cloudflare edge reachability scans on operator demand.
// Stateless: nothing is persisted, nothing is auto-applied (Rule 0).
type CleanIpService struct{}

// Scan clamps the request onto safe bounds and probes the ranges.
// Defaults: port 443, top 30 of a 256-address sample, 1.5 s per probe.
func (s *CleanIpService) Scan(p cleanip.Params) ([]cleanip.Result, error) {
	if p.Port == 0 {
		p.Port = 443
	}
	if p.Count == 0 {
		p.Count = 30
	}
	if p.Count > 100 {
		p.Count = 100
	}
	if p.Sample == 0 {
		p.Sample = 256
	}
	if p.Sample > 4096 {
		p.Sample = 4096
	}
	if p.TimeoutMs == 0 {
		p.TimeoutMs = 1500
	}
	if p.TimeoutMs > 10000 {
		p.TimeoutMs = 10000
	}
	if p.TlsCheck && p.Sni == "" {
		p.Sni = "www.cloudflare.com"
	}
	return cleanip.Scan(p)
}
