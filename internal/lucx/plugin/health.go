// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// runHealth probes a running plugin on a ticker and records pass/fail under
// the managed lock. It exits when the process's done channel closes, so
// Stop/exit tears it down without extra plumbing.
func runHealth(p *managed, done chan struct{}, h HealthCheck) {
	interval := time.Duration(h.IntervalSeconds) * time.Second
	tick := time.NewTicker(interval)
	defer tick.Stop()
	run := func() {
		ok, detail := probeHealth(h)
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.done != done {
			return
		}
		if ok {
			p.healthStatus = "pass"
			p.healthDetail = ""
		} else {
			p.healthStatus = "fail"
			p.healthDetail = detail
		}
		p.healthCheckedAt = time.Now().Unix()
	}
	run()
	for {
		select {
		case <-done:
			return
		case <-tick.C:
			run()
		}
	}
}

func probeHealth(h HealthCheck) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.TimeoutSeconds)*time.Second)
	defer cancel()
	if h.Type == "http" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.Target, nil)
		if err != nil {
			return false, err.Error()
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return true, ""
		}
		return false, fmt.Sprintf("http status %d", resp.StatusCode)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", h.Target)
	if err != nil {
		return false, err.Error()
	}
	conn.Close()
	return true, ""
}
