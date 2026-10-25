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

func wgoFixtureInbound(settings string) *model.Inbound {
	return &model.Inbound{Id: 3, Port: 51820, Protocol: model.WireGuard, Settings: settings}
}

const wgoFixtureSettings = `{"kernel":true,"secretKey":"k","address":"10.8.0.1/24","tproxyPort":40000,
"obfuscation":{"enabled":true,"blockDirectPort":false,"presets":[
 {"id":"p1","name":"Phobos RU","extPort":51822,"key":"k1","masking":"STUN",
  "maxDummy":4,"idleTimeout":300,"clientWgLocalPort":13255,"isDefault":true},
 {"id":"p2","name":"media","extPort":51823,"key":"k2","masking":"MEDIA",
  "mediaSsrc":"305419896","mediaPt":100,"mediaClock":30,
  "maxDummy":45,"idleTimeout":300,"clientWgLocalPort":13256,"isDefault":false}
]}}`

func wgoFixture() WgoObfuscation {
	o, ok := ParseObfuscation(wgoFixtureInbound(wgoFixtureSettings))
	if !ok {
		panic("fixture must parse")
	}
	return o
}

func TestParseObfuscation(t *testing.T) {
	if _, ok := ParseObfuscation(wgoFixtureInbound(`{"kernel":true}`)); ok {
		t.Fatal("absent obfuscation key must parse as off")
	}
	if _, ok := ParseObfuscation(wgoFixtureInbound(`{broken`)); ok {
		t.Fatal("malformed settings must parse as off, not error (Rule 0b)")
	}
	if _, ok := ParseObfuscation(nil); ok {
		t.Fatal("nil inbound must parse as off")
	}
	o := wgoFixture()
	if !o.Enabled || o.BlockDirectPort || len(o.Presets) != 2 {
		t.Fatalf("fixture = %+v", o)
	}
	p := o.Presets[0]
	if p.Name != "Phobos RU" || p.ExtPort != 51822 || p.Key != "k1" || p.Masking != WgoMaskStun ||
		p.MaxDummy != 4 || p.IdleTimeout != 300 || p.ClientWgLocalPort != 13255 || !p.IsDefault {
		t.Fatalf("preset = %+v", p)
	}
	m := o.Presets[1]
	if m.Masking != WgoMaskMedia || m.MediaSsrc != "305419896" || m.MediaPt != 100 || m.MediaClock != 30 {
		t.Fatalf("media preset = %+v", m)
	}
}

func TestValidateObfuscation(t *testing.T) {
	valid := wgoFixture()
	if err := ValidateObfuscation(valid, 51820, nil); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	if err := ValidateObfuscation(WgoObfuscation{Enabled: false}, 51820, nil); err != nil {
		t.Fatalf("disabled block must pass: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*WgoObfuscation)
	}{
		{"no presets", func(o *WgoObfuscation) { o.Presets = nil }},
		{"empty name", func(o *WgoObfuscation) { o.Presets[0].Name = " " }},
		{"port zero", func(o *WgoObfuscation) { o.Presets[0].ExtPort = 0 }},
		{"port equals wg", func(o *WgoObfuscation) { o.Presets[0].ExtPort = 51820 }},
		{"dup port", func(o *WgoObfuscation) { o.Presets[1].ExtPort = 51822 }},
		{"dup section name", func(o *WgoObfuscation) { o.Presets[1].Name = "phobos ru" }},
		{"empty key", func(o *WgoObfuscation) { o.Presets[0].Key = "" }},
		{"long key", func(o *WgoObfuscation) { o.Presets[0].Key = strings.Repeat("x", 256) }},
		{"tls masking", func(o *WgoObfuscation) { o.Presets[0].Masking = WgoMaskTLS }},
		{"unknown masking", func(o *WgoObfuscation) { o.Presets[0].Masking = "RTP" }},
		{"bad dummy", func(o *WgoObfuscation) { o.Presets[0].MaxDummy = 1025 }},
		{"bad idle", func(o *WgoObfuscation) { o.Presets[0].IdleTimeout = 1 }},
		{"bad local port", func(o *WgoObfuscation) { o.Presets[0].ClientWgLocalPort = 0 }},
		{"bad media pt", func(o *WgoObfuscation) { o.Presets[1].MediaPt = 95 }},
		{"bad media clock", func(o *WgoObfuscation) { o.Presets[1].MediaClock = 90001 }},
		{"two defaults", func(o *WgoObfuscation) { o.Presets[1].IsDefault = true }},
		{"no default", func(o *WgoObfuscation) { o.Presets[0].IsDefault = false }},
		{"reserved port", func(o *WgoObfuscation) {}},
	}
	for _, tc := range cases {
		o := wgoFixture()
		tc.mutate(&o)
		reserved := func(port int, proto string) bool { return tc.name == "reserved port" }
		if err := ValidateObfuscation(o, 51820, reserved); err == nil {
			t.Fatalf("%s: expected error", tc.name)
		} else if tc.name == "reserved port" && !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("reserved port: unexpected error %v", err)
		}
	}
}

func TestSectionName(t *testing.T) {
	cases := map[string]string{
		"Phobos RU":   "phobos-ru",
		"подмосковье": "preset1",
		"A/B_C":       "a-b-c",
		"  x  ":       "x",
	}
	for in, want := range cases {
		if got := sectionName(WgoPreset{Name: in}, 0); got != want {
			t.Fatalf("sectionName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := sectionName(WgoPreset{Name: "###"}, 4); got != "preset5" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestRenderWgoConfGolden(t *testing.T) {
	got := RenderWgoConf(wgoFixture(), 51820)
	want := `[media]
mode = wireguard
source-lport = 51823
target = 127.0.0.1:51820
key = k2
masking = MEDIA
media-ssrc = 305419896
media-pt = 100
media-clock = 30
max-dummy = 45
idle-timeout = 300000
verbose = 1

[phobos-ru]
mode = wireguard
source-lport = 51822
target = 127.0.0.1:51820
key = k1
masking = STUN
max-dummy = 4
idle-timeout = 300000
verbose = 1

`
	if got != want {
		t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if RenderWgoConf(WgoObfuscation{}, 51820) != "" {
		t.Fatal("empty presets must render empty conf")
	}
}

func TestInstanceSectionGolden(t *testing.T) {
	p := wgoFixture().Presets[0]
	got := InstanceSection(p, "vpn.example.com")
	want := `[instance]
source-if = 127.0.0.1
source-lport = 13255
target = vpn.example.com:51822
key = k1
masking = STUN
verbose = INFO
idle-timeout = 300
max-dummy = 4
`
	if got != want {
		t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	media := InstanceSection(wgoFixture().Presets[1], "1.2.3.4")
	if !strings.Contains(media, "media-ssrc = 305419896") {
		t.Fatalf("media section missing ssrc:\n%s", media)
	}
}

func TestPhobosLink(t *testing.T) {
	link := PhobosLink("x", "Клиент 1")
	want := "phobos://eA#%D0%9A%D0%BB%D0%B8%D0%B5%D0%BD%D1%82%201"
	if link != want {
		t.Fatalf("link = %q, want %q", link, want)
	}
	long := PhobosLink(strings.Repeat("a", 100), "bob&co ?")
	if strings.Contains(long, "=") {
		t.Fatal("base64url must be unpadded")
	}
	if !strings.HasSuffix(long, "#bob%26co%20%3F") {
		t.Fatalf("fragment encoding wrong: %q", long)
	}
}

func TestWgoKey(t *testing.T) {
	if WgoKey(7) != "wgo-7" {
		t.Fatalf("WgoKey = %q", WgoKey(7))
	}
}

func TestWgoCoreRegistration(t *testing.T) {
	if !Wgo.Valid() || !Wgosocks.Valid() {
		t.Fatal("wgo/wgosocks must be valid cores")
	}
	if Wgo.DisplayName() != "WG obfuscator" || Wgosocks.DisplayName() != "WG obfuscator SOCKS5" {
		t.Fatalf("display names: %q / %q", Wgo.DisplayName(), Wgosocks.DisplayName())
	}
	if Wgo.BinaryName() != Wgosocks.BinaryName() {
		t.Fatalf("cores must share one binary: %q vs %q", Wgo.BinaryName(), Wgosocks.BinaryName())
	}
	if !strings.Contains(Wgo.BinaryName(), "wgobfs-") {
		t.Fatalf("binary name = %q", Wgo.BinaryName())
	}
	found := false
	for _, n := range All() {
		if n == Wgosocks {
			found = true
		}
	}
	if !found {
		t.Fatal("Wgosocks missing from All()")
	}
	if WgoKey(3) != "wgo-3" || WgosocksKey(9) != "wgosocks-9" {
		t.Fatal("manager keys wrong")
	}
}
