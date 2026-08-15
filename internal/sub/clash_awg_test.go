// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBuildAwgProxy_AmneziaOption(t *testing.T) {
	settings, _ := json.Marshal(map[string]any{
		"privateKey": "YAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=",
		"mtu":        1320,
		"dns":        "1.1.1.1",
		"awgVersion": "2",
		"jc":         5,
		"jmin":       50,
		"jmax":       100,
		"s1":         20,
		"s2":         30,
		"s3":         15,
		"s4":         15,
		"h1":         "1-100",
		"h2":         "101-200",
		"h3":         "201-300",
		"h4":         "301-400",
		"i1":         "<b 0xaa>",
		"i2":         "",
		"i3":         "",
		"i4":         "",
		"i5":         "",
	})
	ib := &model.Inbound{
		Id:       1,
		Protocol: model.AWG,
		Port:     51820,
		Listen:   "1.2.3.4",
		Remark:   "awg-test",
		Tag:      "awg-1",
		Settings: string(settings),
	}
	client := model.Client{
		Email:        "alice@test",
		PrivateKey:   "CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=",
		AllowedIPs:   []string{"10.200.0.2/32"},
		PreSharedKey: "PSKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=",
	}
	svc := NewSubClashService(false, "", &SubService{})
	proxy := svc.buildAwgProxy(svc.SubService, ib, client, map[string]any{})
	if proxy == nil {
		t.Fatal("buildAwgProxy returned nil")
	}
	if proxy["type"] != "wireguard" {
		t.Fatalf("type = %v, want wireguard", proxy["type"])
	}
	if proxy["private-key"] != client.PrivateKey {
		t.Fatalf("private-key missing")
	}
	if proxy["ip"] != "10.200.0.2" {
		t.Fatalf("ip = %v", proxy["ip"])
	}
	opt, ok := proxy["amnezia-wg-option"].(map[string]any)
	if !ok {
		t.Fatal("amnezia-wg-option missing")
	}
	if opt["version"] != 2 {
		t.Fatalf("version = %v, want 2", opt["version"])
	}
	if opt["jc"] != 5 {
		t.Fatalf("jc = %v", opt["jc"])
	}
	if opt["s3"] != 15 {
		t.Fatalf("s3 = %v (v2 should keep s3)", opt["s3"])
	}
	if opt["i1"] != "<b 0xaa>" {
		t.Fatalf("i1 = %v", opt["i1"])
	}
	if _, has := opt["header-protection-key"]; has {
		t.Fatal("v2 must not emit header-protection-key")
	}
	// public-key derived from privateKey
	if pk, _ := proxy["public-key"].(string); pk == "" {
		t.Fatal("public-key empty")
	}
}

func TestBuildAwgProxy_V15DropsS3I(t *testing.T) {
	settings, _ := json.Marshal(map[string]any{
		"privateKey": "YAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=",
		"awgVersion": "1.5",
		"jc":         3,
		"s1":         20,
		"s2":         30,
		"s3":         15,
		"i1":         "<b 0xaa>",
		"h1":         "1",
	})
	ib := &model.Inbound{Protocol: model.AWG, Port: 1, Listen: "1.1.1.1", Settings: string(settings)}
	client := model.Client{Email: "a", PrivateKey: "CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=", AllowedIPs: []string{"10.0.0.2/32"}}
	svc := NewSubClashService(false, "", &SubService{})
	proxy := svc.buildAwgProxy(svc.SubService, ib, client, nil)
	opt := proxy["amnezia-wg-option"].(map[string]any)
	if opt["version"] != 1 {
		t.Fatalf("version = %v, want 1 for 1.5", opt["version"])
	}
	if _, ok := opt["s3"]; ok {
		t.Fatal("v1.5 must drop s3")
	}
	if _, ok := opt["i1"]; ok {
		t.Fatal("v1.5 must drop i1")
	}
}

// An AWG inbound with no address of its own (wildcard listen, no node, no
// shareAddr) must advertise the host the subscription was fetched on — that
// Clash `server:` is the address the client dials.
func TestGetProxies_AwgWithoutOwnAddressUsesSubscriptionHost(t *testing.T) {
	initSubDB(t)
	settings, _ := json.Marshal(map[string]any{
		"privateKey": "YAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=",
		"awgVersion": "2",
	})
	ib := &model.Inbound{
		Id:       3,
		Protocol: model.AWG,
		Port:     51820,
		Remark:   "awg-fallback",
		Tag:      "awg-3",
		Settings: string(settings),
	}
	client := model.Client{
		Email:      "carol@test",
		PrivateKey: "CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=",
		AllowedIPs: []string{"10.200.0.4/32"},
	}

	subReq := (&SubService{}).ForRequest("sub.example.net")
	svc := NewSubClashService(false, "", subReq)
	proxies := svc.getProxies(subReq, ib, client, "sub.example.net")

	if len(proxies) != 1 {
		t.Fatalf("proxies = %d, want 1", len(proxies))
	}
	if got := proxies[0]["server"]; got != "sub.example.net" {
		t.Fatalf("server = %v, want sub.example.net", got)
	}
}

func TestBuildProxy_DispatchesAwg(t *testing.T) {
	settings, _ := json.Marshal(map[string]any{
		"privateKey": "YAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=",
		"awgVersion": "2",
		"jc":         4,
	})
	ib := &model.Inbound{Protocol: model.AWG, Port: 9, Listen: "9.9.9.9", Settings: string(settings)}
	client := model.Client{Email: "b", PrivateKey: "CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=", AllowedIPs: []string{"10.0.0.3/32"}}
	svc := NewSubClashService(false, "", &SubService{})
	proxy := svc.buildProxy(svc.SubService, ib, client, map[string]any{}, map[string]any{})
	if proxy == nil {
		t.Fatal("buildProxy must dispatch AWG")
	}
	raw, _ := json.Marshal(proxy)
	if !strings.Contains(string(raw), "amnezia-wg-option") {
		t.Fatalf("proxy JSON missing amnezia-wg-option: %s", raw)
	}
}
