// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A mieru inbound must reach a Clash-family client. mihomo has no mierus://
// link parser — it only knows the YAML proxy type — so without a Clash entry
// the node is missing from the subscription entirely (issue #202).
func TestBuildMieruProxy_MihomoEntry(t *testing.T) {
	initSubDB(t)
	settings, _ := json.Marshal(map[string]any{
		"portBindings":  []map[string]any{{"port": 20100, "protocol": "TCP"}},
		"mtu":           1400,
		"multiplexing":  "MULTIPLEXING_MIDDLE",
		"handshakeMode": "HANDSHAKE_STANDARD",
	})
	ib := &model.Inbound{
		Id:       7,
		Protocol: model.Mieru,
		Port:     20100,
		Listen:   "1.2.3.4",
		Remark:   "mieru-test",
		Tag:      "mieru-7",
		Enable:   true,
		Settings: string(settings),
	}
	client := model.Client{Email: "alice@test"}

	subReq := (&SubService{}).ForRequest("sub.example.net")
	svc := NewSubClashService(false, "", false, subReq)
	proxy := svc.buildMieruProxy(subReq, ib, client, map[string]any{})
	if proxy == nil {
		t.Fatal("buildMieruProxy returned nil")
	}
	if proxy["type"] != "mieru" {
		t.Fatalf("type = %v, want mieru", proxy["type"])
	}
	if proxy["server"] != "1.2.3.4" {
		t.Fatalf("server = %v, want 1.2.3.4", proxy["server"])
	}
	if proxy["port"] != 20100 {
		t.Fatalf("port = %v, want 20100", proxy["port"])
	}
	if proxy["transport"] != "TCP" {
		t.Fatalf("transport = %v, want TCP", proxy["transport"])
	}
	if proxy["username"] == "" || proxy["password"] == "" {
		t.Fatal("username/password must be the HMAC pair")
	}
	if proxy["multiplexing"] != "MULTIPLEXING_MIDDLE" {
		t.Fatalf("multiplexing = %v", proxy["multiplexing"])
	}
	if proxy["handshake-mode"] != "HANDSHAKE_STANDARD" {
		t.Fatalf("handshake-mode = %v", proxy["handshake-mode"])
	}
}

// A disabled inbound must not be exported, and a port range rides in
// port-range (mihomo takes one port xor one range per proxy).
func TestBuildMieruProxy_SkipsDisabledAndCarriesRange(t *testing.T) {
	initSubDB(t)
	settings, _ := json.Marshal(map[string]any{
		"portBindings": []map[string]any{{"portRange": "30000-30005", "protocol": "UDP"}},
	})
	ib := &model.Inbound{
		Id:       8,
		Protocol: model.Mieru,
		Port:     30000,
		Listen:   "1.2.3.4",
		Enable:   false,
		Settings: string(settings),
	}
	client := model.Client{Email: "bob@test"}
	subReq := (&SubService{}).ForRequest("sub.example.net")
	svc := NewSubClashService(false, "", false, subReq)

	if proxy := svc.buildMieruProxy(subReq, ib, client, map[string]any{}); proxy != nil {
		t.Fatalf("disabled inbound must not be exported: %+v", proxy)
	}

	ib.Enable = true
	proxy := svc.buildMieruProxy(subReq, ib, client, map[string]any{})
	if proxy == nil {
		t.Fatal("buildMieruProxy returned nil")
	}
	if proxy["port-range"] != "30000-30005" {
		t.Fatalf("port-range = %v, want 30000-30005", proxy["port-range"])
	}
	if _, ok := proxy["port"]; ok {
		t.Fatal("port and port-range are mutually exclusive in mihomo")
	}
	if proxy["transport"] != "UDP" {
		t.Fatalf("transport = %v, want UDP", proxy["transport"])
	}
}

// buildProxy must route a mieru inbound into the mihomo entry.
func TestBuildProxy_DispatchesMieru(t *testing.T) {
	initSubDB(t)
	settings, _ := json.Marshal(map[string]any{
		"portBindings": []map[string]any{{"port": 20100, "protocol": "TCP"}},
	})
	ib := &model.Inbound{
		Id:       9,
		Protocol: model.Mieru,
		Port:     20100,
		Listen:   "9.9.9.9",
		Enable:   true,
		Settings: string(settings),
	}
	client := model.Client{Email: "c@test"}
	subReq := (&SubService{}).ForRequest("sub.example.net")
	svc := NewSubClashService(false, "", false, subReq)
	proxy := svc.buildProxy(subReq, ib, client, map[string]any{}, map[string]any{})
	if proxy == nil {
		t.Fatal("buildProxy must dispatch mieru")
	}
	if proxy["type"] != "mieru" {
		t.Fatalf("type = %v, want mieru", proxy["type"])
	}
}
