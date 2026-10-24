// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func testKey(n byte) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Repeat(string(n), 32)))
}

func testInbound(t *testing.T) *model.Inbound {
	t.Helper()
	settings := map[string]any{"kernel": true, "secretKey": testKey(1), "address": "10.90.0.1/24", "mtu": 1280, "tproxyPort": 32101, "clients": []map[string]any{{"email": "alice", "publicKey": testKey(2), "preSharedKey": testKey(3), "allowedIPs": []string{"10.90.0.2/32"}, "keepAlive": "25", "enable": true}, {"email": "disabled", "publicKey": testKey(4), "allowedIPs": []string{"10.90.0.3/32"}, "enable": false}}}
	b, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	return &model.Inbound{Id: 7, Protocol: model.WireGuard, Enable: true, Port: 51820, Tag: "wg-test", Settings: string(b)}
}

func TestKernelWireGuardOptInPreservesDefault(t *testing.T) {
	ib := testInbound(t)
	if !IsKernel(ib) {
		t.Fatal("kernel backend not detected")
	}
	ib.Settings = `{"secretKey":"legacy","clients":[]}`
	if IsKernel(ib) {
		t.Fatal("legacy backend changed")
	}
	ib.Protocol = model.AWG
	ib.Settings = `{"kernel":true}`
	if IsKernel(ib) {
		t.Fatal("AWG misclassified")
	}
}

func TestParseKernelWireGuard(t *testing.T) {
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	if inst.Name() != "lxwg7" || inst.Address != "10.90.0.1/24" || inst.MTU != 1280 || len(inst.Peers) != 1 || inst.Peers[0].Email != "alice" || inst.Peers[0].Keepalive != 25 {
		t.Fatalf("unexpected parsed instance: name=%s address=%s mtu=%d peers=%d", inst.Name(), inst.Address, inst.MTU, len(inst.Peers))
	}
}

func TestParseKernelWireGuardRejectsUnsafeSettings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"ipv6 server", func(s map[string]any) { s["address"] = "fd00::1/64" }},
		{"invalid secret", func(s map[string]any) { s["secretKey"] = "bad" }},
		{"transparent port", func(s map[string]any) { s["tproxyPort"] = 51820 }},
		{"ipv6 peer", func(s map[string]any) {
			s["clients"].([]any)[0].(map[string]any)["allowedIPs"] = []string{"fd00::2/128"}
		}},
		{"server as peer", func(s map[string]any) {
			s["clients"].([]any)[0].(map[string]any)["allowedIPs"] = []string{"10.90.0.1/32"}
		}},
		{"peer network", func(s map[string]any) {
			s["clients"].([]any)[0].(map[string]any)["allowedIPs"] = []string{"10.90.0.0/24"}
		}},
		{"keepalive range", func(s map[string]any) { s["clients"].([]any)[0].(map[string]any)["keepAlive"] = "15-25" }},
		{"duplicate key", func(s map[string]any) {
			p := s["clients"].([]any)
			p[1].(map[string]any)["enable"] = true
			p[1].(map[string]any)["publicKey"] = p[0].(map[string]any)["publicKey"]
		}},
		{"duplicate address", func(s map[string]any) {
			p := s["clients"].([]any)
			p[1].(map[string]any)["enable"] = true
			p[1].(map[string]any)["allowedIPs"] = p[0].(map[string]any)["allowedIPs"]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ib := testInbound(t)
			var s map[string]any
			if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
				t.Fatal(err)
			}
			tc.mutate(s)
			b, _ := json.Marshal(s)
			ib.Settings = string(b)
			if _, err := Parse(ib); err == nil {
				t.Fatal("unsafe settings accepted")
			}
		})
	}
}

func TestWireGuardConfigContainsNoObfuscationOrShellHooks(t *testing.T) {
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	conf := RenderConfig(inst)
	for _, want := range []string{"PrivateKey = " + testKey(1), "ListenPort = 51820", "Address = 10.90.0.1/24", "MTU = 1280", "Table = off", "PublicKey = " + testKey(2), "PresharedKey = " + testKey(3), "AllowedIPs = 10.90.0.2/32", "PersistentKeepalive = 25"} {
		if !strings.Contains(conf, want) {
			t.Fatalf("missing config field %s", strings.Split(want, " = ")[0])
		}
	}
	for _, bad := range []string{"HeaderProtectionKey", "Jc =", "H1 =", "PostUp", "PostDown", "DNS ="} {
		if strings.Contains(conf, bad) {
			t.Fatalf("unexpected field %s", bad)
		}
	}
	before := inst.DeviceFingerprint()
	inst.Peers = nil
	if inst.DeviceFingerprint() != before {
		t.Fatal("peer edit changes device fingerprint")
	}
	inst.MTU++
	if inst.DeviceFingerprint() == before {
		t.Fatal("MTU edit does not change device fingerprint")
	}
}

func TestTproxyRulesFailClosedAfterMarkRestore(t *testing.T) {
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	nft := RenderFirewall(inst)
	for _, want := range []string{"priority mangle + 2", "lxwg7", "tproxy ip to 127.0.0.1:32101", "0x50000007", "drop", "10.90.0.0/24"} {
		if !strings.Contains(nft, want) {
			t.Fatalf("missing firewall invariant %s", want)
		}
	}
}

func TestParseKernelWireguardFiltersQuotaAndExpiry(t *testing.T) {
	for _, stat := range []xray.ClientTraffic{{Email: "alice", Enable: false}, {Email: "alice", Enable: true, ExpiryTime: time.Now().Add(-time.Minute).UnixMilli()}, {Email: "alice", Enable: true, Total: 100, Up: 40, Down: 60}} {
		ib := testInbound(t)
		ib.ClientStats = []xray.ClientTraffic{stat}
		inst, err := Parse(ib)
		if err != nil {
			t.Fatal(err)
		}
		if len(inst.Peers) != 0 {
			t.Fatal("disabled, expired or exhausted peer remained active")
		}
	}
}

func TestTransportMarkCannotUseManagedTproxyRange(t *testing.T) {
	for _, mark := range []string{"0x50000007", "0x50000008"} {
		ib := testInbound(t)
		var s map[string]any
		json.Unmarshal([]byte(ib.Settings), &s)
		s["fwMark"] = mark
		b, _ := json.Marshal(s)
		ib.Settings = string(b)
		if _, err := Parse(ib); err == nil {
			t.Fatalf("reserved transport mark accepted: %s", mark)
		}
	}
}
