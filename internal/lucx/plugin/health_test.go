// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeHealthHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	ok, detail := probeHealth(HealthCheck{Type: "http", Target: srv.URL, TimeoutSeconds: 2})
	if !ok {
		t.Fatalf("expected a pass, got %q", detail)
	}

	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closed.Close()
	ok, detail = probeHealth(HealthCheck{Type: "http", Target: closed.URL, TimeoutSeconds: 2})
	if ok {
		t.Fatal("expected a failure for a stopped server")
	}
	if detail == "" {
		t.Fatal("expected a failure detail")
	}
}

func TestProbeHealthTCP(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	ok, detail := probeHealth(HealthCheck{Type: "tcp", Target: l.Addr().String(), TimeoutSeconds: 2})
	if !ok {
		t.Fatalf("expected a pass, got %q", detail)
	}
	ok, detail = probeHealth(HealthCheck{Type: "tcp", Target: "127.0.0.1:1", TimeoutSeconds: 2})
	if ok {
		t.Fatal("expected a failure for a closed port")
	}
	if detail == "" {
		t.Fatal("expected a failure detail")
	}
}

func TestRunHealthRecordsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	p := &managed{}
	done := make(chan struct{})
	p.done = done
	go runHealth(p, done, HealthCheck{Type: "http", Target: srv.URL, IntervalSeconds: 5, TimeoutSeconds: 1})
	deadline := time.Now().Add(5 * time.Second)
	for {
		p.mu.Lock()
		status := p.healthStatus
		p.mu.Unlock()
		if status == "pass" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("health loop did not record a pass")
		}
		time.Sleep(50 * time.Millisecond)
	}
	close(done)
}
