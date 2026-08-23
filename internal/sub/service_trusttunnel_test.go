// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetSubs_TrustTunnel_TLVPlusURILines locks the two-line TrustTunnel
// subscription output: the official TLV deep link (the only form Exclave and
// the official TrustTunnel app parse) followed by the Throne-compatible URI
// (the form Throne and NekoBox+ parse). A URI-only subscription left Exclave
// without TrustTunnel entirely (tester report): parseTrustTunnel base64url-
// decodes everything after tt:// and drops the line when it is not TLV.
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
	if len(lines) != 2 {
		t.Fatalf("TrustTunnel sub must emit TLV + Throne URI lines, got %d: %q", len(lines), lines)
	}

	if !strings.HasPrefix(lines[0], "tt://?") {
		t.Fatalf("first line must be the TLV deep link, got %q", lines[0])
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(lines[0], "tt://?"))
	if err != nil {
		t.Fatalf("TLV deep link must be base64url: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("TLV payload empty")
	}

	if strings.HasPrefix(lines[1], "tt://?") || !strings.HasPrefix(lines[1], "tt://") {
		t.Fatalf("second line must be the Throne URI, got %q", lines[1])
	}
	for _, want := range []string{":8443", "security=tls", "sni=tt.example.com", "alpn=h2", "client_random_prefix=aabbccdd/ffffffff"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("Throne URI missing %q, got %q", want, lines[1])
		}
	}
	// Dial address resolves through resolveInboundAddress (request host here);
	// the inbound hostname rides in sni, not in the authority.
	if !strings.Contains(lines[1], "@sub.example.com:8443") {
		t.Errorf("Throne URI must dial the resolved share host, got %q", lines[1])
	}
}
