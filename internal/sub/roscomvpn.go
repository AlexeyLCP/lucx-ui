// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RoscomVPN routing sources served from hydraponique/roscomvpn-routing (HAPP
// profile). Each .DEEPLINK file contains a single-line happ://routing/onadd/<base64>
// value placed into the Happ `Routing` response header. Ported from
// hydraponique/3x-ui (sub/roscomvpn.go) as an additive LucX subscription option.
const (
	RoscomVPNSourceDefault   = "default"
	RoscomVPNSourceJsonSub   = "jsonsub"
	RoscomVPNSourceWhitelist = "whitelist"
	RoscomVPNSourceCustom    = "custom"

	roscomvpnCacheTTL      = 10 * time.Minute
	roscomvpnNegativeCache = 30 * time.Second
	roscomvpnHTTPTimeout   = 4 * time.Second
	roscomvpnMaxBodyBytes  = 1 << 20
)

var roscomvpnSourceURLs = map[string]string{
	RoscomVPNSourceDefault:   "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/DEFAULT.DEEPLINK",
	RoscomVPNSourceJsonSub:   "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/JSONSUB.DEEPLINK",
	RoscomVPNSourceWhitelist: "https://raw.githubusercontent.com/hydraponique/roscomvpn-routing/main/HAPP/WHITELIST.DEEPLINK",
}

type roscomvpnCacheEntry struct {
	value     string
	fetchedAt time.Time
	lastFail  time.Time
}

var (
	roscomvpnMu     sync.RWMutex
	roscomvpnCache  = map[string]roscomvpnCacheEntry{}
	roscomvpnClient = &http.Client{Timeout: roscomvpnHTTPTimeout}

	roscomvpnFetchLocks sync.Map
)

func roscomvpnLockFor(src string) *sync.Mutex {
	if m, ok := roscomvpnFetchLocks.Load(src); ok {
		return m.(*sync.Mutex)
	}
	m, _ := roscomvpnFetchLocks.LoadOrStore(src, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// ResolveRoutingRules returns the value for the "Routing" response header.
// Known RoscomVPN sources are fetched from GitHub with a TTL cache; "custom"
// (or unknown) returns the supplied custom string as-is. On fetch failure the
// last known good value is served, falling back to custom when the cache is cold.
func ResolveRoutingRules(source, custom string) string {
	src := strings.ToLower(strings.TrimSpace(source))
	if src == "" {
		src = RoscomVPNSourceDefault
	}
	if src == RoscomVPNSourceCustom {
		return custom
	}
	url, ok := roscomvpnSourceURLs[src]
	if !ok {
		return custom
	}

	roscomvpnMu.RLock()
	entry, hit := roscomvpnCache[src]
	roscomvpnMu.RUnlock()
	if hit && time.Since(entry.fetchedAt) < roscomvpnCacheTTL {
		return entry.value
	}
	if hit && !entry.lastFail.IsZero() && time.Since(entry.lastFail) < roscomvpnNegativeCache {
		if entry.value != "" {
			return entry.value
		}
		return custom
	}

	mu := roscomvpnLockFor(src)
	mu.Lock()
	defer mu.Unlock()

	roscomvpnMu.RLock()
	entry, hit = roscomvpnCache[src]
	roscomvpnMu.RUnlock()
	if hit && time.Since(entry.fetchedAt) < roscomvpnCacheTTL {
		return entry.value
	}

	if v, err := fetchRoscomVPNDeepLink(url); err == nil {
		roscomvpnMu.Lock()
		roscomvpnCache[src] = roscomvpnCacheEntry{value: v, fetchedAt: time.Now()}
		roscomvpnMu.Unlock()
		return v
	}

	roscomvpnMu.Lock()
	prev := roscomvpnCache[src]
	prev.lastFail = time.Now()
	roscomvpnCache[src] = prev
	roscomvpnMu.Unlock()

	if hit && entry.value != "" {
		return entry.value
	}
	return custom
}

func fetchRoscomVPNDeepLink(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), roscomvpnHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := roscomvpnClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("roscomvpn deeplink fetch failed: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, roscomvpnMaxBodyBytes))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}
