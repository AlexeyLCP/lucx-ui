// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAivpnDefaultSubnetStable(t *testing.T) {
	for id := 1; id <= 260; id++ {
		subnet := AivpnDefaultSubnet(id)
		again := AivpnDefaultSubnet(id)
		if subnet != again {
			t.Fatalf("subnet for id %d is not stable", id)
		}
		// Must remain a valid IPv4 /24 away from the other panel TUN ranges.
		for _, reserved := range []string{"10.254.", "10.68.", "10.66."} {
			if strings.Contains(subnet, reserved) {
				t.Fatalf("id %d collides with the reserved range %s: %s", id, reserved, subnet)
			}
		}
		check := AivpnConfig{Subnet: subnet}
		if _, _, err := check.serverVPNSuffix(); err != nil {
			t.Fatalf("derived subnet invalid: %v", err)
		}
	}
}

func TestAivpnRenderConfig(t *testing.T) {
	dir := t.TempDir()
	prev := tunnelDir
	tunnelDir = func() string { return dir }
	defer func() { tunnelDir = prev }()

	cfg := AivpnConfig{
		ListenHost: "0.0.0.0",
		Subnet:     "10.21.0.1/24",
		ServerIP:   "203.0.113.7",
	}
	raw := cfg.RenderConfig(AivpnKey, 45001, "10.21.0.1/24")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if parsed["listen_addr"] != "0.0.0.0:45001" {
		t.Fatalf("listen_addr = %v", parsed["listen_addr"])
	}
	if parsed["server_ip"] != "203.0.113.7" {
		t.Fatalf("server_ip = %v", parsed["server_ip"])
	}
	network, ok := parsed["network_config"].(map[string]any)
	if !ok {
		t.Fatalf("network_config missing")
	}
	if network["server_vpn_ip"] != "10.21.0.1" {
		t.Fatalf("server_vpn_ip = %v", network["server_vpn_ip"])
	}
	if network["prefix_len"] != float64(24) {
		t.Fatalf("prefix_len = %v", network["prefix_len"])
	}
	paths := []string{"key_file", "clients_db", "audit_log", "management_socket", "mask_dir"}
	for _, p := range paths {
		if s, ok := parsed[p].(string); !ok || s == "" {
			t.Fatalf("%s missing/empty: %v", p, parsed[p])
		}
	}
	if parsed["mask_dir"] != filepath.Join(dir, "aivpn-masks") {
		t.Fatalf("mask_dir = %v, want %v", parsed["mask_dir"], filepath.Join(dir, "aivpn-masks"))
	}
	// IPv6 disabled by default: no ipv6_prefix key, ipv6_enabled false.
	if network["ipv6_enabled"] != false {
		t.Fatalf("ipv6_enabled default = %v", network["ipv6_enabled"])
	}
	if _, ok := network["ipv6_prefix"]; ok {
		t.Fatalf("ipv6_prefix must be omitted when IPv6 is off")
	}
}

func TestAivpnKeyFileWrittenOnce(t *testing.T) {
	dir := t.TempDir()
	prev := tunnelDir
	tunnelDir = func() string { return dir }
	defer func() { tunnelDir = prev }()

	if err := EnsureAivpnKeyFile(AivpnKey); err != nil {
		t.Fatalf("mint: %v", err)
	}
	keyPath := filepath.Join(dir, "aivpn-data", "server.key")
	first, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if len(first) != 32 {
		t.Fatalf("server key must be 32 bytes, got %d", len(first))
	}
	// Second call must keep the same key (Rule 0: never rotate silently).
	if err := EnsureAivpnKeyFile(AivpnKey); err != nil {
		t.Fatalf("re-ensure: %v", err)
	}
	second, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("re-read key: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("server key was rewritten by EnsureKeyFile")
	}
}

func writeAivpnTestMask(dir string) error {
	masks := filepath.Join(dir, "aivpn-masks")
	if err := os.MkdirAll(masks, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(masks, "quic_https_v2.json"),
		[]byte(`{"mask_id":"quic_https_v2"}`), 0o600)
}

func TestAivpnInstanceFromInbound(t *testing.T) {
	dir := t.TempDir()
	prev := tunnelDir
	tunnelDir = func() string { return dir }
	defer func() { tunnelDir = prev }()

	ib := &model.Inbound{
		Remark:   "aivpn test",
		Port:     45001,
		Protocol: model.Aivpn,
		Enable:   false,
	}
	if _, ok := AivpnInstanceFromInbound(ib); !ok {
		t.Fatalf("expected an instance")
	}
	ib.Enable = true
	// An enabled inbound without mask profiles must not spawn a
	// crash-looping server — it stays down until the masks arrive.
	inst, ok := AivpnInstanceFromInbound(ib)
	if !ok || inst.Enabled {
		t.Fatalf("enabled inbound without masks must not spawn")
	}
	if err := writeAivpnTestMask(dir); err != nil {
		t.Fatalf("mask fixture: %v", err)
	}
	inst, ok = AivpnInstanceFromInbound(ib)
	if !ok || !inst.Enabled {
		t.Fatalf("expected an enabled instance")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(inst.ConfigText), &cfg); err != nil {
		t.Fatalf("instance config not JSON: %v", err)
	}
	if cfg["listen_addr"] != "0.0.0.0:45001" {
		t.Fatalf("listen_addr = %v", cfg["listen_addr"])
	}
	if cfg["mask_dir"] != filepath.Join(dir, "aivpn-masks") {
		t.Fatalf("mask_dir = %v, want %v", cfg["mask_dir"], filepath.Join(dir, "aivpn-masks"))
	}
	// Args must point the core at the rendered config file.
	if len(inst.Args) != 2 || inst.Args[0] != "--config" {
		t.Fatalf("args = %v", inst.Args)
	}
	if _, ok := AivpnConfigFromInbound(ib); !ok {
		t.Fatalf("config parse failed")
	}
	// Fingerprint must differ between two different ports (restart on change).
	other := *ib
	other.Port = 45002
	otherInst, _ := AivpnInstanceFromInbound(&other)
	if otherInst.Fingerprint() == inst.Fingerprint() {
		t.Fatalf("fingerprint must change with the port")
	}
}

// aivpnTestAPI returns an API client whose "unix" dialer lands on the test
// server — the same Transport shape the real client uses.
func aivpnTestAPI(t *testing.T, srv *httptest.Server) *AivpnAPI {
	t.Helper()
	return &AivpnAPI{hc: &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "tcp", srv.Listener.Addr().String())
			},
		},
	}}
}

func TestAivpnClientKeyEndpoint(t *testing.T) {
	scheme := "aivpn://c3R1Yi1ib2R5"
	const pskStripped = `{"id":"abc","name":"x","vpn_ip":"10.201.0.2","enabled":true}`

	t.Run("v1.2.0 serves the key from the connection-key endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/clients/abc/connection-key" {
				t.Errorf("path = %s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"connection_key":"` + scheme + `"}`))
		}))
		defer srv.Close()
		key, err := aivpnTestAPI(t, srv).ClientKey(context.Background(), "abc")
		if err != nil || key != scheme {
			t.Fatalf("key = %q err = %v", key, err)
		}
	})
	t.Run("legacy payload fallback", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/clients/abc" {
				_, _ = w.Write([]byte(`{"id":"abc","name":"x","connection_key":"` + scheme + `"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		key, err := aivpnTestAPI(t, srv).ClientKey(context.Background(), "abc")
		if err != nil || key != scheme {
			t.Fatalf("key = %q err = %v", key, err)
		}
	})
	t.Run("no key anywhere is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/connection-key") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(pskStripped))
		}))
		defer srv.Close()
		if _, err := aivpnTestAPI(t, srv).ClientKey(context.Background(), "abc"); err == nil {
			t.Fatalf("expected an error for a stripped payload")
		}
	})
}
