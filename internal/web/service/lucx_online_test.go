// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"testing"
)

func TestMergeShareOnlySlimClients(t *testing.T) {
	got := mergeShareOnlySlimClients(`{"remark":"qwdtt"}`, []shareOnlySlimClient{
		{Email: "fox", Enable: true, Comment: "c"},
		{Email: " ", Enable: true},
		{Email: "owl", Enable: false},
	})
	var raw map[string]any
	if err := json.Unmarshal([]byte(got), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["remark"] != "qwdtt" {
		t.Fatalf("remark lost: %v", raw["remark"])
	}
	clients, ok := raw["clients"].([]any)
	if !ok || len(clients) != 2 {
		t.Fatalf("clients = %v", raw["clients"])
	}
	first, _ := clients[0].(map[string]any)
	if first["email"] != "fox" || first["comment"] != "c" || first["enable"] != true {
		t.Fatalf("first = %v", first)
	}
}

func TestOnlineProcessFallback(t *testing.T) {
	p := onlineProcess()
	if p == nil {
		t.Fatal("nil")
	}
	p.RefreshLocalOnline([]string{"fox@x"}, []string{"tproxy-1"}, 1000, 20000)
	got := p.GetLocalOnlineClients()
	if len(got) != 1 || got[0] != "fox@x" {
		t.Fatalf("online = %v", got)
	}
}
