// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAnytlsValidate(t *testing.T) {
	if err := (AnytlsConfig{Port: 8443, Password: "s3cret"}).Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := (AnytlsConfig{Port: 0, Password: "x"}).Validate(); err == nil {
		t.Fatal("port 0 must be rejected")
	}
	if err := (AnytlsConfig{Port: 70000, Password: "x"}).Validate(); err == nil {
		t.Fatal("port 70000 must be rejected")
	}
	if err := (AnytlsConfig{Port: 8443}).Validate(); err == nil {
		t.Fatal("empty password must be rejected")
	}
}

func TestAnytlsBuildArgsAndListen(t *testing.T) {
	cfg := AnytlsConfig{Port: 9443, Password: "pass word"}
	args := cfg.BuildArgs()
	want := []string{"-l", "0.0.0.0:9443", "-p", "pass word"}
	if len(args) != len(want) {
		t.Fatalf("args = %v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestAnytlsClientLink(t *testing.T) {
	cfg := AnytlsConfig{Port: 8443, Password: "hunter2"}
	got := cfg.ClientLink("node.example", "home")
	if !strings.HasPrefix(got, "anytls://hunter2@node.example:8443/") {
		t.Fatalf("ClientLink = %q", got)
	}
	if !strings.Contains(got, "insecure=1") {
		t.Fatalf("self-signed anytls-server needs insecure=1: %q", got)
	}
	if !strings.Contains(got, "#home") {
		t.Fatalf("remark fragment missing: %q", got)
	}
	got = (AnytlsConfig{Port: 443, Password: "p@ss"}).ClientLink("203.0.113.9", "")
	if !strings.Contains(got, "p%40ss") {
		t.Fatalf("password must be percent-encoded: %q", got)
	}
	if cfg.ClientLink("", "x") != "" {
		t.Fatal("empty host must yield empty link")
	}
}

func TestAnytlsEnsurePassword(t *testing.T) {
	cfg := AnytlsConfig{Port: 8443}
	out, err := cfg.EnsurePassword()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Password) != 24 {
		t.Fatalf("password len = %d, want 24", len(out.Password))
	}
	again, err := out.EnsurePassword()
	if err != nil {
		t.Fatal(err)
	}
	if again.Password != out.Password {
		t.Fatal("EnsurePassword must not rotate an existing password")
	}
}

func TestAnytlsInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Id:       12,
		Protocol: model.Anytls,
		Enable:   true,
		Remark:   "anytls-home",
		Settings: `{"port": 9443, "password": "shared"}`,
	}
	inst, ok := AnytlsInstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected ok")
	}
	if inst.Key != "anytls-12" {
		t.Fatalf("Key = %q", inst.Key)
	}
	if inst.Core != Anytls || !inst.Enabled {
		t.Fatalf("inst = %+v", inst)
	}
	if inst.ProbePort != 9443 {
		t.Fatalf("ProbePort = %d", inst.ProbePort)
	}
	if strings.Join(inst.Args, " ") != "-l 0.0.0.0:9443 -p shared" {
		t.Fatalf("Args = %v", inst.Args)
	}

	disabled := *ib
	disabled.Enable = false
	inst, ok = AnytlsInstanceFromInbound(&disabled)
	if !ok || inst.Enabled {
		t.Fatalf("disabled inbound must yield Enabled:false, got %+v", inst)
	}

	other := &model.Inbound{Id: 1, Protocol: model.VLESS}
	if _, ok := AnytlsInstanceFromInbound(other); ok {
		t.Fatal("non-anytls inbound must not map")
	}

	passwordless := &model.Inbound{
		Id:       7,
		Protocol: model.Anytls,
		Enable:   true,
		Settings: `{"port": 8443}`,
	}
	inst, ok = AnytlsInstanceFromInbound(passwordless)
	if !ok || inst.Enabled {
		t.Fatalf("passwordless inbound must stay down until save mints a password: %+v", inst)
	}

	ported := &model.Inbound{
		Id:       3,
		Protocol: model.Anytls,
		Enable:   true,
		Port:     9444,
		Settings: `{"port": 8443, "password": "x"}`,
	}
	inst, ok = AnytlsInstanceFromInbound(ported)
	if !ok || inst.ProbePort != 9444 {
		t.Fatalf("inbound.Port must win over settings.port: %+v", inst)
	}
}
