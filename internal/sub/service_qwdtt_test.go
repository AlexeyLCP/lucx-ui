// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestGetSubs_Qwdtt_SingleConfigLine locks the SpaceNeuroX 1.4.2 import
// contract: one qwdtt://config? line, never a trailing wdtt://. parsePayload
// treats the whole clipboard as one URI, so a second line corrupts pass and
// the DTLS handshake dies (VladufQa, 24.08.2026).
func TestGetSubs_Qwdtt_SingleConfigLine(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const subId = "sub-qwdtt"
	const email = "qwdtt@example.com"

	db := database.GetDB()
	ib := &model.Inbound{
		UserId:   1,
		Tag:      "qwdtt-in",
		Enable:   true,
		Port:     56000,
		Protocol: model.Qwdtt,
		Settings: `{"listenAddr":"0.0.0.0:56000","wgPort":56001,"password":"secret","dns":"8.8.8.8",` +
			`"subHost":"1.2.3.4:56000","vkHashes":"h1,h2","workers":16,"clientPort":9000,"remark":"Home"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Enable: true}
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
		t.Fatalf("qWDTT sub must emit one qwdtt://config? line, got %d: %q", len(lines), lines)
	}
	got := lines[0]
	if !strings.HasPrefix(got, "qwdtt://config?") {
		t.Fatalf("want qwdtt://config?, got %q", got)
	}
	if strings.ContainsAny(got, "\r\n") || strings.HasPrefix(got, "wdtt://") {
		t.Fatalf("must not append legacy wdtt://, got %q", got)
	}
	for _, want := range []string{"name=Home", "peer=1.2.3.4%3A56000", "pass=secret", "hashes=h1%2Ch2"} {
		if !strings.Contains(got, want) {
			t.Errorf("URI missing %q: %s", want, got)
		}
	}
}
