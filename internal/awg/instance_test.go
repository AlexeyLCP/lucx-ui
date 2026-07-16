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
			`"i1":"<b 0xaa>","i2":"<b 0xbb>","i3":"<b 0xcc>","i4":"<b 0xdd>","i5":"<b 0xee>",` +
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
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k",
		MTU: 1320, Jc: 5, Peers: []PeerSpec{{PublicKey: "p1", PSK: "psk", Keepalive: 25, AllowedIPs: "0.0.0.0/0, ::/0"}},
	}
	a := inst.fingerprint()
	b := inst.fingerprint()
	if a != b {
		t.Fatal("fingerprint must be deterministic for equal instances")
	}
}

func TestInstanceFingerprint_ChangesOnPeerMutation(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k",
		Peers: []PeerSpec{{PublicKey: "p1", PSK: "psk"}},
	}
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
		I1: "<b 0xaa>", I2: "<b 0xbb>", I3: "<b 0xcc>", I4: "<b 0xdd>", I5: "<b 0xee>",
		Peers: []PeerSpec{{PublicKey: "peer-pub", PSK: "peer-psk", Keepalive: 25, AllowedIPs: "0.0.0.0/0, ::/0"}},
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
		"PersistentKeepalive = 25",
	}
	for _, s := range mustContain {
		if !strings.Contains(conf, s) {
			t.Errorf("renderServerConf missing %q\nConf:\n%s", s, conf)
		}
	}
	// DNS is CLIENT-ONLY — never in the server .conf.
	if strings.Contains(conf, "DNS =") {
		t.Errorf("DNS must never appear in server .conf, got:\n%s", conf)
	}
}

func TestRenderServerConf_OmitsCPSWhenEmpty(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "I1 =") {
		t.Errorf("CPS I1 must be omitted when empty, got:\n%s", conf)
	}
}

func TestRenderServerConf_NeverWritesDNS(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320, DNS: "1.1.1.1, 1.0.0.1"}
	conf := renderServerConf(inst)
	if strings.Contains(conf, "DNS =") {
		t.Errorf("DNS must never appear in server .conf even when set, got:\n%s", conf)
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
	if !strings.Contains(postUp, "ip_forward") {
		t.Errorf("PostUp must enable ip_forward, got %q", postUp)
	}
	if !strings.Contains(postDown, "MASQUERADE") {
		t.Errorf("PostDown must contain MASQUERADE, got %q", postDown)
	}
}
