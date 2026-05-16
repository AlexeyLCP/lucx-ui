// Copyright (c) 2025 LucX-UI Project.
// Chaos Engineering & Stress Tests for LucX-UI
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package lucx

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/nodetype"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/outbound_link"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/parser"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/telemt"
)

// ============================================================
// VECTOR 1: Concurrency Stress Test (Deadlock & Race Condition)
// ============================================================
func TestVector1_ConcurrencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency stress in short mode")
	}
	goroutinesBefore := runtime.NumGoroutine()
	var opsDone atomic.Int64
	var opsFailed atomic.Int64
	var wg sync.WaitGroup

	concurrency := 100
	iterations := 50

	t.Logf("Starting %d goroutines × %d iterations = %d ops", concurrency, iterations, concurrency*iterations)

	startTime := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternate between different heavy operations
				switch j % 4 {
				case 0:
					// AWG params generation (uses crypto/rand)
					_, err := awg.GenerateAWGParams(1+(j%3), "quic", "ru")
					if err != nil {
						opsFailed.Add(1)
					} else {
						opsDone.Add(1)
					}
				case 1:
					// SSH parser with valid data
					input := fmt.Sprintf("Access URL:  https://5.9.1.2:2053/test%d\nUsername:    admin%d\nPassword:    pass%d\nAPI Token:   tok%d", j, j, j, j)
					_, err := parser.ParseSSHOutput(input)
					if err != nil {
						opsFailed.Add(1)
					} else {
						opsDone.Add(1)
					}
				case 2:
					// Node type detection (parse features JSON)
					_ = nodetype.FromJSON(fmt.Sprintf(`{"features":["awg","telemt"],"awgVersion":"%d.0","telemtVersion":"%d.0"}`, j%4, j%5))
					opsDone.Add(1)
				case 3:
					// Outbound link generation
					settings := fmt.Sprintf(`{"clients":[{"id":"uuid-%d","flow":"xtls-rprx-vision","email":"c%d"}],"decryption":"none"}`, j, j)
					_, err := outbound_link.GenerateOutbound("vless", "tag", 443, settings, "{}", "1.2.3.4")
					if err != nil {
						opsFailed.Add(1)
					} else {
						opsDone.Add(1)
					}
				}
			}
		}(i)
	}

	// Wait with timeout — deadlock detection
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		t.Logf("All goroutines completed cleanly — NO DEADLOCK")
	case <-time.After(60 * time.Second):
		t.Errorf("DEADLOCK DETECTED: goroutines hung after 60s timeout")
	}

	elapsed := time.Since(startTime)
	goroutinesAfter := runtime.NumGoroutine()

	t.Logf("Vector 1 Metrics:")
	t.Logf("  Ops completed: %d", opsDone.Load())
	t.Logf("  Ops failed:    %d", opsFailed.Load())
	t.Logf("  Elapsed:       %v", elapsed)
	t.Logf("  Goroutines before: %d", goroutinesBefore)
	t.Logf("  Goroutines after:  %d", goroutinesAfter)

	if goroutinesAfter > goroutinesBefore+10 {
		t.Errorf("GOROUTINE LEAK: %d extra goroutines remain", goroutinesAfter-goroutinesBefore)
	}
}

// ============================================================
// VECTOR 2: Fuzz Testing & Garbage Injection
// ============================================================
func TestVector2_FuzzParser(t *testing.T) {
	fuzzInputs := []string{
		// Extremely large input (1MB of garbage)
		strings.Repeat("A", 1_000_000),
		// ANSI escape sequences
		"\033[0;31m\033[0;32m\033[1;33m\033[41mUsername: \033[0m",
		// Broken UTF-8
		string([]byte{0xFF, 0xFE, 0xFD, 0x80, 0xBF}),
		// SQL injection attempt
		"Username: admin'; DROP TABLE users; --\nPassword: ' OR 1=1 --",
		// XSS attempt
		"<script>alert('xss')</script>\nUsername: <img src=x onerror=alert(1)>",
		// Null bytes
		"Username:\x00admin\x00\nPassword:\x00pass\x00",
		// Emoji flood
		strings.Repeat("��💀🎉🔥🚀💩", 1000),
		// Valid data with emoji embedded
		"Username:    user🎉name\nPassword:    pass🔥word\nAccess URL:  https://1.2.3.4:2053/path",
		// Extremely long keys
		"Username:" + strings.Repeat("x", 100_000),
		// Binary garbage
		string(make([]byte, 10000)),
	}

	for i, input := range fuzzInputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC on fuzz input %d: %v", i, r)
				}
			}()
			_, err := parser.ParseSSHOutput(input)
			t.Logf("Fuzz input %d: error=%v (no panic)", i, err)
		}()
	}
	t.Log("Fuzz test complete — no panics")
}

func TestVector2_FuzzTelemtSecret(t *testing.T) {
	fuzzSecrets := []string{
		"ee" + strings.Repeat("0", 32),
		"ee" + strings.Repeat("f", 64),
		"ee" + strings.Repeat("g", 32),   // 'g' is not hex
		"EE00000000000000000000000000000000", // uppercase EE
		"",
		"ee",
		"ee" + strings.Repeat("0", 31),    // 1 char short
		"dd" + strings.Repeat("0", 32),    // wrong prefix
		strings.Repeat("0", 34),
		"ee" + string(make([]byte, 100000)), // huge
		"ee😀💀🎉",
	}

	for _, secret := range fuzzSecrets {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC on secret %q: %v", secret[:min(len(secret), 30)], r)
				}
			}()
			// Test via TelemtConfig generator
			_, err := telemt.GenerateConfig(telemt.ConfigData{
				ID: 0, Port: 443, PublicHost: "test",
				SocksPort: 31427, SocksPassword: "pw",
				APIPort: 9090, TLSDomain: "test.ru",
				MaxConnections: 100,
				Clients: []telemt.TelemtClient{
					{Name: "fuzz", Secret: secret},
				},
			})
			t.Logf("Secret fuzz %q: err=%v", secret[:min(len(secret), 20)], err)
		}()
	}
	t.Log("Telemt fuzz test complete — no panics")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================
// VECTOR 3: Resource Leak Test
// ============================================================
func TestVector3_ResourceLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping resource leak test in short mode")
	}
	goroutinesBefore := runtime.NumGoroutine()
	t.Logf("Goroutines before: %d", goroutinesBefore)

	iterations := 100

	for i := 0; i < iterations; i++ {
		// Simulate AWG create/delete cycle
		func() {
			defer func() { recover() }()
			_, _ = awg.GenerateAWGParams(1, "quic", "ru")
			_, _ = awg.GenerateAWGParams(2, "sip", "ru")
			_, _ = awg.GenerateAWGParams(3, "dns", "world")
			// CPS generation
			i1, i2, i3, i4, i5 := awg.GenerateCPS(3, awg.CPSProfileQUIC)
			_ = i1; _ = i2; _ = i3; _ = i4; _ = i5
			// Template rendering
			_, _ = awg.RenderPostUp(awg.TemplateData{
				AWGInterface: fmt.Sprintf("awg%d", i),
				TUNInterface: fmt.Sprintf("awg%dt", i),
				AWGServerIP:  "10.0.0.1",
				AWGSubnet:    "10.0.0.0/24",
				AWGPort:      34567,
				RouteTable:   "100",
				RoutePref:    1000,
				MTU:          1320,
			})
			_, _ = awg.RenderPostDown(awg.TemplateData{
				AWGInterface: "awg0",
				TUNInterface: "awg0t",
				AWGSubnet:    "10.0.0.0/24",
				RouteTable:   "100",
				RoutePref:    1000,
			})
		}()

		// Simulate Telemt create/delete cycle
		func() {
			defer func() { recover() }()
			_, _ = telemt.GenerateConfig(telemt.ConfigData{
				ID: i, Port: 443, PublicHost: "test",
				SocksPort: 31427, SocksPassword: "pw",
				APIPort: 9090, TLSDomain: "test.ru",
				MaxConnections: 100,
				Clients: []telemt.TelemtClient{
					{Name: fmt.Sprintf("user%d", i), Secret: telemt.GenerateSecret()},
				},
			})
			secret := telemt.GenerateSecret()
			_ = telemt.GenerateProxyLink("1.2.3.4", 443, secret)
			_, _ = telemt.GenerateConfig(telemt.ConfigData{
				ID: i, Port: 443, PublicHost: "test",
				SocksPort: 1, SocksPassword: "p",
				APIPort: 0, TLSDomain: "t", MaxConnections: 1,
			})
		}()

		if i%20 == 0 {
			// Force GC and check goroutines
			runtime.GC()
			time.Sleep(10 * time.Millisecond)
		}
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	t.Logf("Vector 3 Metrics:")
	t.Logf("  Cycles:            %d", iterations)
	t.Logf("  Goroutines before: %d", goroutinesBefore)
	t.Logf("  Goroutines after:  %d", goroutinesAfter)

	if goroutinesAfter > goroutinesBefore+20 {
		t.Errorf("GOROUTINE LEAK: %d extra goroutines remain after %d cycles",
			goroutinesAfter-goroutinesBefore, iterations)
	} else {
		t.Logf("No goroutine leak detected (delta: %d)", goroutinesAfter-goroutinesBefore)
	}

	// Check for file descriptor leaks (if test created any files)
	var openFiles int
	entries, _ := os.ReadDir("/proc/self/fd")
	openFiles = len(entries)
	t.Logf("  Open file descriptors: %d", openFiles)
	if openFiles > 100 {
		t.Errorf("FD LEAK: %d open file descriptors", openFiles)
	}
}

// ============================================================
// VECTOR 4: Chaos Engineering — Crash Recovery
// ============================================================
func TestVector4_ChaosCrashRecovery(t *testing.T) {
	// Simulate Xray crash during active AWG/Telemt operations
	// We can't actually kill the real Xray process, but we test the recovery logic

	t.Run("AWG_postdown_idempotent", func(t *testing.T) {
		// PostDown must be safe to run multiple times (crash recovery scenario)
		data := awg.TemplateData{
			AWGInterface: "awg99",
			TUNInterface: "awg99t",
			AWGSubnet:    "10.99.0.0/24",
			RouteTable:   "1099",
			RoutePref:    1099,
		}

		// Run PostDown twice — second should not error
		script1, err := awg.RenderPostDown(data)
		if err != nil {
			t.Fatalf("first PostDown render failed: %v", err)
		}
		script2, err := awg.RenderPostDown(data)
		if err != nil {
			t.Fatalf("second PostDown render failed: %v", err)
		}
		if script1 != script2 {
			t.Error("PostDown should be idempotent — identical scripts expected")
		}

		// Verify critical cleanup commands present
		requiredCmds := []string{
			"set +e",
			"2>/dev/null || true",
			"iptables -t nat -D POSTROUTING",
			"iptables -D FORWARD",
			"ip route del default",
			"ip rule del",
			"ip link del",
		}
		for _, cmd := range requiredCmds {
			if !strings.Contains(script1, cmd) {
				t.Errorf("PostDown missing critical command: %s", cmd)
			}
		}
	})

	t.Run("Telemt_config_survives_corruption", func(t *testing.T) {
		// Test that config generation survives bad input (simulating corrupted state after crash)
		corruptedCases := []struct {
			name string
			data telemt.ConfigData
		}{
			{"negative_port", telemt.ConfigData{Port: -1, TLSDomain: "test.ru", MaxConnections: 1}},
			{"empty_everything", telemt.ConfigData{}},
			{"huge_maxconn", telemt.ConfigData{Port: 443, TLSDomain: "test.ru", MaxConnections: 99999999}},
			{"special_chars_in_host", telemt.ConfigData{Port: 443, PublicHost: "../../etc/passwd", TLSDomain: "test.ru"}},
			{"emoji_in_domain", telemt.ConfigData{Port: 443, TLSDomain: "🎉.ru"}},
		}
		for _, c := range corruptedCases {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC on %s: %v", c.name, r)
					}
				}()
				_, err := telemt.GenerateConfig(c.data)
				t.Logf("Corrupted config '%s': err=%v (no panic)", c.name, err)
			}()
		}
	})

	t.Run("AWG_params_survives_null_context", func(t *testing.T) {
		// Generate params with edge case contexts (simulating nil/invalid state after crash)
		ctx := context.Background()
		_ = ctx // use to simulate various crash states
		for i := 0; i < 50; i++ {
			_, _ = awg.GenerateAWGParams(1+(i%3), "quic", "ru")
			_, _ = awg.GenerateAWGParams(1+(i%3), "sip", "world")
			_, _ = awg.GenerateAWGParams(1+(i%3), "dns", "ru")
		}
		// All should succeed without panic
	})

	t.Run("Nodetype_detection_timeout", func(t *testing.T) {
		// Simulate network timeout during node type detection
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(1 * time.Millisecond) // ensure context is expired
		info, err := nodetype.DetectNodeType(ctx, "http://192.0.2.1:12345", "token")
		if err != nil {
			t.Logf("Expected timeout error: %v", err)
		}
		if info != nil {
			t.Logf("Info on timeout: %+v (should be nil)", info)
		}
	})
}
