// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func kernelWireguardTestInbound(t *testing.T) *model.Inbound {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))
	s := map[string]any{"kernel": true, "secretKey": key, "address": "10.90.0.1/24", "tproxyPort": 32101, "clients": []any{}}
	b, _ := json.Marshal(s)
	return &model.Inbound{Id: 7, Protocol: model.WireGuard, Enable: true, Tag: "wg-kernel-test", Port: 51820, Settings: string(b)}
}

func TestInjectKernelWireguardRetainsRouting(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.RouterConfig)
	ib := kernelWireguardTestInbound(t)
	if err := injectKernelWireguard(cfg, ib); err != nil {
		t.Fatal(err)
	}
	bridge := cfg.InboundConfigs[len(cfg.InboundConfigs)-1]
	if bridge.Port != 32101 || bridge.Tag != ib.Tag || bridge.Protocol != "dokodemo-door" {
		t.Fatal("missing kernel WireGuard transparent bridge")
	}
	if string(cfg.RouterConfig) != before {
		t.Fatal("kernel WireGuard changed routing order")
	}
	ib.Settings = `{"secretKey":"legacy","clients":[]}`
	count := len(cfg.InboundConfigs)
	if err := injectKernelWireguard(cfg, ib); err != nil || len(cfg.InboundConfigs) != count {
		t.Fatal("legacy WireGuard backend changed")
	}
}

func TestKernelWireguardTransparentPortReserved(t *testing.T) {
	setupConflictDB(t)
	ib := kernelWireguardTestInbound(t)
	seedInboundConflict(t, "kernel-wg", "", ib.Port, model.WireGuard, "", ib.Settings)
	conflict, err := (&InboundService{}).checkPortConflict(&model.Inbound{Listen: "127.0.0.1", Port: 32101, Protocol: model.VLESS}, 0)
	if err != nil || conflict == nil || !conflict.Relay {
		t.Fatal("kernel WireGuard TPROXY port was not reserved")
	}
}

func TestKernelWireguardAllocationUsesServerSubnet(t *testing.T) {
	base := explicitWireguardSubnetBase(`{"kernel":true,"address":"10.90.0.9/28"}`)
	if base != "10.90.0.0/28" {
		t.Fatalf("allocation escaped kernel subnet: %s", base)
	}
}

func TestKernelWireguardMetadataEditDoesNotRestartXray(t *testing.T) {
	setupEnablePortTest(t)
	ib := kernelWireguardTestInbound(t)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	edited := *ib
	edited.Remark = "metadata only"
	_, restart, err := (&InboundService{}).UpdateInbound(&edited)
	if err != nil {
		t.Fatal(err)
	}
	if restart {
		t.Fatal("metadata-only kernel WireGuard edit requested an Xray restart")
	}
}

func TestKernelWireguardUpdateUsesInPlaceRuntime(t *testing.T) {
	setupConflictDB(t)
	fake := &fakeNodeRuntime{}
	mgr := useTestRuntimeManager(t)
	mgr.SetLocalRuntimeOverride(fake)
	ib := kernelWireguardTestInbound(t)
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	edited := *ib
	edited.Settings = strings.Replace(ib.Settings, `"clients":[]`, `"clients":[],"mtu":1300`, 1)
	_, _, err := (&InboundService{}).UpdateInbound(&edited)
	if err != nil {
		t.Fatal(err)
	}
	if fake.updateInbound.Load() != 1 || fake.delInbound.Load() != 0 || fake.addInbound.Load() != 0 {
		t.Fatalf("kernel update bypassed in-place runtime: update=%d del=%d add=%d", fake.updateInbound.Load(), fake.delInbound.Load(), fake.addInbound.Load())
	}
}

func TestKernelWireguardAutoTagChangeRestartsBridge(t *testing.T) {
	setupEnablePortTest(t)
	ib := kernelWireguardTestInbound(t)
	ib.Tag = "in-51820-udp"
	ib.StreamSettings = `{"network":"udp"}`
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	edited := *ib
	edited.Port = 51821
	saved, restart, err := (&InboundService{}).UpdateInbound(&edited)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Tag == ib.Tag || !restart {
		t.Fatalf("auto-tag bridge not refreshed: tag=%s restart=%v", saved.Tag, restart)
	}
}
