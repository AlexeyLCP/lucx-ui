// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// deviceFP is what ensureLocked compares: the device half of the rendered conf.
func deviceFP(inst Instance) string { return deviceFingerprint(renderServerConf(inst)) }

func TestInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Id:       7,
		Tag:      "inbound-awg-7",
		Listen:   "0.0.0.0",
		Port:     47010,
		Protocol: model.AWG,
		Settings: `{"privateKey":"yKb...priv","publicKey":"xKb...pub",` +
			`"mtu":1420,"dns":"1.1.1.1","obfLevel":3,"mimicryProfile":"quic",` +
			`"jc":8,"jmin":70,"jmax":200,"s1":30,"s2":60,"s3":20,"s4":10,` +
			`"h1":"100000-500000","h2":"600000-900000",` +
			`"h3":"1000000-1500000","h4":"1600000-2000000",` +
			`"i1":"<b 0xaa>","i2":"<b 0xbb>","i3":"<b 0xcc>","i4":"<b 0xdd>","i5":"<b 0xee>",` +
			`"headerProtectionKey":"aBcD...base64hpk==",` +
			`"awgVersion":"3",` +
			`"routeThroughXray":true,"outboundTag":"warp",` +
			`"clients":[{"id":"peer-pub-1","password":"psk-1","enable":true},` +
			`{"id":"peer-pub-2","password":"psk-2","enable":false},` +
			`{"id":"","password":"psk-3","enable":true}]}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Id != 7 || inst.Tag != "inbound-awg-7" || inst.Port != 47010 {
		t.Fatalf("bad identity: %+v", inst)
	}
	if inst.Ifname != "awg7" {
		t.Fatalf("expected ifname awg7, got %s", inst.Ifname)
	}
	if inst.MTU != 1420 || inst.DNS != "1.1.1.1" {
		t.Fatalf("mtu/dns not parsed: %+v", inst)
	}
	if inst.Jc != 8 || inst.Jmin != 70 || inst.Jmax != 200 {
		t.Fatalf("jc/jmin/jmax not parsed: %+v", inst)
	}
	if inst.S1 != 30 || inst.S2 != 60 || inst.S3 != 20 || inst.S4 != 10 {
		t.Fatalf("s1-s4 not parsed: %+v", inst)
	}
	if inst.H1 != "100000-500000" || inst.H4 != "1600000-2000000" {
		t.Fatalf("h1/h4 not parsed: %+v", inst)
	}
	if inst.I1 != "<b 0xaa>" || inst.I5 != "<b 0xee>" {
		t.Fatalf("i1-i5 not parsed: %+v", inst)
	}
	if inst.HeaderProtectionKey != "aBcD...base64hpk==" {
		t.Fatalf("headerProtectionKey not parsed: %+v", inst)
	}
	if inst.AwgVersion != "3" {
		t.Fatalf("awgVersion not parsed: %+v", inst)
	}
	if !inst.RouteThroughXray || inst.OutboundTag != "warp" {
		t.Fatalf("routing not parsed: %+v", inst)
	}
	// Only enabled peers with non-empty id+psk should be desired.
	if len(inst.Peers) != 1 {
		t.Fatalf("expected 1 enabled peer, got %d", len(inst.Peers))
	}
	if inst.Peers[0].PublicKey != "peer-pub-1" || inst.Peers[0].PSK != "psk-1" {
		t.Fatalf("peer not parsed: %+v", inst.Peers[0])
	}
	// No keepAlive in settings → empty (off). Pre-lucx.75 defaulted 0→25.
	if !inst.Peers[0].Keepalive.IsZero() {
		t.Fatalf("expected keepalive empty/off, got %q", inst.Peers[0].Keepalive)
	}
}

func TestInstanceFromInbound_FormPasswordIsNotPSK(t *testing.T) {
	ib := &model.Inbound{
		Id:       32,
		Protocol: model.AWG,
		Settings: `{"privateKey":"yKb...priv","clients":[` +
			`{"publicKey":"peer-pub-real","password":"vgmg2ms952ceemgc","enable":true},` +
			`{"id":"legacy-pub","password":"legacy-psk","enable":true}]}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if len(inst.Peers) != 2 {
		t.Fatalf("peers = %d, want 2", len(inst.Peers))
	}
	if inst.Peers[0].PublicKey != "peer-pub-real" {
		t.Fatalf("modern pub = %q", inst.Peers[0].PublicKey)
	}
	if inst.Peers[0].PSK != "" {
		t.Fatalf("form password must not become PSK, got %q", inst.Peers[0].PSK)
	}
	if inst.Peers[1].PublicKey != "legacy-pub" || inst.Peers[1].PSK != "legacy-psk" {
		t.Fatalf("legacy id/password pair: %+v", inst.Peers[1])
	}
}

func TestInstanceFromInbound_RejectsNonAWG(t *testing.T) {
	ib := &model.Inbound{Protocol: model.VLESS, Settings: `{}`}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("expected false for non-AWG protocol")
	}
}

func TestInstanceFromInbound_RejectsMissingPrivateKey(t *testing.T) {
	ib := &model.Inbound{Protocol: model.AWG, Settings: `{"publicKey":"x"}`}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("expected false when privateKey is empty")
	}
}

func TestInstanceFromInbound_RejectsBadJSON(t *testing.T) {
	ib := &model.Inbound{Protocol: model.AWG, Settings: `not json`}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("expected false for malformed settings JSON")
	}
}

func TestInstanceFromInbound_NilInbound(t *testing.T) {
	if _, ok := InstanceFromInbound(nil); ok {
		t.Fatal("expected false for nil inbound")
	}
}

func TestInstanceFingerprint_StableForEqualInstances(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k",
		MTU: 1320, Jc: 5, Peers: []PeerSpec{{PublicKey: "p1", PSK: "psk", Keepalive: "25", AllowedIPs: "0.0.0.0/0, ::/0"}},
	}
	a := deviceFP(inst)
	b := deviceFP(inst)
	if a != b {
		t.Fatal("fingerprint must be deterministic for equal instances")
	}
}

// Address must be set and both routing modes covered: PostUp/PostDown are the
// only device lines peer data could ever leak into, and no Address means no PostUp.
func TestInstanceFingerprint_StableOnPeerMutation(t *testing.T) {
	for _, routed := range []bool{false, true} {
		t.Run("routeThroughXray="+strconv.FormatBool(routed), func(t *testing.T) {
			inst := Instance{
				Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k",
				Address: "10.8.0.1/24", RouteThroughXray: routed,
				Peers: []PeerSpec{{PublicKey: "p1", PSK: "psk", AllowedIPs: "10.8.0.2/32"}},
			}
			before := deviceFP(inst)
			if routed && !strings.Contains(before, "PostUp") {
				t.Fatal("PostUp missing from the device half: this test would guard nothing")
			}
			peerBefore := inst.peerFingerprint()
			inst.Peers = append(inst.Peers, PeerSpec{PublicKey: "p2", PSK: "psk2", AllowedIPs: "10.8.0.3/32"})
			if deviceFP(inst) != before {
				t.Fatal("device fingerprint must NOT change when a peer is added (syncconf, not restart)")
			}
			if inst.peerFingerprint() == peerBefore {
				t.Fatal("peer fingerprint must change when a peer is added")
			}
			peerBefore = inst.peerFingerprint()
			inst.Peers[0].PSK = "psk1-rotated"
			inst.Peers[0].AllowedIPs = "10.8.0.9/32"
			if deviceFP(inst) != before {
				t.Fatal("device fingerprint must NOT change when an existing peer is edited")
			}
			if inst.peerFingerprint() == peerBefore {
				t.Fatal("peer fingerprint must change when an existing peer is edited")
			}
		})
	}
}

// DNS is the one field that really is client-export-only: it lands in the
// exported .conf and never in the server's, so it cannot need a restart.
func TestInstanceFingerprint_StableOnDNS(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", DNS: "1.1.1.1"}
	before := deviceFP(inst)
	inst.DNS = "8.8.8.8"
	if deviceFP(inst) != before {
		t.Fatal("DNS is client-export-only and must not restart the interface")
	}
}

func TestInstanceFingerprint_ChangesOnObfuscation(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", Jc: 5}
	before := deviceFP(inst)
	inst.Jc = 9
	after := deviceFP(inst)
	if before == after {
		t.Fatal("fingerprint must change when obfuscation (Jc) changes")
	}
}

// Address is what makes PostUp/PostDown render at all, and those two lines are
// where routeThroughXray reaches the file.
func TestInstanceFingerprint_ChangesOnRoutingToggle(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", Address: "10.8.0.1/24"}
	before := deviceFP(inst)
	inst.RouteThroughXray = true
	after := deviceFP(inst)
	if before == after {
		t.Fatal("fingerprint must change when routeThroughXray is toggled")
	}
}

// Without an Address neither routing mode renders PostUp, so the .conf is
// byte-identical and a restart would drop every client for nothing.
func TestInstanceFingerprint_StableOnRoutingToggleWithoutAddress(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k"}
	before := deviceFP(inst)
	inst.RouteThroughXray = true
	if deviceFP(inst) != before {
		t.Fatal("no Address means no PostUp either way: the fingerprint must not move")
	}
}

func TestInstanceFingerprint_ChangesOnHeaderProtectionKey(t *testing.T) {
	awg3 := true
	SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", AwgVersion: "3"}
	before := deviceFP(inst)
	inst.HeaderProtectionKey = "aBcD...base64hpk=="
	after := deviceFP(inst)
	if before == after {
		t.Fatal("fingerprint must change when HeaderProtectionKey is set (restart trigger)")
	}
}

func TestRenderServerConf_IncludesObfuscationAndPeers(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "server-priv",
		MTU: 1320, DNS: "1.1.1.1",
		Jc: 8, Jmin: 70, Jmax: 200, S1: 30, S2: 60, S3: 20, S4: 10,
		H1: "100000-500000", H2: "600000-900000", H3: "1000000-1500000", H4: "1600000-2000000",
		I1: "<b 0xaa>", I2: "<b 0xbb>", I3: "<b 0xcc>", I4: "<b 0xdd>", I5: "<b 0xee>",
		Peers: []PeerSpec{{PublicKey: "peer-pub", PSK: "peer-psk", Keepalive: "25", AllowedIPs: "0.0.0.0/0, ::/0"}},
	}
	conf := renderServerConf(inst)
	mustContain := []string{
		"[Interface]",
		"PrivateKey = server-priv",
		"ListenPort = 47000",
		"MTU = 1320",
		"Jc = 8", "Jmin = 70", "Jmax = 200",
		"S1 = 30", "S2 = 60", "S3 = 20", "S4 = 10",
		"H1 = 100000-500000", "H4 = 1600000-2000000",
		// I1-I5 are client-only — NOT in the server .conf (kernel module
		// rejects CPS tags in setconf). Server conf has Jc/S/H only.
		"[Peer]",
		"PublicKey = peer-pub",
		"PresharedKey = peer-psk",
		"AllowedIPs = 0.0.0.0/0, ::/0",
	}
	for _, s := range mustContain {
		if !strings.Contains(conf, s) {
			t.Errorf("renderServerConf missing %q\nConf:\n%s", s, conf)
		}
	}
	if strings.Contains(conf, "DNS =") {
		t.Errorf("DNS must never appear in server .conf, got:\n%s", conf)
	}
	if strings.Contains(conf, "PersistentKeepalive") {
		t.Errorf("PersistentKeepalive is client-only, must not appear in server .conf, got:\n%s", conf)
	}
}

func TestRenderServerConf_V15OmitsS3S4(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		AwgVersion: "1.5",
		Jc:         6, Jmin: 50, Jmax: 100, S1: 80, S2: 79, S3: 21, S4: 13,
		H1: "1", H2: "2", H3: "3", H4: "4",
	}
	conf := renderServerConf(inst)
	if !strings.Contains(conf, "S1 = 80") || !strings.Contains(conf, "S2 = 79") {
		t.Fatalf("v1.5 must keep S1/S2, got:\n%s", conf)
	}
	if strings.Contains(conf, "S3 =") || strings.Contains(conf, "S4 =") {
		t.Fatalf("v1.5 server conf must omit S3/S4 (client export strips them), got:\n%s", conf)
	}
}

// I1-I5 must be dropped for v1.5: those tools reject the tags with
// "Line unrecognized," and the client export already strips them too.
func TestRenderServerConf_NoIFieldsOnV15(t *testing.T) {
	for _, tc := range []struct {
		name       string
		awgVersion string
		wantI      bool
	}{
		{"1.5 drops I-fields", "1.5", false},
		{"2 keeps I-fields", "2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst := Instance{
				Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
				AwgVersion: tc.awgVersion,
				I1:         "aa", I2: "bb", I3: "cc", I4: "dd", I5: "ee",
			}
			conf := renderServerConf(inst)
			got := strings.Contains(conf, "I1 = aa")
			if got != tc.wantI {
				t.Fatalf("awgVersion %q: I1 present = %v, want %v\nConf:\n%s", tc.awgVersion, got, tc.wantI, conf)
			}
		})
	}
}

func TestRenderServerConf_OmitsCPSWhenEmpty(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "I1 =") {
		t.Errorf("CPS I1 must be omitted when empty, got:\n%s", conf)
	}
}

func TestRenderServerConf_OmitsPeerPersistentKeepalive(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 1, PrivateKey: "k", MTU: 1320,
		Peers: []PeerSpec{{PublicKey: "p", Keepalive: "15-25", AllowedIPs: "10.0.0.2/32"}},
	}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "PersistentKeepalive") {
		t.Fatalf("peer PersistentKeepalive is client-export only, got:\n%s", conf)
	}
}

func TestInstanceFromInbound_KeepAliveRange(t *testing.T) {
	ib := &model.Inbound{
		Id: 1, Protocol: model.AWG,
		Settings: `{"privateKey":"k","clients":[{"publicKey":"p","enable":true,"keepAlive":"15-25","allowedIPs":["10.0.0.2/32"]}]}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok || len(inst.Peers) != 1 {
		t.Fatalf("parse failed: ok=%v peers=%d", ok, len(inst.Peers))
	}
	if inst.Peers[0].Keepalive != "15-25" {
		t.Fatalf("Keepalive = %q, want 15-25", inst.Peers[0].Keepalive)
	}
}

func TestRenderServerConf_NeverWritesDNS(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320, DNS: "1.1.1.1, 1.0.0.1"}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "DNS =") {
		t.Errorf("DNS must never appear in server .conf even when set, got:\n%s", conf)
	}
}

// HeaderProtectionKey (AWG3) is version-gated in the server .conf: written
// only when AwgVersion == "3" AND the key is non-empty. The upstream kernel
// v3.0.20260731 + tools v3.0.20260730 parse the field; older builds reject it
// with "Line unrecognized", awg-quick rolls the interface back, and reconcile
// fails every 10s. Version-gating keeps v1/v2 inbounds working on any kernel,
// and lets a v3 inbound opt in once the AWG3 module is installed.
func TestRenderServerConf_HeaderProtectionKeyVersionGated(t *testing.T) {
	awg3 := true
	SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	for _, tc := range []struct {
		name    string
		version string
		hpk     string
		want    bool
	}{
		{"empty key v3", "3", "", false},
		{"set key v3", "3", "aBcD...base64hpk==", true},
		{"set key v3.1", "3.1", "aBcD...base64hpk==", true},
		{"set key v2", "2", "aBcD...base64hpk==", false},
		{"set key v1.5", "1.5", "aBcD...base64hpk==", false},
		{"set key no version", "", "aBcD...base64hpk==", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst := Instance{
				Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320, Jc: 5,
				AwgVersion:          tc.version,
				HeaderProtectionKey: tc.hpk,
			}
			conf := renderServerConf(inst)
			contains := strings.Contains(conf, "HeaderProtectionKey = "+tc.hpk)
			if contains != tc.want {
				t.Errorf("version=%q hpk-set=%v: want HeaderProtectionKey in conf=%v, got=%v\nConf:\n%s",
					tc.version, tc.hpk != "", tc.want, contains, conf)
			}
		})
	}
}

// On a host still running the v1.x amneziawg module, an inbound with
// awgVersion "3" and a non-empty HeaderProtectionKey must NOT emit the line
// — the v1 kernel rejects "Line unrecognized: HeaderProtectionKey=..." and
// awg-quick deletes the half-built interface. ModuleSupportsAwg3() is the
// last line of defense against that regression.
func TestRenderServerConf_HeaderProtectionKeyDroppedOnV1Module(t *testing.T) {
	v1 := false
	SetModuleSupportsAwg3(&v1)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320, Jc: 5,
		AwgVersion:          "3",
		HeaderProtectionKey: "aBcD...base64hpk==",
	}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "HeaderProtectionKey") {
		t.Errorf("v1.x module must drop HPK even when awgVersion=3, got:\n%s", conf)
	}
}

// Every version step adds fields the peer must match: 1.5→2 brings S3/S4,
// 2→3 the header protection key, 3→3.1 the device flags.
func TestInstanceFingerprint_ChangesOnAwgVersion(t *testing.T) {
	yes := true
	SetModuleSupportsAwg3(&yes)
	SetModuleSupportsAwg31(&yes)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil); SetModuleSupportsAwg31(nil) })
	base := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", S3: 20, S4: 10,
		HeaderProtectionKey: "aBcD...base64hpk==", RandomTrailers: true,
	}
	for _, tc := range []struct{ from, to string }{
		{"1.5", "2"}, {"2", "3"}, {"3", "3.1"},
	} {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			before, after := base, base
			before.AwgVersion, after.AwgVersion = tc.from, tc.to
			if deviceFP(before) == deviceFP(after) {
				t.Fatalf("%s -> %s adds fields to the .conf but does not restart the interface", tc.from, tc.to)
			}
		})
	}
}

func TestNormalizeAwgVersion(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "2"}, {"1.5", "1.5"}, {"2", "2"}, {"3", "3"}, {"3.1", "3.1"}, {"garbage", "2"}, {"4", "2"},
	} {
		if got := NormalizeAWGVersion(tc.in); got != tc.want {
			t.Errorf("NormalizeAWGVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsAwg3Plus(t *testing.T) {
	if !IsAwg3Plus("3") || !IsAwg3Plus("3.1") {
		t.Fatal("3 and 3.1 must be AWG3+")
	}
	if IsAwg3Plus("2") || IsAwg3Plus("1.5") || IsAwg3Plus("") {
		t.Fatal("v1/v2 must not be AWG3+")
	}
}

func TestParseAwgToolsVersion(t *testing.T) {
	maj, min := parseAwgToolsVersion("amneziawg-tools v3.1.20260812 - https://amnezia.org")
	if maj != 3 || min != 1 {
		t.Fatalf("v3.1 banner: got %d.%d", maj, min)
	}
	maj, min = parseAwgToolsVersion("amneziawg-tools v3.0.20260730 - https://amnezia.org")
	if maj != 3 || min != 0 {
		t.Fatalf("v3.0 banner: got %d.%d", maj, min)
	}
	if awgToolsAtLeast("amneziawg-tools v3.0.20260730", 3, 1) {
		t.Fatal("v3.0 must not satisfy ≥ 3.1")
	}
	if !awgToolsAtLeast("amneziawg-tools v3.1.20260812", 3, 1) {
		t.Fatal("v3.1 must satisfy ≥ 3.1")
	}
}

func TestRenderServerConf_Awg31FlagsVersionGated(t *testing.T) {
	awg3, awg31 := true, true
	SetModuleSupportsAwg3(&awg3)
	SetModuleSupportsAwg31(&awg31)
	t.Cleanup(func() {
		SetModuleSupportsAwg3(nil)
		SetModuleSupportsAwg31(nil)
	})
	for _, tc := range []struct {
		name                                         string
		version                                      string
		trailers, cookies, wantTrailers, wantCookies bool
	}{
		{"v3.1 both", "3.1", true, true, true, true},
		{"v3.1 trailers only", "3.1", true, false, true, false},
		{"v3 drops", "3", true, true, false, false},
		{"v2 drops", "2", true, true, false, false},
		{"v3.1 false omitted", "3.1", false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst := Instance{
				Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
				AwgVersion:     tc.version,
				RandomTrailers: tc.trailers,
				DisableCookies: tc.cookies,
			}
			conf := renderServerConf(inst)
			if got := strings.Contains(conf, "RandomTrailers = on"); got != tc.wantTrailers {
				t.Errorf("RandomTrailers: got %v want %v\n%s", got, tc.wantTrailers, conf)
			}
			if got := strings.Contains(conf, "DisableCookies = on"); got != tc.wantCookies {
				t.Errorf("DisableCookies: got %v want %v\n%s", got, tc.wantCookies, conf)
			}
		})
	}
}

func TestRenderServerConf_Awg31DroppedOnV30Tools(t *testing.T) {
	awg3, v30 := true, false
	SetModuleSupportsAwg3(&awg3)
	SetModuleSupportsAwg31(&v30)
	t.Cleanup(func() {
		SetModuleSupportsAwg3(nil)
		SetModuleSupportsAwg31(nil)
	})
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		AwgVersion: "3.1", RandomTrailers: true, DisableCookies: true,
		HeaderProtectionKey: "aBcD...base64hpk==",
	}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "RandomTrailers") || strings.Contains(conf, "DisableCookies") {
		t.Errorf("v3.0 tools must drop 3.1 flags, got:\n%s", conf)
	}
	if !strings.Contains(conf, "HeaderProtectionKey") {
		t.Errorf("v3.0 tools must still emit HPK for 3.1 inbound, got:\n%s", conf)
	}
}

func TestIfnameFor(t *testing.T) {
	if got := ifnameFor(1); got != "awg1" {
		t.Fatalf("ifnameFor(1) = %s", got)
	}
	if got := ifnameFor(42); got != "awg42" {
		t.Fatalf("ifnameFor(42) = %s", got)
	}
}

func TestClientSubnet(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"10.8.0.1/24", "10.8.0.0/24"},
		{"10.0.0.5/16", "10.0.0.0/16"},
		{"192.168.1.1/32", "192.168.1.1/32"},
		{"", ""},
		{"garbage", ""},
		{"10.8.0.1", ""},
	}
	for _, c := range cases {
		got := clientSubnet(c.in)
		if got != c.want {
			t.Errorf("clientSubnet(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAwgRouteTable(t *testing.T) {
	if got := awgRouteTable(1); got != 1001 {
		t.Fatalf("awgRouteTable(1) = %d, want 1001", got)
	}
	if got := awgRouteTable(42); got != 1042 {
		t.Fatalf("awgRouteTable(42) = %d, want 1042", got)
	}
}

// routeThroughXray steers client-originated traffic into the Xray TUN via an
// iif policy rule and a per-inbound table. The route itself (default dev tunN)
// is owned by the reconcile loop — tunN does not exist yet at PostUp time and
// is recreated on every Xray restart, so a one-shot PostUp cannot own it.
func TestRenderServerConf_RouteThroughXrayPolicyRouting(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", RouteThroughXray: true,
	}
	conf := renderServerConf(inst)
	mustContain := []string{
		"PostUp",
		"ip rule add iif awg1 lookup 1001",
		"ip_forward",
		"net.ipv4.conf.awg1.rp_filter=2",
		"FORWARD -i awg1 -j ACCEPT",
		"FORWARD -o awg1 -j ACCEPT",
		"FORWARD -i tun1 -j ACCEPT",
		"FORWARD -o tun1 -j ACCEPT",
	}
	for _, s := range mustContain {
		if !strings.Contains(conf, s) {
			t.Errorf("conf missing %q\nConf:\n%s", s, conf)
		}
	}
	mustNotContain := []string{
		"MASQUERADE",
		"ip route replace 10.8.0.0/24",
		"ip rule add from",
		"sleep",
	}
	for _, s := range mustNotContain {
		if strings.Contains(conf, s) {
			t.Errorf("conf must not contain %q\nConf:\n%s", s, conf)
		}
	}
	postDown := ""
	for _, line := range strings.Split(conf, "\n") {
		if strings.HasPrefix(line, "PostDown") {
			postDown = line
		}
	}
	for _, s := range []string{"ip rule del iif awg1 lookup 1001", "ip route flush table 1001"} {
		if !strings.Contains(postDown, s) {
			t.Errorf("PostDown missing %q, got %q", s, postDown)
		}
	}
}

func TestNatPostUpPostDown_RouteThroughXrayPerInbound(t *testing.T) {
	inst := Instance{
		Id: 7, Ifname: "awg7", Port: 47007, PrivateKey: "k", MTU: 1320,
		Address: "10.9.0.1/24", RouteThroughXray: true,
	}
	postUp, postDown := natPostUpPostDown(inst)
	for _, s := range []string{"iif awg7 lookup 1007", "FORWARD -i tun7"} {
		if !strings.Contains(postUp, s) {
			t.Errorf("PostUp missing %q, got %q", s, postUp)
		}
	}
	for _, s := range []string{"iif awg7 lookup 1007", "flush table 1007"} {
		if !strings.Contains(postDown, s) {
			t.Errorf("PostDown missing %q, got %q", s, postDown)
		}
	}
}

func TestEnsureXrayRoutingCmds(t *testing.T) {
	inst := Instance{Id: 3, Ifname: "awg3", RouteThroughXray: true}
	cmds := ensureXrayRoutingCmds(inst)
	want := []string{
		"ip route replace default dev tun3 table 1003",
		"sysctl -qw net.ipv4.conf.tun3.rp_filter=2",
	}
	if len(cmds) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(cmds), cmds)
	}
	for i, w := range want {
		if strings.Join(cmds[i], " ") != w {
			t.Errorf("cmd %d = %q, want %q", i, strings.Join(cmds[i], " "), w)
		}
	}
}

func TestRuleMissing(t *testing.T) {
	withRule := "32765:\tfrom all iif awg3 lookup 1003\n"
	if ruleMissing(withRule, 1003) {
		t.Error("rule present in output must not be reported missing")
	}
	if !ruleMissing("", 1003) {
		t.Error("empty output means the rule is missing")
	}
	if !ruleMissing("32765:\tfrom all iif awg3 lookup 100\n", 1003) {
		t.Error("a different table must not satisfy the check")
	}
}

func TestRenderServerConf_NoPostUpWhenNoAddress(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		RouteThroughXray: false,
	}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "PostUp") {
		t.Errorf("PostUp must be absent when Address is empty, got:\n%s", conf)
	}
}

func TestNatPostUpPostDown_ContainsMasquerade(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", RouteThroughXray: false,
	}
	postUp, postDown := natPostUpPostDown(inst)
	ext := defaultRouteInterface()
	if ext == "" {
		if postUp != "" || postDown != "" {
			t.Errorf("no default route: PostUp/PostDown must be empty, got up=%q down=%q", postUp, postDown)
		}
		return
	}
	if postUp == "" {
		t.Fatalf("default route %q exists but PostUp is empty", ext)
	}
	if !strings.Contains(postUp, "MASQUERADE") {
		t.Errorf("PostUp must contain MASQUERADE, got %q", postUp)
	}
	if !strings.Contains(postUp, "-s 10.8.0.0/24") {
		t.Errorf("PostUp must MASQUERADE the client subnet, got %q", postUp)
	}
	if !strings.Contains(postUp, "ip_forward") {
		t.Errorf("PostUp must enable ip_forward, got %q", postUp)
	}
	if !strings.Contains(postDown, "MASQUERADE") {
		t.Errorf("PostDown must contain MASQUERADE, got %q", postDown)
	}
}

func TestNatRulesFor(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", RouteThroughXray: false,
	}
	rules := natRulesFor(inst, "eth0")
	if len(rules) != 5 {
		t.Fatalf("natRulesFor = %d rules, want 5: %+v", len(rules), rules)
	}
	markSpec := strings.Join(rules[0].spec, " ")
	if rules[0].table != "mangle" || rules[0].chain != "PREROUTING" ||
		!strings.Contains(markSpec, "-i awg1") || !strings.Contains(markSpec, "MARK") {
		t.Errorf("rule[0] must MARK iif awg1, got %s %s %s", rules[0].table, rules[0].chain, markSpec)
	}
	subnetMasq := strings.Join(rules[1].spec, " ")
	if rules[1].table != "nat" || rules[1].chain != "POSTROUTING" ||
		!strings.Contains(subnetMasq, "-s 10.8.0.0/24") || !strings.Contains(subnetMasq, "-o eth0") ||
		!strings.Contains(subnetMasq, "MASQUERADE") {
		t.Errorf("rule[1] must MASQUERADE -s subnet out eth0, got %s %s %s", rules[1].table, rules[1].chain, subnetMasq)
	}
	masq := strings.Join(rules[2].spec, " ")
	if rules[2].table != "nat" || rules[2].chain != "POSTROUTING" ||
		!strings.Contains(masq, "mark") || !strings.Contains(masq, "-o eth0") ||
		!strings.Contains(masq, "MASQUERADE") {
		t.Errorf("rule[2] must MASQUERADE by mark out eth0, got %s %s %s", rules[2].table, rules[2].chain, masq)
	}
	fwdIn := strings.Join(rules[3].spec, " ")
	fwdOut := strings.Join(rules[4].spec, " ")
	if !strings.Contains(fwdIn, "-i awg1") || !strings.Contains(fwdOut, "-o awg1") {
		t.Errorf("FORWARD rules must cover both awg1 legs, got %q / %q", fwdIn, fwdOut)
	}
}

// A host between DHCP leases answers `ip route show default` with exit 0 and
// no output; without stickiness that flaps every non-routed interface twice.
func TestStickyDefaultRoute_SurvivesAMissingDefaultRoute(t *testing.T) {
	t.Cleanup(func() { lastDefaultRouteIface.Store("") })
	lastDefaultRouteIface.Store("")
	if got := stickyDefaultRoute(""); got != "" {
		t.Fatalf("nothing seen yet: got %q, want empty", got)
	}
	if got := stickyDefaultRoute("eth0"); got != "eth0" {
		t.Fatalf("a real answer must pass through: got %q", got)
	}
	if got := stickyDefaultRoute(""); got != "eth0" {
		t.Fatalf("a momentary gap must keep the last interface: got %q, want eth0", got)
	}
	if got := stickyDefaultRoute("eth1"); got != "eth1" {
		t.Fatalf("a real failover must still land: got %q, want eth1", got)
	}
	if got := stickyDefaultRoute(""); got != "eth1" {
		t.Fatalf("after failover the new interface must stick: got %q, want eth1", got)
	}
}

func TestNatRulesFor_SkipsUnroutable(t *testing.T) {
	base := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320, Address: "10.8.0.1/24"}

	routed := base
	routed.RouteThroughXray = true
	if got := natRulesFor(routed, "eth0"); got != nil {
		t.Errorf("routeThroughXray must skip NAT (Xray owns routing), got %+v", got)
	}

	noAddr := base
	noAddr.Address = ""
	if got := natRulesFor(noAddr, "eth0"); got != nil {
		t.Errorf("empty Address must skip NAT, got %+v", got)
	}

	if got := natRulesFor(base, ""); got != nil {
		t.Errorf("no external interface must skip NAT, got %+v", got)
	}
}

func TestInstanceFingerprint_ChangesOnDeviceField(t *testing.T) {
	awg3 := true
	SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", AwgVersion: "3"}
	before := deviceFP(inst)
	inst.RekeyAfterTime = "120"
	after := deviceFP(inst)
	if before == after {
		t.Fatal("fingerprint must change when an AWG3 device timer field changes (restart trigger)")
	}
}

// The six device-level AWG3 fields are version-gated in the server .conf: they
// appear in [Interface] only when AwgVersion == "3" AND the field > 0. On a
// non-v3 inbound the lines must NOT appear even when the field carries a value
// (older kernels reject them in setconf), and a zero field stays silent on v3
// too (0 = kernel default). Mirrors the HeaderProtectionKey gating.
func TestRenderServerConf_DeviceFieldsVersionGated(t *testing.T) {
	awg3 := true
	SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	cases := []struct {
		name    string
		version string
		line    string
		set     func(inst *Instance)
		want    bool
	}{
		{"ContentPaddingAddition v3 set", "3", "ContentPaddingAddition = 32", func(i *Instance) { i.ContentPaddingAddition = "32" }, true},
		{"ContentPaddingAddition v2 set", "2", "ContentPaddingAddition = 32", func(i *Instance) { i.ContentPaddingAddition = "32" }, false},
		{"ContentPaddingAddition v3 zero", "3", "ContentPaddingAddition =", func(i *Instance) {}, false},
		{"RekeyAfterTime v3 set", "3", "RekeyAfterTime = 120", func(i *Instance) { i.RekeyAfterTime = "120" }, true},
		{"RekeyAfterTime v2 set", "2", "RekeyAfterTime = 120", func(i *Instance) { i.RekeyAfterTime = "120" }, false},
		{"RekeyAfterTime v3 zero", "3", "RekeyAfterTime =", func(i *Instance) {}, false},
		{"RekeyTimeout v3 set", "3", "RekeyTimeout = 5", func(i *Instance) { i.RekeyTimeout = "5" }, true},
		{"RekeyTimeout v2 set", "2", "RekeyTimeout = 5", func(i *Instance) { i.RekeyTimeout = "5" }, false},
		{"RekeyTimeout v1.5 set", "1.5", "RekeyTimeout = 5", func(i *Instance) { i.RekeyTimeout = "5" }, false},
		{"RejectAfterTime v3 set", "3", "RejectAfterTime = 180", func(i *Instance) { i.RejectAfterTime = "180" }, true},
		{"RejectAfterTime v2 set", "2", "RejectAfterTime = 180", func(i *Instance) { i.RejectAfterTime = "180" }, false},
		{"KeepaliveTimeout v3 set", "3", "KeepaliveTimeout = 10", func(i *Instance) { i.KeepaliveTimeout = "10" }, true},
		{"KeepaliveTimeout v2 set", "2", "KeepaliveTimeout = 10", func(i *Instance) { i.KeepaliveTimeout = "10" }, false},
		{"MaxHandshakeAttempts v3 set", "3", "MaxHandshakeAttempts = 18", func(i *Instance) { i.MaxHandshakeAttempts = "18" }, true},
		{"MaxHandshakeAttempts v2 set", "2", "MaxHandshakeAttempts = 18", func(i *Instance) { i.MaxHandshakeAttempts = "18" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inst := Instance{
				Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320, Jc: 5,
				AwgVersion: tc.version,
			}
			tc.set(&inst)
			conf := renderServerConf(inst)
			contains := strings.Contains(conf, tc.line)
			if contains != tc.want {
				t.Errorf("version=%q want %q in conf=%v, got=%v\nConf:\n%s",
					tc.version, tc.line, tc.want, contains, conf)
			}
		})
	}
}

func TestInstanceFromInbound_DeviceFields(t *testing.T) {
	ib := &model.Inbound{
		Id:       9,
		Protocol: model.AWG,
		Settings: `{"privateKey":"k","awgVersion":"3",` +
			`"contentPaddingAddition":32,"rekeyAfterTime":120,"rekeyTimeout":5,` +
			`"rejectAfterTime":180,"keepaliveTimeout":10,"maxHandshakeAttempts":18}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.ContentPaddingAddition != "32" || inst.RekeyAfterTime != "120" ||
		inst.RekeyTimeout != "5" || inst.RejectAfterTime != "180" ||
		inst.KeepaliveTimeout != "10" || inst.MaxHandshakeAttempts != "18" {
		t.Fatalf("device fields not parsed: %+v", inst)
	}
}

// TestInstanceFromInbound_DeviceFieldRange pins the lucx.60 passthrough: a
// timer stored as an inclusive range string ("100-200") — the kernel's native
// u16_range form — reaches the Instance verbatim instead of being collapsed.
func TestInstanceFromInbound_DeviceFieldRange(t *testing.T) {
	ib := &model.Inbound{
		Id:       9,
		Protocol: model.AWG,
		Settings: `{"privateKey":"k","awgVersion":"3",` +
			`"contentPaddingAddition":"10-64","rekeyAfterTime":"100-200","rekeyTimeout":"3-7",` +
			`"rejectAfterTime":180,"keepaliveTimeout":"8-12","maxHandshakeAttempts":"15-20"}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.ContentPaddingAddition != "10-64" || inst.RekeyAfterTime != "100-200" ||
		inst.RekeyTimeout != "3-7" || inst.RejectAfterTime != "180" ||
		inst.KeepaliveTimeout != "8-12" || inst.MaxHandshakeAttempts != "15-20" {
		t.Fatalf("range/number mix not parsed verbatim: %+v", inst)
	}
}

func TestCollapseTimerForVersion(t *testing.T) {
	if got := CollapseTimerForVersion("15-25", "2"); got != "15" {
		t.Fatalf("v2 range → lo, got %q", got)
	}
	if got := CollapseTimerForVersion("15-25", "1.5"); got != "15" {
		t.Fatalf("v1.5 range → lo, got %q", got)
	}
	if got := CollapseTimerForVersion("15-25", "3"); got != "15-25" {
		t.Fatalf("v3 keeps range, got %q", got)
	}
	if got := CollapseTimerForVersion("25", "2"); got != "25" {
		t.Fatalf("single value passthrough, got %q", got)
	}
	if got := CollapseTimerForVersion("0", "3"); got != "" {
		t.Fatalf("zero omitted, got %q", got)
	}
}

func TestValidateObfuscationFields(t *testing.T) {
	if err := ValidateObfuscationFields("2", 4, 0, "5000-40000", "1", "2", "3"); err != nil {
		t.Fatalf("v2 range H must pass: %v", err)
	}
	if err := ValidateObfuscationFields("1.5", 4, 0, "5000-40000", "1", "2", "3"); err == nil {
		t.Fatal("v1.5 range H must fail")
	}
	if err := ValidateObfuscationFields("1.5", 4, 0, "abc", "1", "2", "3"); err == nil {
		t.Fatal("garbage H must fail")
	}
	if err := ValidateObfuscationFields("1.5", 4, 0, "5000", "100005", "200005", "300005"); err != nil {
		t.Fatalf("v1.5 singles must pass: %v", err)
	}
}

// Blank H1-H4 must fail when obfuscated (it would render "H1 = " and awg
// setconf rejects the whole conf); a plain WireGuard inbound must still pass.
func TestValidateObfuscationFields_RejectsEmptyH(t *testing.T) {
	tests := []struct {
		name    string
		jc, s1  int
		h       [4]string
		wantErr bool
	}{
		{"blank H2 with jc>0 is rejected", 4, 0, [4]string{"1001", "", "1003", "1004"}, true},
		{"blank H with no obfuscation passes", 0, 0, [4]string{"", "", "", ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateObfuscationFields("2", tt.jc, tt.s1, tt.h[0], tt.h[1], tt.h[2], tt.h[3])
			if tt.wantErr && !errors.Is(err, ErrEmptyObfuscationHeader) {
				t.Fatalf("want %v, got %v", ErrEmptyObfuscationHeader, err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}

// Upstream tools bound-check against UINT32_MAX and silently truncate
// (RekeyTimeout=70000 -> 4464); the panel must reject what u16 cannot hold.
func TestValidateDeviceTimer(t *testing.T) {
	tests := []struct {
		name         string
		value        AwgTimer
		wantErr      bool
		wantOutRange bool
	}{
		{"empty is kernel default", "", false, false},
		{"zero is kernel default", "0", false, false},
		{"zero range is kernel default", "0-0", false, false},
		{"legal single value", "150", false, false},
		{"legal range", "100-500", false, false},
		{"single value above u16 max", "70000", true, true},
		{"range hi above u16 max", "100-70000", true, true},
		{"range hi below lo", "500-100", true, true},
		{"not a number", "abc", true, false},
		{"trailing hyphen with no hi", "100-", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeviceTimer("RekeyTimeout", tt.value)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateDeviceTimer(%q) = %v, want nil", tt.value, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateDeviceTimer(%q) = nil, want error", tt.value)
			}
			if got := errors.Is(err, ErrTimerOutOfRange); got != tt.wantOutRange {
				t.Fatalf("ValidateDeviceTimer(%q) = %v, errors.Is(_, ErrTimerOutOfRange) = %v, want %v", tt.value, err, got, tt.wantOutRange)
			}
		})
	}
}

func TestOnlineTTLSeconds_UsesRekeyHi(t *testing.T) {
	inst := Instance{RekeyAfterTime: "300-600"}
	if got := onlineTTLSeconds(inst); got != 660 {
		t.Fatalf("online TTL = %d, want 660 (rekey hi + 60)", got)
	}
	if got := onlineTTLSeconds(Instance{}); got != handshakeOnlineTTL {
		t.Fatalf("default TTL = %d, want %d", got, handshakeOnlineTTL)
	}
}

// OutboundTag steers the Xray routing rule injectAwgEgress builds; it never
// reaches the .conf, and its edit already forces an Xray config regen.
func TestInstanceFingerprint_StableOnOutboundTag(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", RouteThroughXray: true, OutboundTag: "warp",
	}
	before := deviceFP(inst)
	inst.OutboundTag = "vless-out"
	if deviceFP(inst) != before {
		t.Fatal("OutboundTag never reaches the .conf: editing it must not restart the interface")
	}
}

// The renderer drops HPK, the device timers and the 3.1 flags when the host's
// module cannot take them, so the fingerprint has to follow the host too.
func TestInstanceFingerprint_ReflectsModuleCapabilities(t *testing.T) {
	base := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", AwgVersion: "3.1", Jc: 4,
		HeaderProtectionKey: "aBcD...base64hpk==",
		RekeyAfterTime:      "120",
		RandomTrailers:      true,
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
			supported := deviceFP(base)
			tc.flip(&no)
			if deviceFP(base) == supported {
				t.Fatalf("%s support is invisible to the fingerprint: a module upgrade changes the .conf without restarting the interface", tc.name)
			}
		})
	}
}

// Every renderable [Interface] value must restart the device; nothing else can
// carry a new key or port to the kernel.
func TestInstanceFingerprint_ChangesOnInterfaceFields(t *testing.T) {
	base := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", Jc: 4,
	}
	for _, tc := range []struct {
		name string
		mod  func(*Instance)
	}{
		{"Port", func(i *Instance) { i.Port = 47001 }},
		{"PrivateKey", func(i *Instance) { i.PrivateKey = "k2" }},
		{"MTU", func(i *Instance) { i.MTU = 1280 }},
		{"Address", func(i *Instance) { i.Address = "10.9.0.1/24" }},
		{"H1", func(i *Instance) { i.H1 = "100000-500000" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := base
			tc.mod(&changed)
			if deviceFP(base) == deviceFP(changed) {
				t.Fatalf("%s is not in the fingerprint: editing it would not restart the interface", tc.name)
			}
		})
	}
}

// A non-deterministic renderer would make the fingerprint drift on its own and
// restart every tunnel on the host for no reason.
func TestRenderServerConf_Deterministic(t *testing.T) {
	yes := true
	SetModuleSupportsAwg3(&yes)
	SetModuleSupportsAwg31(&yes)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil); SetModuleSupportsAwg31(nil) })
	base := Instance{
		Id: 3, Ifname: "awg3", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", AwgVersion: "3.1", DNS: "1.1.1.1",
		Jc: 8, Jmin: 70, Jmax: 200, S1: 30, S2: 60, S3: 20, S4: 10,
		H1: "1-2", H2: "3-4", H3: "5-6", H4: "7-8",
		I1: "<b 0xaa>", I2: "<b 0xbb>", I3: "<b 0xcc>", I4: "<b 0xdd>", I5: "<b 0xee>",
		HeaderProtectionKey:    "aBcD...base64hpk==",
		ContentPaddingAddition: "16", RekeyAfterTime: "120-180", RekeyTimeout: "5",
		RejectAfterTime: "180", KeepaliveTimeout: "10", MaxHandshakeAttempts: "18",
		RandomTrailers: true, DisableCookies: true,
		Peers: []PeerSpec{
			{PublicKey: "p1", PSK: "psk1", AllowedIPs: "10.8.0.2/32"},
			{PublicKey: "p2", AllowedIPs: "10.8.0.3/32"},
		},
	}
	for _, routed := range []bool{false, true} {
		inst := base
		inst.RouteThroughXray = routed
		want := renderServerConf(inst)
		for i := 0; i < 20; i++ {
			if got := renderServerConf(inst); got != want {
				t.Fatalf("routeThroughXray=%v: render %d differs from the first:\n%s\n---\n%s", routed, i, want, got)
			}
		}
		if fp := deviceFP(inst); fp != deviceFP(inst) {
			t.Fatalf("routeThroughXray=%v: fingerprint is not stable: %q", routed, fp)
		}
	}
}

// I1-I5 now go into the server .conf, so a change to one has to restart the
// interface. Without this the panel saves a new CPS set and the running device
// keeps sending the old one, with nothing to show for it.
func TestFingerprint_ChangesWithIFields(t *testing.T) {
	base := Instance{Ifname: "awg1", Port: 51820, PrivateKey: "k", Jc: 4}
	for _, tc := range []struct {
		name string
		mod  func(*Instance)
	}{
		{"I1", func(i *Instance) { i.I1 = "<b 0xaa>" }},
		{"I2", func(i *Instance) { i.I2 = "<b 0xaa>" }},
		{"I3", func(i *Instance) { i.I3 = "<b 0xaa>" }},
		{"I4", func(i *Instance) { i.I4 = "<b 0xaa>" }},
		{"I5", func(i *Instance) { i.I5 = "<b 0xaa>" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := base
			tc.mod(&changed)
			if deviceFP(base) == deviceFP(changed) {
				t.Fatalf("%s is not in the fingerprint: editing it would not restart the interface", tc.name)
			}
		})
	}
}
func TestAwgTimer_RejectsInject(t *testing.T) {
	var tm AwgTimer
	if err := json.Unmarshal([]byte(`"15\nPostUp = id"`), &tm); err != nil {
		t.Fatal(err)
	}
	if tm != "" {
		t.Fatalf("inject timer must unmarshal empty, got %q", tm)
	}
	if err := json.Unmarshal([]byte(`"100-500"`), &tm); err != nil {
		t.Fatal(err)
	}
	if tm != "100-500" {
		t.Fatalf("range timer = %q", tm)
	}
}

func TestConfValue_StripsNewlines(t *testing.T) {
	got := confValue("abc\nPostUp = evil")
	if strings.Contains(got, "\n") || strings.Contains(got, "PostUp") && strings.Contains(got, "\n") {
		t.Fatalf("newline survived: %q", got)
	}
	if got != "abcPostUp = evil" {
		t.Fatalf("confValue = %q", got)
	}
}

func TestRenderServerConf_NoInjectedLine(t *testing.T) {
	conf := renderServerConf(Instance{
		PrivateKey: "k",
		Port:       51820,
		H1:         "1\nPostUp = wget",
		Peers:      []PeerSpec{{PublicKey: "p"}},
	})
	for _, line := range strings.Split(conf, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "PostUp") && strings.Contains(line, "wget") {
			t.Fatalf("injected PostUp: %q", line)
		}
	}
}
