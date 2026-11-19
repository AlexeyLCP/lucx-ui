// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Each subscriber gets an AnyTLS link with their own password — the one the
// server's users file holds — once the binary supports it; before that, and
// for clients the server doesn't list, the shared password.
func TestGetSubs_Anytls_PerUserPassword(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	ib := &model.Inbound{
		UserId: 1, Tag: "anytls-in", Enable: true, Port: 443, Remark: "Anytls", Protocol: model.Anytls,
		Settings: `{"password":"shared-pass","sni":"a.example.com",` +
			`"clients":[{"email":"u1@x","enable":true},{"email":"u2@x","enable":true}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ email, sub string }{{"u1@x", "s1"}, {"u2@x", "s2"}} {
		rec := &model.ClientRecord{Email: c.email, SubID: c.sub, Enable: true}
		if err := db.Create(rec).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatal(err)
		}
	}
	link := func(subID string) string {
		links, _, _, _, err := NewSubService("").GetSubs(subID, "sub.example.com")
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(links, "\n")
	}

	restore := tunnel.SetAnytlsCapsForTest(false, false)
	if got := link("s1"); !strings.HasPrefix(got, "anytls://shared-pass@") {
		t.Fatalf("old binary: want the shared password, got %q", got)
	}
	restore()

	defer tunnel.SetAnytlsCapsForTest(true, true)()
	secret, err := (&service.SettingService{}).GetSecret()
	if err != nil || len(secret) == 0 {
		t.Fatalf("secret: %v", err)
	}
	users := tunnel.AnytlsUsers(secret, ib)
	if len(users) != 2 {
		t.Fatalf("server users = %+v", users)
	}
	l1, l2 := link("s1"), link("s2")
	if !strings.HasPrefix(l1, "anytls://"+users[0].Password+"@") || !strings.HasPrefix(l2, "anytls://"+users[1].Password+"@") {
		t.Fatalf("links don't carry the server's per-user passwords:\n%s\n%s", l1, l2)
	}
	if strings.Contains(l1+l2, "shared-pass") || l1 == l2 {
		t.Fatalf("per-user links leak or share the password:\n%s\n%s", l1, l2)
	}
	if !strings.Contains(l1, "sni=a.example.com") {
		t.Fatalf("link lost its sni: %s", l1)
	}
}

// A node-hosted AnyTLS inbound gets per-user links too — but only when the
// node's learned panelVersion proves its sidecar takes a users file (lucx.342
// and later). Older or unknown node versions stay on the shared password,
// which such an older binary accepts and nothing else.
func TestGetSubs_Anytls_NodePanelVersionGate(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	node := &model.Node{Name: "de-fra-1", Address: "node1.example.com", Port: 2053, PanelVersion: "v3.11.0-lucx.368"}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	seed := "0123456789abcdef0123456789abcdef"
	ib := &model.Inbound{
		UserId: 1, Tag: "anytls-node-in", Enable: true, Port: 9443, Remark: "Anytls node", Protocol: model.Anytls,
		NodeID: &node.Id,
		Settings: `{"password":"shared-pass","sni":"a.example.com","authSeed":"` + seed + `",` +
			`"clients":[{"email":"nu1@x","enable":true}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{Email: "nu1@x", SubID: "sn1", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatal(err)
	}
	link := func() string {
		links, _, _, _, err := NewSubService("").GetSubs("sn1", "sub.example.com")
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(links, "\n")
	}

	defer tunnel.SetAnytlsCapsForTest(true, true)()
	secret, err := (&service.SettingService{}).GetSecret()
	if err != nil || len(secret) == 0 {
		t.Fatalf("secret: %v", err)
	}
	users := tunnel.AnytlsUsers(secret, ib)
	if len(users) != 1 {
		t.Fatalf("server users = %+v", users)
	}

	if got := link(); !strings.HasPrefix(got, "anytls://"+users[0].Password+"@") {
		t.Fatalf("node %q: want the derived password, got %q", node.PanelVersion, got)
	}

	for _, stale := range []string{"v3.10.2-lucx.341", "v3.x.x", ""} {
		if err := db.Model(&model.Node{}).Where("id = ?", node.Id).Update("panel_version", stale).Error; err != nil {
			t.Fatal(err)
		}
		if got := link(); !strings.HasPrefix(got, "anytls://shared-pass@") {
			t.Fatalf("node panelVersion %q: want the shared password, got %q", stale, got)
		}
	}
}

func TestParseLucxRelease(t *testing.T) {
	cases := map[string]int{
		"v3.11.0-lucx.368":            368,
		"3.11.0-lucx.368+dev+1d1128cf": 368,
		"V3.11.0-Lucx.342":            342,
		"v3.10.2-lucx.341":            341,
		"lucx.1":                      1,
		"v3.8.5":                      0,
		"lucx.x":                      0,
		"lucx.":                       0,
		"":                            0,
	}
	for version, want := range cases {
		if got := parseLucxRelease(version); got != want {
			t.Fatalf("parseLucxRelease(%q) = %d, want %d", version, got, want)
		}
	}
}
