// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAwgTproxyPortReservedForTCPAndUDP(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-bridge", "", 51820, model.AWG, "", `{"routeThroughXray":true,"xrayRoutingMode":"tproxy","tproxyPort":51453}`)
	for _, protocol := range []model.Protocol{model.VLESS, model.WireGuard} {
		conflict, err := (&InboundService{}).checkPortConflict(&model.Inbound{Listen: "0.0.0.0", Port: 51453, Protocol: protocol}, 0)
		if err != nil || conflict == nil || !conflict.Relay {
			t.Fatalf("unreserved TPROXY port: %v, %v", conflict, err)
		}
	}
	conflict, err := (&InboundService{}).checkPortConflict(&model.Inbound{Listen: "192.0.2.1", Port: 51453, Protocol: model.VLESS}, 0)
	if err != nil || conflict != nil {
		t.Fatal("distinct bind address falsely conflicts")
	}
}

func TestInjectAwgTproxyEgress(t *testing.T) {
	for _, target := range []string{"", "warp"} {
		cfg := egressTestConfig()
		before := string(cfg.RouterConfig)
		ib := awgInbound("inbound-awg-1", `{"routeThroughXray":true,"xrayRoutingMode":"tproxy","tproxyPort":51453,"outboundTag":"`+target+`"}`)
		injectAwgEgress(cfg, ib)
		if len(cfg.InboundConfigs) != 2 {
			t.Fatal("bridge not injected")
		}
		bridge := cfg.InboundConfigs[1]
		if bridge.Protocol != "dokodemo-door" || bridge.Port != 51453 || bridge.Tag != ib.Tag {
			t.Fatal("wrong bridge", bridge)
		}
		if target == "" && string(cfg.RouterConfig) != before {
			t.Fatal("general routing was overwritten")
		}
		if target != "" {
			var routing egressRouting
			if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
				t.Fatal(err)
			}
			if routing.Rules[0].OutboundTag != target {
				t.Fatal("forced outbound lost")
			}
		}
		if !awgRoutesThroughXray(ib) {
			t.Fatal("save/delete will not request Xray restart")
		}
		injectAwgEgress(cfg, ib)
		if len(cfg.InboundConfigs) != 2 {
			t.Fatal("duplicate bridge tag")
		}
	}
}

func TestInjectAwgTproxyInvalidOrDisabled(t *testing.T) {
	for _, settings := range []string{
		`{"routeThroughXray":true,"xrayRoutingMode":"tproxy","tproxyPort":0,"outboundTag":"warp"}`,
		`{"routeThroughXray":false,"xrayRoutingMode":"tproxy","tproxyPort":51453}`,
	} {
		cfg := egressTestConfig()
		before := string(cfg.RouterConfig)
		injectAwgEgress(cfg, awgInbound("awg", settings))
		if len(cfg.InboundConfigs) != 1 || string(cfg.RouterConfig) != before {
			t.Fatal("invalid/disabled mode modified config")
		}
	}
}

func TestAwgTproxyNoEmbeddedFallback(t *testing.T) {
	_, ok := kernelAwgToEmbedded(awg.Instance{RouteThroughXray: true, XrayRoutingMode: "tproxy", PrivateKey: strings.Repeat("A", 43) + "=", Port: 12345})
	if ok {
		t.Fatal("explicit TPROXY silently became userspace AWG")
	}
}
