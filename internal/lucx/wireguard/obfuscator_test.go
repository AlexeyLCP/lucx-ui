// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

func obfsInbound(enabled, block bool) *model.Inbound {
	settings := `{"kernel":true,"secretKey":"k","address":"10.8.0.1/24","tproxyPort":40000,"mtu":1420,
"obfuscation":{"enabled":` + boolJSON(enabled) + `,"blockDirectPort":` + boolJSON(block) + `,"presets":[
 {"id":"p1","name":"phobos-ru","extPort":51822,"key":"k1","masking":"STUN",
  "maxDummy":4,"idleTimeout":300,"clientWgLocalPort":13255,"isDefault":true}
]}}`
	return &model.Inbound{Id: 3, Port: 51820, Protocol: model.WireGuard, Enable: true, Settings: settings}
}

func boolJSON(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestObfuscationTargets(t *testing.T) {
	inst, fw, ok := ObfuscationTargets(obfsInbound(true, true))
	if !ok {
		t.Fatal("obfuscated inbound must produce targets")
	}
	if inst.Core != tunnel.Wgo || inst.Key != "wgo-3" || !inst.Enabled {
		t.Fatalf("instance = %+v", inst)
	}
	if !strings.Contains(inst.ConfigText, "target = 127.0.0.1:51820") ||
		!strings.Contains(inst.ConfigText, "source-lport = 51822") {
		t.Fatalf("config:\n%s", inst.ConfigText)
	}
	if fw != (WgoFirewallState{ID: 3, WGPort: 51820, Block: true}) {
		t.Fatalf("fw = %+v", fw)
	}

	inst, fw, ok = ObfuscationTargets(obfsInbound(true, false))
	if !ok || fw.Block {
		t.Fatalf("block off: %+v", fw)
	}

	inst, _, ok = ObfuscationTargets(obfsInbound(false, true))
	if !ok || inst.Enabled {
		t.Fatalf("disabled block must yield disabled instance: %+v", inst)
	}

	disabledInbound := obfsInbound(true, true)
	disabledInbound.Enable = false
	inst, fw, ok = ObfuscationTargets(disabledInbound)
	if !ok || inst.Enabled || fw.Block {
		t.Fatalf("disabled inbound must yield disabled targets: %+v %+v", inst, fw)
	}

	plain := obfsInbound(true, true)
	plain.Settings = `{"kernel":true,"secretKey":"k","address":"10.8.0.1/24","tproxyPort":40000,"mtu":1420}`
	if _, _, ok := ObfuscationTargets(plain); ok {
		t.Fatal("plain inbound must not produce targets")
	}
	if _, _, ok := ObfuscationTargets(nil); ok {
		t.Fatal("nil inbound must not produce targets")
	}
}
