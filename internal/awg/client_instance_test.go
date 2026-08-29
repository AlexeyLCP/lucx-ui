// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientInstanceFromOutbound(t *testing.T) {
	cases := []struct {
		name     string
		settings string
		wantOk   bool
		wantMTU  int // 0 = use the 1420 default fallback (see below)
	}{
		{
			name:     "valid",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up.example.com:51820","keepalive":25,"mtu":1320}`,
			wantOk:   true,
			wantMTU:  1320, // explicit in the JSON — must NOT be overridden by the default
		},
		{
			name:     "missing address",
			settings: `{"privateKey":"k","publicKey":"pub","endpoint":"up.example.com:51820"}`,
			wantOk:   false,
		},
		{
			name:     "missing publicKey",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","endpoint":"up.example.com:51820"}`,
			wantOk:   false,
		},
		{
			name:     "missing endpoint",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub"}`,
			wantOk:   false,
		},
		{
			name:     "empty settings",
			settings: ``,
			wantOk:   false,
		},
		{
			name:     "malformed json",
			settings: `{broken`,
			wantOk:   false,
		},
		{
			name:     "defaults applied (mtu, allowedIPs)",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up.example.com:51820"}`,
			wantOk:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &model.AwgOutbound{Id: 7, Tag: "awgo-7", Settings: tc.settings}
			ci, ok := ClientInstanceFromOutbound(o)
			if ok != tc.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOk)
			}
			if !ok {
				return
			}
			if ci.Ifname != "awgo-7" {
				t.Errorf("Ifname = %q, want awgo-7", ci.Ifname)
			}
			if ci.Id != 7 {
				t.Errorf("Id = %d, want 7", ci.Id)
			}
			wantMTU := tc.wantMTU
			if wantMTU == 0 {
				wantMTU = 1420
			}
			if ci.Settings.MTU != wantMTU {
				t.Errorf("MTU = %d, want %d", ci.Settings.MTU, wantMTU)
			}
			if ci.Settings.AllowedIPs != "0.0.0.0/0, ::/0" {
				t.Errorf("AllowedIPs default = %q, want 0.0.0.0/0, ::/0", ci.Settings.AllowedIPs)
			}
		})
	}
}

func TestClientInstanceFingerprint_Stable(t *testing.T) {
	o := &model.AwgOutbound{Id: 3, Tag: "awgo-3", Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320,"keepalive":25}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	ci2, _ := ClientInstanceFromOutbound(o)
	if renderClientConf(ci1) != renderClientConf(ci2) {
		t.Error("fingerprint not stable for same input")
	}
}

// renderClientConf deliberately never writes DNS (resolvconf takes the host's
// resolver down), so a DNS edit must not tear the outbound tunnel down either.
func TestClientInstanceFingerprint_StableOnDNS(t *testing.T) {
	o := &model.AwgOutbound{Id: 3, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320,"dns":"1.1.1.1"}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	o.Settings = `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320,"dns":"8.8.8.8"}`
	ci2, _ := ClientInstanceFromOutbound(o)
	if renderClientConf(ci1) != renderClientConf(ci2) {
		t.Fatal("DNS never reaches the client .conf: editing it must not restart the interface")
	}
}

// Same host-capability blindness as the inbound side: renderClientConf gates
// HPK, the device timers and the 3.1 flags on the module, the fingerprint did not.
func TestClientInstanceFingerprint_ReflectsModuleCapabilities(t *testing.T) {
	o := &model.AwgOutbound{Id: 3, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820",` +
		`"mtu":1320,"awgVersion":"3.1","jc":4,"jmin":50,"jmax":100,"s1":30,"s2":60,"s3":20,"s4":10,` +
		`"h1":"1","h2":"2","h3":"3","h4":"4","headerProtectionKey":"aBcD...base64hpk==",` +
		`"rekeyAfterTime":"120","randomTrailers":true}`}
	ci, ok := ClientInstanceFromOutbound(o)
	if !ok {
		t.Fatal("outbound must parse")
	}
	for _, tc := range []struct {
		name string
		flip func(*bool)
	}{
		{"awg3", SetModuleSupportsAwg3},
		{"awg31", SetModuleSupportsAwg31},
	} {
		t.Run(tc.name, func(t *testing.T) {
			yes, no := true, false
			SetModuleSupportsAwg3(&yes)
			SetModuleSupportsAwg31(&yes)
			t.Cleanup(func() { SetModuleSupportsAwg3(nil); SetModuleSupportsAwg31(nil) })
			supported := renderClientConf(ci)
			tc.flip(&no)
			if renderClientConf(ci) == supported {
				t.Fatalf("%s support is invisible to the client fingerprint: a module upgrade changes the .conf without restarting the interface", tc.name)
			}
		})
	}
}

// The client renderer has no syncconf path — the whole file, peer included, is
// what a restart applies, so its render must be byte-stable too.
func TestRenderClientConf_Deterministic(t *testing.T) {
	yes := true
	SetModuleSupportsAwg3(&yes)
	SetModuleSupportsAwg31(&yes)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil); SetModuleSupportsAwg31(nil) })
	o := &model.AwgOutbound{Id: 3, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","psk":"s",` +
		`"endpoint":"up:51820","keepalive":"15-25","mtu":1320,"dns":"1.1.1.1","awgVersion":"3.1",` +
		`"jc":4,"jmin":50,"jmax":100,"s1":30,"s2":60,"s3":20,"s4":10,` +
		`"h1":"1","h2":"2","h3":"3","h4":"4","i1":"<b 0xaa>","i2":"<b 0xbb>",` +
		`"headerProtectionKey":"aBcD...base64hpk==","contentPaddingAddition":"16","rekeyAfterTime":"120-180",` +
		`"randomTrailers":true,"disableCookies":true}`}
	ci, ok := ClientInstanceFromOutbound(o)
	if !ok {
		t.Fatal("outbound must parse")
	}
	want := renderClientConf(ci)
	for i := 0; i < 20; i++ {
		if got := renderClientConf(ci); got != want {
			t.Fatalf("render %d differs from the first:\n%s\n---\n%s", i, want, got)
		}
	}
}

func TestClientInstanceFingerprint_ChangesOnEdit(t *testing.T) {
	o := &model.AwgOutbound{Id: 3, Tag: "awgo-3", Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	o.Settings = `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`
	ci2, _ := ClientInstanceFromOutbound(o)
	if renderClientConf(ci1) == renderClientConf(ci2) {
		t.Error("fingerprint did not change when Address changed")
	}
}
