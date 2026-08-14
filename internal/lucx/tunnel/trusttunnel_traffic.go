// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TrustTunnelScrapeTarget is one endpoint instance the job wants scraped:
// manager key, inbound tag for accounting, and the loopback metrics port.
type TrustTunnelScrapeTarget struct {
	Key         string
	Tag         string
	MetricsPort int
}

// TrustTunnelTraffic is one scrape snapshot: inbound-level byte delta plus
// the live client_sessions gauge. Metrics have no username labels, so
// per-client accounting is only possible when the inbound has a single
// enabled client (the job attributes the delta / online to that email).
type TrustTunnelTraffic struct {
	Tag      string
	Up       int64
	Down     int64
	Sessions int64
}

type trustTunnelCursor struct {
	up, down    int64
	initialized bool
}

// CollectTrustTunnelTraffic scrapes each running endpoint's Prometheus
// listener and folds the cumulative byte counters into deltas (high-water
// mark: a counter reset never overcounts).
func (m *Manager) CollectTrustTunnelTraffic(targets []TrustTunnelScrapeTarget) []TrustTunnelTraffic {
	var out []TrustTunnelTraffic

	m.mu.Lock()
	if m.trustTunnelTraffic == nil {
		m.trustTunnelTraffic = make(map[string]*trustTunnelCursor)
	}
	m.mu.Unlock()

	for _, t := range targets {
		if t.MetricsPort <= 0 || !m.IsRunningKey(t.Key) {
			continue
		}
		up, down, sessions, ok := scrapeTrustTunnelMetrics(t.MetricsPort)
		if !ok {
			continue
		}
		m.mu.Lock()
		cur, ok := m.trustTunnelTraffic[t.Key]
		if !ok {
			cur = &trustTunnelCursor{}
			m.trustTunnelTraffic[t.Key] = cur
		}
		d := TrustTunnelTraffic{Tag: t.Tag, Sessions: sessions}
		if cur.initialized {
			if up > cur.up {
				d.Up = up - cur.up
			}
			if down > cur.down {
				d.Down = down - cur.down
			}
		}
		cur.up, cur.down, cur.initialized = up, down, true
		m.mu.Unlock()
		out = append(out, d)
	}
	return out
}

// scrapeTrustTunnelMetrics fetches the endpoint's /metrics and sums the
// inbound/outbound byte counters across protocol_type labels.
func scrapeTrustTunnelMetrics(port int) (up, down, sessions int64, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, 0, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, 0, 0, false
	}
	return parseTrustTunnelMetrics(string(body))
}

// parseTrustTunnelMetrics extracts the aggregate counters from Prometheus
// text format:
//
//	inbound_traffic_bytes{protocol_type="http2"} 1234567
//	outbound_traffic_bytes{protocol_type="http1"} 7654321
func parseTrustTunnelMetrics(body string) (up, down, sessions int64, ok bool) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var name string
		var rest string
		if i := strings.IndexByte(line, '{'); i > 0 {
			name = line[:i]
			if j := strings.IndexByte(line[i:], '}'); j >= 0 {
				rest = strings.TrimSpace(line[i+j+1:])
			}
		} else {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			name, rest = fields[0], fields[1]
		}
		v, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			continue
		}
		switch name {
		case "inbound_traffic_bytes":
			up += int64(v)
			ok = true
		case "outbound_traffic_bytes":
			down += int64(v)
			ok = true
		case "client_sessions":
			sessions += int64(v)
			ok = true
		}
	}
	return up, down, sessions, ok
}

// TrustTunnelSoleClient returns the only non-empty email, or "" when the
// inbound has zero or several clients (aggregate metrics cannot be split).
func TrustTunnelSoleClient(emails []string) string {
	var sole string
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if sole != "" {
			return ""
		}
		sole = e
	}
	return sole
}
