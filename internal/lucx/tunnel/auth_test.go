// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInboundAuthPair_EmptySeedKeepsPanelHMAC(t *testing.T) {
	secret := []byte("master-secret")
	ib := &model.Inbound{Id: 15, Protocol: model.Mieru, Settings: `{"clients":[]}`}
	got := InboundAuthPair(secret, ib, "a@x")
	want := MieruClientAuth(secret, 15, "a@x")
	if got != want {
		t.Fatalf("empty seed must equal old HMAC, got %+v want %+v", got, want)
	}
}

func TestInboundAuthPair_SeedIgnoresPanelSecretAndId(t *testing.T) {
	settings := SetAuthSeed(`{"clients":[{"email":"a@x","enable":true}]}`, "seed-one")
	master := &model.Inbound{Id: 15, Protocol: model.Naive, Settings: settings}
	node := &model.Inbound{Id: 3, Protocol: model.Naive, Settings: settings}
	a := InboundAuthPair([]byte("master-secret"), master, "a@x")
	b := InboundAuthPair([]byte("node-secret"), node, "a@x")
	if a != b {
		t.Fatal("same authSeed must yield the same pair on master and node")
	}
	if a == ClientAuthForInbound([]byte("master-secret"), 15, "a@x") {
		t.Fatal("seeded pair must not equal panel-secret HMAC")
	}
}

func TestEnsureAuthSeed_IdempotentAndPreserve(t *testing.T) {
	first, ok := EnsureAuthSeed(`{"domain":"n.example.org"}`)
	if !ok || AuthSeed(first) == "" {
		t.Fatalf("mint failed: changed=%v seed=%q", ok, AuthSeed(first))
	}
	second, ok := EnsureAuthSeed(first)
	if ok || AuthSeed(second) != AuthSeed(first) {
		t.Fatal("second EnsureAuthSeed must keep the seed")
	}
	stripped := `{"domain":"n.example.org","clients":[]}`
	got := PreserveAuthSeed(first, stripped)
	if AuthSeed(got) != AuthSeed(first) {
		t.Fatalf("preserve lost seed: %s", got)
	}
	if !strings.Contains(got, `"domain":"n.example.org"`) {
		t.Fatalf("preserve dropped domain: %s", got)
	}
}
