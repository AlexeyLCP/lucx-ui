// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestQwdttTunHelpers(t *testing.T) {
	if got := QwdttTunName(35); got != "tun35" {
		t.Fatalf("TunName: %s", got)
	}
	if got := QwdttRouteTable(35); got != 1935 {
		t.Fatalf("RouteTable: %d", got)
	}
	if got := QwdttTunGateway(35); got != "10.253.35.1/30" {
		t.Fatalf("Gateway: %s", got)
	}
}

func TestRuleMissingLookup(t *testing.T) {
	out := "0:\tfrom all lookup local\n32765:\tfrom all iif wdtt0 lookup 1935\n"
	if ruleMissingLookup(out, 1935) {
		t.Fatal("should find 1935")
	}
	if !ruleMissingLookup(out, 1900) {
		t.Fatal("should miss 1900")
	}
}

func TestQwdttConfigFromInbound_DefaultRouteThroughXray(t *testing.T) {
	ib := &model.Inbound{
		Id:       35,
		Protocol: model.Qwdtt,
		Enable:   true,
		Settings: `{"listenAddr":"0.0.0.0:52285","password":"x","dns":"8.8.8.8"}`,
	}
	cfg, ok := QwdttConfigFromInbound(ib)
	if !ok || !cfg.RouteThroughXray {
		t.Fatalf("missing key should default routeThroughXray=true, got %+v ok=%v", cfg, ok)
	}
	inst, ok := QwdttInstanceFromInbound(ib)
	if !ok || !inst.RouteThroughXray || inst.TunName != "tun35" || inst.RouteTable != 1935 {
		t.Fatalf("instance routing: %+v", inst)
	}

	ib2 := &model.Inbound{
		Id: 1, Protocol: model.Qwdtt, Enable: true,
		Settings: `{"listenAddr":"0.0.0.0:56000","password":"x","dns":"8.8.8.8","routeThroughXray":false}`,
	}
	cfg2, _ := QwdttConfigFromInbound(ib2)
	if cfg2.RouteThroughXray {
		t.Fatal("explicit false must stay false")
	}
	inst2, _ := QwdttInstanceFromInbound(ib2)
	if inst2.RouteThroughXray {
		t.Fatal("instance must not route when false")
	}
}

func TestInstanceFingerprintIncludesRoute(t *testing.T) {
	a := Instance{Core: Qwdtt, Key: QwdttKey, Args: []string{"-listen", "x"}, RouteThroughXray: false}
	b := Instance{Core: Qwdtt, Key: QwdttKey, Args: []string{"-listen", "x"}, RouteThroughXray: true, TunName: "tun1", RouteTable: 1901}
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("fingerprint must change when routeThroughXray toggles")
	}
}
