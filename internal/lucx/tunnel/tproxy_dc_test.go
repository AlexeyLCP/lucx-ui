// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func resetTgReach(t *testing.T) {
	t.Helper()
	orig := dcProbeDial
	t.Cleanup(func() { dcProbeDial = orig })
	tgReach.mu.Lock()
	tgReach.until = time.Time{}
	tgReach.ok = false
	tgReach.mu.Unlock()
}

func writeTgConf(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "proxy-multi.conf")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTgProxyAddresses(t *testing.T) {
	confPath := writeTgConf(t, strings.Join([]string{
		"S = 3c6ff8f5e8...",
		"proxy_for_medium#1 = 149.154.167.51:8888;",
		"proxy_for_medium#2 = 91.105.192.110:8888;",
		"proxy_for_medium#3 = 149.154.167.51:8888;",
		"proxy_dc_4 = 149.154.175.50:443 # comment",
		"junk line 3001:db8::1:443 only ipv6 ignored",
	}, "\n"))

	got := tgProxyAddresses(confPath, 10)
	if strings.Join(got, "|") != "149.154.167.51:8888|91.105.192.110:8888|149.154.175.50:443" {
		t.Fatalf("extraction/dedupe wrong: %v", got)
	}

	if got := tgProxyAddresses(confPath, 2); len(got) != 2 {
		t.Fatalf("limit=2 must spread to 2 addresses, got %v", got)
	}
}

func TestTgProxiesReachable(t *testing.T) {
	resetTgReach(t)

	confPath := writeTgConf(t, "proxy_for_medium#1 = 149.154.167.51:8888;\nproxy_dc_1 = 91.105.192.110:443;\n")

	t.Run("stops at first reachable", func(t *testing.T) {
		var dials int
		dcProbeDial = func(addr string, timeout time.Duration) error {
			dials++
			if addr == "91.105.192.110:443" {
				return nil
			}
			return errors.New("refused")
		}
		if !tgProxiesReachable(confPath) {
			t.Fatal("a reachable entry must open the gate")
		}
		if dials < 2 {
			t.Fatalf("probe must skip the failed entry and continue, dials=%d", dials)
		}
	})

	t.Run("cache suppresses re-dial until ttl", func(t *testing.T) {
		var dials int
		dcProbeDial = func(string, time.Duration) error { dials++; return errors.New("refused") }
		tgReach.mu.Lock()
		tgReach.until = time.Time{}
		tgReach.mu.Unlock()

		if tgProxiesReachable(confPath) {
			t.Fatal("all-probes-failed must close the gate")
		}
		first := dials
		if tgProxiesReachable(confPath) {
			t.Fatal("cached failed verdict must persist within ttl")
		}
		if dials != first {
			t.Fatalf("cache must suppress re-dial: %d then %d", first, dials)
		}
		tgReach.mu.Lock()
		tgReach.until = time.Now().Add(-time.Second)
		tgReach.mu.Unlock()
		if tgProxiesReachable(confPath) {
			t.Fatal("expired cache must re-probe (still failed)")
		}
		if dials <= first {
			t.Fatalf("expired cache must re-dial: %d then %d", first, dials)
		}
	})

	t.Run("conf without addresses never gates", func(t *testing.T) {
		var dials int
		dcProbeDial = func(string, time.Duration) error { dials++; return errors.New("refused") }
		tgReach.mu.Lock()
		tgReach.until = time.Time{}
		tgReach.mu.Unlock()

		emptyPath := writeTgConf(t, "S = 3c6ff8f5e8\n")
		if !tgProxiesReachable(emptyPath) {
			t.Fatal("no parseable addresses must not gate the start")
		}
		if dials != 0 {
			t.Fatalf("no addresses — no dialing, got %d", dials)
		}
	})
}
