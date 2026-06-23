// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

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
			`"i1":"aa","i2":"bb","i3":"cc","i4":"dd","i5":"ee",` +
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
	if inst.I1 != "aa" || inst.I5 != "ee" {
		t.Fatalf("i1-i5 not parsed: %+v", inst)
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
	if inst.Peers[0].Keepalive != 25 {
		t.Fatalf("expected keepalive 25, got %d", inst.Peers[0].Keepalive)
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
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k",
		MTU: 1320, Jc: 5, Peers: []PeerSpec{{PublicKey: "p1", PSK: "psk", Keepalive: 25, AllowedIPs: "0.0.0.0/0, ::/0"}}}
	a := inst.fingerprint()
	b := inst.fingerprint()
	if a != b {
		t.Fatal("fingerprint must be deterministic for equal instances")
	}
}

func TestInstanceFingerprint_ChangesOnPeerMutation(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k",
		Peers: []PeerSpec{{PublicKey: "p1", PSK: "psk"}}}
	before := inst.fingerprint()
	inst.Peers = append(inst.Peers, PeerSpec{PublicKey: "p2", PSK: "psk2"})
	after := inst.fingerprint()
	if before == after {
		t.Fatal("fingerprint must change when a peer is added")
	}
}

func TestInstanceFingerprint_ChangesOnObfuscation(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", Jc: 5}
	before := inst.fingerprint()
	inst.Jc = 9
	after := inst.fingerprint()
	if before == after {
		t.Fatal("fingerprint must change when obfuscation (Jc) changes")
	}
}

func TestInstanceFingerprint_ChangesOnRoutingToggle(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k"}
	before := inst.fingerprint()
	inst.RouteThroughXray = true
	inst.OutboundTag = "warp"
	after := inst.fingerprint()
	if before == after {
		t.Fatal("fingerprint must change when routeThroughXray is toggled")
	}
}

func TestRenderServerConf_IncludesObfuscationAndPeers(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "server-priv",
		MTU: 1320, DNS: "1.1.1.1",
		Jc: 8, Jmin: 70, Jmax: 200, S1: 30, S2: 60, S3: 20, S4: 10,
		H1: "100000-500000", H2: "600000-900000", H3: "1000000-1500000", H4: "1600000-2000000",
		I1: "aa", I2: "bb", I3: "cc", I4: "dd", I5: "ee",
		Peers: []PeerSpec{{PublicKey: "peer-pub", PSK: "peer-psk", Keepalive: 25, AllowedIPs: "0.0.0.0/0, ::/0"}},
	}
	conf := renderServerConf(inst)
	mustContain := []string{
		"[Interface]",
		"PrivateKey = server-priv",
		"ListenPort = 47000",
		"MTU = 1320",
		"DNS = 1.1.1.1",
		"Jc = 8", "Jmin = 70", "Jmax = 200",
		"S1 = 30", "S2 = 60", "S3 = 20", "S4 = 10",
		"H1 = 100000-500000", "H4 = 1600000-2000000",
		"I1 = <b 0xaa>", "I5 = <b 0xee>",
		"[Peer]",
		"PublicKey = peer-pub",
		"PresharedKey = peer-psk",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
	}
	for _, s := range mustContain {
		if !strings.Contains(conf, s) {
			t.Errorf("renderServerConf missing %q\nConf:\n%s", s, conf)
		}
	}
}

func TestRenderServerConf_OmitsCPSWhenEmpty(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "I1 =") {
		t.Errorf("CPS I1 must be omitted when empty, got:\n%s", conf)
	}
}

func TestRenderServerConf_OmitsDNSWhenEmpty(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "DNS =") {
		t.Errorf("DNS must be omitted when empty, got:\n%s", conf)
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