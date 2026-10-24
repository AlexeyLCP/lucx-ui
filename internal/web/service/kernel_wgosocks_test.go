// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

func TestNormalizeWgosocksSettings(t *testing.T) {
	s := &InboundService{}
	ib := &model.Inbound{
		Id: 1, Port: 51862, Protocol: model.Wgosocks,
		Settings: `{"clients":[{"email":"u1@x","login":"","password":""}]}`,
	}
	s.normalizeWgosocksSettings(ib)

	cfg, ok := tunnel.WgosocksConfigFromInbound(ib)
	if !ok {
		t.Fatal("normalized settings must parse")
	}
	if len(cfg.Key) != 16 || cfg.Masking != tunnel.WgoMaskMedia || cfg.MaxDummy != 4 ||
		cfg.IdleTimeout != 300 || cfg.MaxClients != 1024 {
		t.Fatalf("normalized = %+v", cfg)
	}
	if len(cfg.Clients) != 1 || cfg.Clients[0].Login == "" || cfg.Clients[0].Password == "" {
		t.Fatalf("client creds not minted: %+v", cfg.Clients)
	}
	if !strings.Contains(ib.Settings, `"clients"`) {
		t.Fatal("clients must survive normalization")
	}

	// second save: minted values persist, not regenerated (Rule 0)
	oldKey := cfg.Key
	oldLogin := cfg.Clients[0].Login
	s.normalizeWgosocksSettings(ib)
	cfg, _ = tunnel.WgosocksConfigFromInbound(ib)
	if cfg.Key != oldKey || cfg.Clients[0].Login != oldLogin {
		t.Fatal("existing credentials must not rotate on re-save")
	}

	// validation hook shape: empty key after normalize is impossible, but a
	// bad masking must be rejected
	bad := &model.Inbound{
		Id: 2, Port: 51862, Protocol: model.Wgosocks,
		Settings: `{"key":"k","masking":"BOGUS","maxDummy":4,"idleTimeout":300,"maxClients":16,"clients":[]}`,
	}
	if err := tunnel.ValidateWgosocks(mustWgosocksCfg(t, bad), bad.Port, nil); err == nil {
		t.Fatal("bogus masking must be rejected")
	}
}

func mustWgosocksCfg(t *testing.T, ib *model.Inbound) tunnel.WgosocksSettings {
	t.Helper()
	cfg, ok := tunnel.WgosocksConfigFromInbound(ib)
	if !ok {
		t.Fatal("settings must parse")
	}
	return cfg
}
