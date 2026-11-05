// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"encoding/json"
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
// the live client_sessions gauge. With per_client_metrics the endpoint's
// /clients adds each user's delta, sessions and IP (PerUser); without it the
// job can only credit an inbound with a single enabled client.
type TrustTunnelTraffic struct {
	Tag      string
	Up       int64
	Down     int64
	Sessions int64
	PerUser  bool
	Users    []TrustTunnelUserTraffic
}

// TrustTunnelUserTraffic is one /clients row: the credentials username, its
// byte delta since the last scrape, live sessions and the IP it was seen on.
type TrustTunnelUserTraffic struct {
	User     string
	Up       int64
	Down     int64
	Sessions int64
	IP       string
}

type trustTunnelCursor struct {
	up, down    int64
	initialized bool
	users       map[string][2]int64
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
		if rows, ok := scrapeTrustTunnelClients(t.MetricsPort); ok {
			d.PerUser = true
			d.Users = trustTunnelUserDeltas(cur, rows)
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
// text format. Despite the HELP text, the endpoint counts inbound_traffic_bytes
// on SimplexDirection::Incoming ("peer to client", lib/src/pipe.rs), i.e. what
// the client downloads, and outbound_traffic_bytes on what it uploads:
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
		case "outbound_traffic_bytes":
			up += int64(v)
			ok = true
		case "inbound_traffic_bytes":
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

// trustTunnelClientRow is one entry of the endpoint's /clients JSON.
type trustTunnelClientRow struct {
	Username string  `json:"username"`
	IP       *string `json:"ip"`
	Sessions int64   `json:"sessions"`
	Inbound  int64   `json:"inbound"`
	Outbound int64   `json:"outbound"`
}

// scrapeTrustTunnelClients fetches /clients; ok=false when the endpoint runs
// without per_client_metrics (404) or isn't reachable.
func scrapeTrustTunnelClients(port int) ([]trustTunnelClientRow, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/clients", port), nil)
	if err != nil {
		return nil, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var rows []trustTunnelClientRow
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rows) != nil {
		return nil, false
	}
	return rows, true
}

// trustTunnelUserDeltas folds lifetime per-user counters into deltas, with
// the same high-water rule as the aggregate: the first scrape and a counter
// that went down (endpoint restart) only set the base. Caller holds m.mu.
func trustTunnelUserDeltas(cur *trustTunnelCursor, rows []trustTunnelClientRow) []TrustTunnelUserTraffic {
	prev, seen := cur.users, cur.users != nil
	next := make(map[string][2]int64, len(rows))
	out := make([]TrustTunnelUserTraffic, 0, len(rows))
	for _, r := range rows {
		name := strings.TrimSpace(r.Username)
		if name == "" {
			continue
		}
		next[name] = [2]int64{r.Inbound, r.Outbound}
		u := TrustTunnelUserTraffic{User: name, Sessions: r.Sessions}
		if r.IP != nil {
			u.IP = strings.TrimSpace(*r.IP)
		}
		// inbound = to the client (download), see parseTrustTunnelMetrics.
		if p, ok := prev[name]; seen && ok {
			if r.Outbound > p[1] {
				u.Up = r.Outbound - p[1]
			}
			if r.Inbound > p[0] {
				u.Down = r.Inbound - p[0]
			}
		} else if seen {
			// A user new since the last scrape counted from zero.
			u.Up, u.Down = r.Outbound, r.Inbound
		}
		out = append(out, u)
	}
	cur.users = next
	return out
}
