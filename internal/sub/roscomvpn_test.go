// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestResolveRoutingRulesCustom(t *testing.T) {
	got := ResolveRoutingRules(RoscomVPNSourceCustom, "happ://custom")
	if got != "happ://custom" {
		t.Fatalf("custom = %q", got)
	}
	got = ResolveRoutingRules("unknown-source", "fallback")
	if got != "fallback" {
		t.Fatalf("unknown = %q", got)
	}
	got = ResolveRoutingRules("", "inline")
	if got != "inline" {
		t.Fatalf("empty source = %q", got)
	}
}

func TestResolveRoutingRulesFetchesAndCaches(t *testing.T) {
	// Isolate cache + client for this test.
	roscomvpnMu.Lock()
	roscomvpnCache = map[string]roscomvpnCacheEntry{}
	roscomvpnMu.Unlock()

	hits := 0
	var hitsMu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsMu.Lock()
		hits++
		hitsMu.Unlock()
		_, _ = w.Write([]byte("happ://routing/onadd/dGVzdA==\n"))
	}))
	defer srv.Close()

	prevURL := roscomvpnSourceURLs[RoscomVPNSourceDefault]
	prevClient := roscomvpnClient
	roscomvpnSourceURLs[RoscomVPNSourceDefault] = srv.URL
	roscomvpnClient = srv.Client()
	defer func() {
		roscomvpnSourceURLs[RoscomVPNSourceDefault] = prevURL
		roscomvpnClient = prevClient
		roscomvpnMu.Lock()
		roscomvpnCache = map[string]roscomvpnCacheEntry{}
		roscomvpnMu.Unlock()
	}()

	got := ResolveRoutingRules(RoscomVPNSourceDefault, "fallback")
	if got != "happ://routing/onadd/dGVzdA==" {
		t.Fatalf("first fetch = %q", got)
	}
	// Second call must hit cache (no extra HTTP).
	_ = ResolveRoutingRules(RoscomVPNSourceDefault, "fallback")
	hitsMu.Lock()
	n := hits
	hitsMu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 HTTP hit, got %d", n)
	}

	// Force TTL expiry and confirm refresh.
	roscomvpnMu.Lock()
	e := roscomvpnCache[RoscomVPNSourceDefault]
	e.fetchedAt = time.Now().Add(-roscomvpnCacheTTL - time.Second)
	roscomvpnCache[RoscomVPNSourceDefault] = e
	roscomvpnMu.Unlock()
	_ = ResolveRoutingRules(RoscomVPNSourceDefault, "fallback")
	hitsMu.Lock()
	n = hits
	hitsMu.Unlock()
	if n != 2 {
		t.Fatalf("expected 2 HTTP hits after TTL, got %d", n)
	}
}

func TestResolveRoutingRulesFallbackOnFailure(t *testing.T) {
	roscomvpnMu.Lock()
	roscomvpnCache = map[string]roscomvpnCacheEntry{}
	roscomvpnMu.Unlock()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()

	prevURL := roscomvpnSourceURLs[RoscomVPNSourceWhitelist]
	prevClient := roscomvpnClient
	roscomvpnSourceURLs[RoscomVPNSourceWhitelist] = srv.URL
	roscomvpnClient = srv.Client()
	defer func() {
		roscomvpnSourceURLs[RoscomVPNSourceWhitelist] = prevURL
		roscomvpnClient = prevClient
		roscomvpnMu.Lock()
		roscomvpnCache = map[string]roscomvpnCacheEntry{}
		roscomvpnMu.Unlock()
	}()

	got := ResolveRoutingRules(RoscomVPNSourceWhitelist, "my-custom")
	if got != "my-custom" {
		t.Fatalf("cold fail should fall back to custom, got %q", got)
	}
}
