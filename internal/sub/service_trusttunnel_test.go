// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetSubs_TrustTunnel_TLVPlusURILines locks the one-link-per-transport
// TrustTunnel subscription output: only spec TLV deep links go out. Every
// current client (official TrustTunnel app, Exclave, husi, Throne) parses
// tt://?TLV; the Throne authority URI threw inside the sing-based Android
// parsers (base64url-decoding "user:pass@host" fails) and duplicated the
// profile in Throne itself.
func TestGetSubs_TrustTunnel_TLVPlusURILines(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const subId = "sub-tt"
	const email = "tt@example.com"
	const uuid = "a1b9265f-26a8-4b75-9be2-c64a94b15de1"

	db := database.GetDB()
	ib := &model.Inbound{
		UserId:   1,
		Tag:      "tt-in",
		Enable:   true,
		Port:     8443,
		Protocol: model.TrustTunnel,
		Settings: `{"hostname":"tt.example.com","listen":"0.0.0.0:8443","upstreamProtocol":"http2",` +
			`"clientRandomPrefix":"aabbccdd/ffffffff",` +
			`"clients":[{"email":"` + email + `","enable":true}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, UUID: uuid, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	s := NewSubService("")
	links, _, _, _, err := s.GetSubs(subId, "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	var lines []string
	for _, l := range links {
		lines = append(lines, splitLinkLines(l)...)
	}
	if len(lines) != 1 {
		t.Fatalf("TrustTunnel http2 sub must emit a single TLV link, got %d: %q", len(lines), lines)
	}

	if !strings.HasPrefix(lines[0], "tt://?") {
		t.Fatalf("line must be the TLV deep link, got %q", lines[0])
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(lines[0], "tt://?"))
	if err != nil {
		t.Fatalf("TLV deep link must be base64url: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("TLV payload empty")
	}
	// Hostname rides in TLV 0x01; dial address = resolved share host.
	if !bytes.Contains(payload, []byte("tt.example.com")) {
		t.Errorf("TLV must carry the inbound hostname, got %q", payload)
	}
}
