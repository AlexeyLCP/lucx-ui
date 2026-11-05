// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A sidecar masked by the SNI gateway listens on loopback and only sees the
// gateway; its real client IPs are on the gateway's public socket, which the
// gateway's Host row names.
func TestSidecarEndpointsBehindGateway(t *testing.T) {
	initClientHwidTestDB(t)
	db := database.GetDB()
	any := &model.Inbound{Tag: "in-anytls", Protocol: model.Anytls, Enable: true, Listen: "127.0.0.1", Port: 1445, Settings: "{}", StreamSettings: "{}"}
	cover := &model.Inbound{Tag: "in-cover", Protocol: model.Cover, Enable: true, Listen: "127.0.0.1", Port: 8443, Settings: "{}", StreamSettings: "{}"}
	for _, ib := range []*model.Inbound{any, cover} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, h := range []model.Host{
		{InboundId: any.Id, Remark: "gateway", Address: "203.0.113.4", Port: 443},
		{InboundId: cover.Id, Remark: "gateway", Address: "site.example.com", Port: 443},
	} {
		if err := db.Create(&h).Error; err != nil {
			t.Fatal(err)
		}
	}
	eps := sidecarEndpoints()
	if got := eps.lookup("203.0.113.4:443"); got != "in-anytls" {
		t.Fatalf("masked AnyTLS: public endpoint maps to %q, want in-anytls (%v)", got, eps)
	}
	// Real client 198.51.100.7 on the gateway's socket, one live user: credited.
	sockets := map[string]map[string]int{"198.51.100.7": {"203.0.113.4:443": 1}, "127.0.0.1": {"127.0.0.1:1445": 1}}
	got := attributeSidecarIPs(sockets, eps, map[string][]string{"in-anytls": {"u@example.com"}})
	if a, ok := got["198.51.100.7"]; !ok || a.email != "u@example.com" || a.tag != "in-anytls" {
		t.Fatalf("real client IP not attributed: %+v", got)
	}
	if _, ok := got["127.0.0.1"]; ok {
		t.Fatal("the gateway's loopback hop must not count as a client IP")
	}
}
