// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const wgosocksFixtureSettings = `{"key":"sk","masking":"MEDIA","mediaSsrc":"305419896",
"maxDummy":4,"idleTimeout":300,"maxClients":1024,
"clients":[
 {"email":"u1@example.com","login":"alice","password":"pw1"},
 {"email":"u2@example.com","login":"bob","password":"pw2"},
 {"email":"disabled@example.com","login":"eve","password":"pw3","enable":false}
]}`

func wgosocksFixtureInbound() *model.Inbound {
	return &model.Inbound{Id: 9, Port: 51862, Protocol: model.Wgosocks, Enable: true, Settings: wgosocksFixtureSettings}
}

func TestWgosocksConfigFromInbound(t *testing.T) {
	if _, ok := WgosocksConfigFromInbound(nil); ok {
		t.Fatal("nil inbound must not parse")
	}
	other := wgosocksFixtureInbound()
	other.Protocol = model.Naive
	if _, ok := WgosocksConfigFromInbound(other); ok {
		t.Fatal("wrong protocol must not parse")
	}
	broken := wgosocksFixtureInbound()
	broken.Settings = `{`
	if _, ok := WgosocksConfigFromInbound(broken); ok {
		t.Fatal("malformed settings must not parse")
	}
	s, ok := WgosocksConfigFromInbound(wgosocksFixtureInbound())
	if !ok || s.Key != "sk" || s.Masking != WgoMaskMedia || len(s.Clients) != 3 {
		t.Fatalf("fixture = %+v ok=%v", s, ok)
	}
}

func TestWgosocksValidate(t *testing.T) {
	s, _ := WgosocksConfigFromInbound(wgosocksFixtureInbound())
	if err := ValidateWgosocks(s, 51862, nil); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	cases := []struct {
		name string
		mut  func(*WgosocksSettings)
	}{
		{"empty key", func(s *WgosocksSettings) { s.Key = "" }},
		{"long key", func(s *WgosocksSettings) { s.Key = strings.Repeat("x", 256) }},
		{"unknown masking", func(s *WgosocksSettings) { s.Masking = "RTP" }},
		{"bad dummy", func(s *WgosocksSettings) { s.MaxDummy = 2000 }},
		{"bad idle", func(s *WgosocksSettings) { s.IdleTimeout = 1 }},
		{"bad maxclients", func(s *WgosocksSettings) { s.MaxClients = 0 }},
		{"bad media pt", func(s *WgosocksSettings) { s.MediaPt = 90 }},
		{"bad media clock", func(s *WgosocksSettings) { s.MediaClock = 100000 }},
		{"empty email", func(s *WgosocksSettings) { s.Clients[0].Email = " " }},
		{"dup email", func(s *WgosocksSettings) { s.Clients[1].Email = "u1@example.com" }},
		{"empty login", func(s *WgosocksSettings) { s.Clients[0].Login = "" }},
		{"padded login", func(s *WgosocksSettings) { s.Clients[0].Login = " alice" }},
		{"empty password", func(s *WgosocksSettings) { s.Clients[0].Password = "" }},
		{"dup login", func(s *WgosocksSettings) { s.Clients[1].Login = "alice" }},
	}
	for _, tc := range cases {
		s, _ := WgosocksConfigFromInbound(wgosocksFixtureInbound())
		tc.mut(&s)
		if err := ValidateWgosocks(s, 51862, nil); err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
	}
	// cap
	s, _ = WgosocksConfigFromInbound(wgosocksFixtureInbound())
	for i := len(s.Clients); i <= wgosocksClientCap; i++ {
		s.Clients = append(s.Clients, WgosocksClient{Email: "e@x", Login: "l", Password: "p"})
	}
	// note: dup logins would trigger first — make them unique
	for i := range s.Clients {
		s.Clients[i].Email = "e@x" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		s.Clients[i].Login = "l" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}
	s.Clients = append(s.Clients, WgosocksClient{Email: "over@x", Login: "over", Password: "p"})
	if err := ValidateWgosocks(s, 51862, nil); err == nil || !strings.Contains(err.Error(), "clients") {
		t.Fatalf("cap: %v", err)
	}
	if err := ValidateWgosocks(s, 51862, func(port int, proto string) bool { return true }); err == nil {
		t.Fatal("reserved port must be rejected")
	}
	if err := ValidateWgosocks(s, 0, nil); err == nil {
		t.Fatal("port 0 must be rejected")
	}
	// TLS masking is valid here (socks5 mode), unlike WG presets.
	tls := s
	tls.Clients = nil
	tls.Masking = WgoMaskTLS
	if err := ValidateWgosocks(tls, 51862, nil); err != nil {
		t.Fatalf("TLS masking must be valid for socks5: %v", err)
	}
}

func TestRenderWgosocksConfGolden(t *testing.T) {
	s, _ := WgosocksConfigFromInbound(wgosocksFixtureInbound())
	got := RenderWgosocksConf(s, 51862)
	want := `[wgosocks]
mode = socks5
role = server
source-lport = 51862
key = sk
masking = MEDIA
media-ssrc = 305419896
max-dummy = 4
idle-timeout = 300000
verbose = 1
socks5-users = alice:pw1,bob:pw2
`
	if got != want {
		t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestWgosocksInstanceFromInbound(t *testing.T) {
	inst, ok := WgosocksInstanceFromInbound(wgosocksFixtureInbound())
	if !ok || inst.Core != Wgosocks || inst.Key != "wgosocks-9" || !inst.Enabled {
		t.Fatalf("instance = %+v ok=%v", inst, ok)
	}
	if inst.ProbePort != 51862 || !strings.Contains(inst.ConfigText, "source-lport = 51862") {
		t.Fatalf("probe/conf wrong: %+v", inst)
	}
	disabled := wgosocksFixtureInbound()
	disabled.Enable = false
	inst, ok = WgosocksInstanceFromInbound(disabled)
	if !ok || inst.Enabled {
		t.Fatalf("disabled instance = %+v ok=%v", inst, ok)
	}
	if _, ok := WgosocksInstanceFromInbound(nil); ok {
		t.Fatal("nil inbound must not produce an instance")
	}
}

func TestWgosocksClientConfGolden(t *testing.T) {
	s, _ := WgosocksConfigFromInbound(wgosocksFixtureInbound())
	got := WgosocksClientConf(s, 51862, "vpn.example.com", s.Clients[0])
	want := `[instance]
mode = socks5
role = client
source-if = 127.0.0.1
source-lport = 1080
target = vpn.example.com:51862
key = sk
masking = MEDIA
verbose = INFO
media-ssrc = 305419896

[socks5]
login = alice
password = pw1
`
	if got != want {
		t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
