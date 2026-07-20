// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientFingerprint_RestartDetection(t *testing.T) {
	o := &model.AwgOutbound{Id: 2, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	o.Settings = `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`
	ci2, _ := ClientInstanceFromOutbound(o)
	if ci1.fingerprint() == ci2.fingerprint() {
		t.Fatal("fingerprint must change when Address changes (restart trigger)")
	}
}

func TestCollectClientTraffic_NoInterface(t *testing.T) {
	m := GetManager()
	_, _, _, ok := m.CollectClientTraffic("awgo-99999")
	if ok {
		t.Error("CollectClientTraffic should return ok=false for non-existent interface")
	}
}

func TestSweepOrphanClients_Idempotent(t *testing.T) {
	m := GetManager()
	m.sweepOrphanClientsOnce()
	m.sweepOrphanClientsOnce()
}
