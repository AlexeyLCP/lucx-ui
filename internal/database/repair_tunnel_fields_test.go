// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The shape below is the one measured on a live node after it updated: the AWG
// inbound still held the working values, the VLESS inbound held a stale copy,
// and the client record had been overwritten with the copy — keys and the
// address, whose subnet belonged to a different server entirely.
const (
	liveTunnelKey  = "awg-live-private"
	liveTunnelPub  = "awg-live-public"
	liveTunnelPSK  = "awg-live-psk"
	liveTunnelAddr = "10.10.0.2/32"
	copyTunnelKey  = "vless-copy-private"
	copyTunnelPub  = "vless-copy-public"
	copyTunnelPSK  = "vless-copy-psk"
	copyTunnelAddr = "10.8.0.3/32"
)

func repairTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	// InitDB already ran the seeder; clear the row so each case drives a real pass.
	if err := db.Where("seeder_name = ?", "RepairClobberedTunnelFields").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder row: %v", err)
	}
}

func clientJSON(t *testing.T, email string, f tunnelFields) string {
	t.Helper()
	obj := map[string]any{"email": email, "enable": true}
	if f.priv != "" {
		obj["privateKey"] = f.priv
	}
	if f.pub != "" {
		obj["publicKey"] = f.pub
	}
	if f.psk != "" {
		obj["preSharedKey"] = f.psk
	}
	if f.allowed != "" {
		obj["allowedIPs"] = []string{f.allowed}
	}
	blob, err := json.Marshal(map[string]any{"clients": []any{obj}})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	return string(blob)
}

func seedRepairInbound(t *testing.T, port int, proto model.Protocol, settings string) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{UserId: 1, Port: port, Protocol: proto, Tag: string(proto) + "-" + itoa(port), Settings: settings}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	return ib
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func seedRepairRecord(t *testing.T, email string, f tunnelFields) *model.ClientRecord {
	t.Helper()
	rec := &model.ClientRecord{
		Email: email, SubID: "sub-" + email, Enable: true,
		PrivateKey: f.priv, PublicKey: f.pub, PreSharedKey: f.psk, AllowedIPs: f.allowed,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}
	return rec
}

func reloadRecord(t *testing.T, email string) model.ClientRecord {
	t.Helper()
	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("reload record: %v", err)
	}
	return rec
}

func live() tunnelFields {
	return tunnelFields{priv: liveTunnelKey, pub: liveTunnelPub, psk: liveTunnelPSK, allowed: liveTunnelAddr}
}

func copied() tunnelFields {
	return tunnelFields{priv: copyTunnelKey, pub: copyTunnelPub, psk: copyTunnelPSK, allowed: copyTunnelAddr}
}

func TestRepairClobberedTunnelFields_RestoresTheClobberedRecord(t *testing.T) {
	repairTestDB(t)
	const email = "demo-user@example.test"
	seedRepairInbound(t, 41001, model.AWG, clientJSON(t, email, live()))
	seedRepairInbound(t, 41002, model.VLESS, clientJSON(t, email, copied()))
	seedRepairRecord(t, email, copied())

	if err := repairClobberedTunnelFields(); err != nil {
		t.Fatalf("repair: %v", err)
	}

	got := reloadRecord(t, email)
	if got.PrivateKey != liveTunnelKey || got.PublicKey != liveTunnelPub || got.PreSharedKey != liveTunnelPSK {
		t.Errorf("keys not restored: (%q, %q, %q)", got.PrivateKey, got.PublicKey, got.PreSharedKey)
	}
	if got.AllowedIPs != liveTunnelAddr {
		t.Errorf("address not restored: %q, want %q", got.AllowedIPs, liveTunnelAddr)
	}
}

// The whole point of the narrow predicate: a value that matches no keyless copy
// is the operator's, and the repair has no business touching it.
func TestRepairClobberedTunnelFields_LeavesAnUnexplainedValueAlone(t *testing.T) {
	repairTestDB(t)
	const email = "hand-edited@example.test"
	seedRepairInbound(t, 41011, model.AWG, clientJSON(t, email, live()))
	seedRepairInbound(t, 41012, model.VLESS, clientJSON(t, email, copied()))
	own := tunnelFields{priv: "operator-typed-key", pub: "operator-typed-pub", psk: "operator-psk", allowed: "10.55.0.9/32"}
	seedRepairRecord(t, email, own)

	if err := repairClobberedTunnelFields(); err != nil {
		t.Fatalf("repair: %v", err)
	}

	got := reloadRecord(t, email)
	if got.PrivateKey != own.priv || got.PublicKey != own.pub || got.PreSharedKey != own.psk || got.AllowedIPs != own.allowed {
		t.Fatalf("a record matching no keyless copy was rewritten: %+v", got)
	}
}

// Two tunnel inbounds disagreeing means there is no single truth to restore.
func TestRepairClobberedTunnelFields_SkipsWhenTunnelInboundsDisagree(t *testing.T) {
	repairTestDB(t)
	const email = "two-tunnels@example.test"
	other := live()
	other.priv = "a-second-awg-inbound-key"
	seedRepairInbound(t, 41021, model.AWG, clientJSON(t, email, live()))
	seedRepairInbound(t, 41022, model.AWG, clientJSON(t, email, other))
	seedRepairInbound(t, 41023, model.VLESS, clientJSON(t, email, copied()))
	seedRepairRecord(t, email, copied())

	if err := repairClobberedTunnelFields(); err != nil {
		t.Fatalf("repair: %v", err)
	}

	got := reloadRecord(t, email)
	if got.PrivateKey != copyTunnelKey {
		t.Errorf("the contested key must be left untouched, got %q", got.PrivateKey)
	}
	// The fields the two inbounds agree on are still repairable.
	if got.PublicKey != liveTunnelPub {
		t.Errorf("an uncontested field should still be restored, got %q", got.PublicKey)
	}
}

func TestRepairClobberedTunnelFields_DrainsTheKeylessInbound(t *testing.T) {
	repairTestDB(t)
	const email = "drain@example.test"
	seedRepairInbound(t, 41031, model.AWG, clientJSON(t, email, live()))
	keyless := seedRepairInbound(t, 41032, model.VLESS, clientJSON(t, email, copied()))
	seedRepairRecord(t, email, copied())

	if err := repairClobberedTunnelFields(); err != nil {
		t.Fatalf("repair: %v", err)
	}
	if err := stripTunnelFieldsFromKeylessInbounds(); err != nil {
		t.Fatalf("drain: %v", err)
	}

	if rec := reloadRecord(t, email); rec.PublicKey != liveTunnelPub {
		t.Fatalf("repair must have run before the drain; record still holds %q", rec.PublicKey)
	}

	var got model.Inbound
	if err := db.First(&got, keyless.Id).Error; err != nil {
		t.Fatalf("reload keyless inbound: %v", err)
	}
	for _, key := range []string{"privateKey", "publicKey", "preSharedKey", "allowedIPs"} {
		if strings.Contains(got.Settings, key) {
			t.Errorf("%s survived in the keyless inbound: %s", key, got.Settings)
		}
	}

	var tunnel model.Inbound
	if err := db.Where("port = ?", 41031).First(&tunnel).Error; err != nil {
		t.Fatalf("reload tunnel inbound: %v", err)
	}
	if !strings.Contains(tunnel.Settings, liveTunnelKey) {
		t.Fatalf("the tunnel inbound must keep its own credentials: %s", tunnel.Settings)
	}
}

func TestRepairClobberedTunnelFields_RunsOnce(t *testing.T) {
	repairTestDB(t)
	const email = "once@example.test"
	seedRepairInbound(t, 41041, model.AWG, clientJSON(t, email, live()))
	seedRepairInbound(t, 41042, model.VLESS, clientJSON(t, email, copied()))
	seedRepairRecord(t, email, copied())

	if err := repairClobberedTunnelFields(); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	// A second pass must be a no-op even though the record now differs from the
	// keyless copy that is no longer there.
	if err := repairClobberedTunnelFields(); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if got := reloadRecord(t, email); got.PrivateKey != liveTunnelKey {
		t.Fatalf("second pass changed the record: %q", got.PrivateKey)
	}

	var rows int64
	if err := db.Model(&model.HistoryOfSeeders{}).
		Where("seeder_name = ?", "RepairClobberedTunnelFields").Count(&rows).Error; err != nil {
		t.Fatalf("count seeder rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("seeder row count = %d, want 1", rows)
	}
}

// Order is load-bearing, not incidental: the stale copy is the evidence the
// repair recognises, and the drain removes it. Run the two the wrong way round
// and the record stays broken with nothing left to diagnose it from.
func TestRepairClobberedTunnelFields_DrainFirstLosesTheEvidence(t *testing.T) {
	repairTestDB(t)
	const email = "order@example.test"
	seedRepairInbound(t, 41051, model.AWG, clientJSON(t, email, live()))
	seedRepairInbound(t, 41052, model.VLESS, clientJSON(t, email, copied()))
	seedRepairRecord(t, email, copied())

	if err := stripTunnelFieldsFromKeylessInbounds(); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if err := repairClobberedTunnelFields(); err != nil {
		t.Fatalf("repair: %v", err)
	}

	if got := reloadRecord(t, email); got.PublicKey != copyTunnelPub {
		t.Fatalf("this test only means something while the wrong order leaves the record broken; got %q", got.PublicKey)
	}
}

// Driving InitDB a second time runs the seeders in their production order, so
// this one fails if the two are ever swapped in runSeeders.
func TestRepairClobberedTunnelFields_RunsInOrderFromInitDB(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "x-ui.db")
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = CloseDB()
		}
	})

	const email = "seeded@example.test"
	seedRepairInbound(t, 41061, model.AWG, clientJSON(t, email, live()))
	seedRepairInbound(t, 41062, model.VLESS, clientJSON(t, email, copied()))
	seedRepairRecord(t, email, copied())
	if err := db.Where("seeder_name = ?", "RepairClobberedTunnelFields").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder row: %v", err)
	}
	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB: %v", err)
	}
	closed = true

	if err := InitDB(dbPath); err != nil {
		t.Fatalf("second InitDB: %v", err)
	}
	closed = false

	if got := reloadRecord(t, email); got.PublicKey != liveTunnelPub {
		t.Fatalf("startup left the record broken: %q — the drain ran before the repair", got.PublicKey)
	}
}
