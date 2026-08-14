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
	if ci1.fingerprint() != ci2.fingerprint() {
		t.Error("fingerprint not stable for same input")
	}
}

func TestClientInstanceFingerprint_ChangesOnEdit(t *testing.T) {
	o := &model.AwgOutbound{Id: 3, Tag: "awgo-3", Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	o.Settings = `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`
	ci2, _ := ClientInstanceFromOutbound(o)
	if ci1.fingerprint() == ci2.fingerprint() {
		t.Error("fingerprint did not change when Address changed")
	}
}
