// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNaiveKey(t *testing.T) {
	if got, want := NaiveKey(12), "naive-12"; got != want {
		t.Fatalf("NaiveKey = %q, want %q", got, want)
	}
}

func TestClientAuthForInbound_Scoped(t *testing.T) {
	secret := []byte("panel-secret")
	a := ClientAuthForInbound(secret, 1, "alice@example.com")
	b := ClientAuthForInbound(secret, 2, "alice@example.com")
	if a.User == b.User || a.Pass == b.Pass {
		t.Fatal("same email on different inbounds must get different credentials")
	}
	a2 := ClientAuthForInbound(secret, 1, "alice@example.com")
	if a != a2 {
		t.Fatal("ClientAuthForInbound must be deterministic")
	}
}

func TestInstanceFromInbound_RendersClients(t *testing.T) {
	ib := &model.Inbound{
		Id:       7,
		Enable:   true,
		Port:     8443,
		Protocol: model.Naive,
		Settings: `{
			"domain":"n.example.com",
			"useAcme":false,
			"certFile":"/c.pem",
			"keyFile":"/k.pem",
			"authUser":"svc",
			"authPass":"svcpass",
			"clients":[
				{"email":"a@x","enable":true},
				{"email":"b@x","enable":false}
			]
		}`,
	}
	secret := []byte("secret")
	inst, ok := InstanceFromInbound(ib, secret)
	if !ok {
		t.Fatal("expected ok")
	}
	if inst.ManageKey() != "naive-7" {
		t.Fatalf("key = %q", inst.ManageKey())
	}
	if !inst.Enabled {
		t.Fatal("expected enabled")
	}
	if !strings.Contains(inst.ConfigText, `basic_auth "svc"`) {
		t.Fatalf("missing service auth:\n%s", inst.ConfigText)
	}
	pair := ClientAuthForInbound(secret, 7, "a@x")
	if !strings.Contains(inst.ConfigText, pair.User) {
		t.Fatalf("missing enabled client auth %q:\n%s", pair.User, inst.ConfigText)
	}
	off := ClientAuthForInbound(secret, 7, "b@x")
	if strings.Contains(inst.ConfigText, off.User) {
		t.Fatal("disabled client must not appear in Caddyfile")
	}
}

func TestInstanceFromInbound_Disabled(t *testing.T) {
	ib := &model.Inbound{Id: 1, Enable: false, Protocol: model.Naive, Port: 443, Settings: `{}`}
	inst, ok := InstanceFromInbound(ib, nil)
	if !ok || inst.Enabled {
		t.Fatalf("disabled inbound: ok=%v enabled=%v", ok, inst.Enabled)
	}
}
