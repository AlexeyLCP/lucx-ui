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

func TestInstanceFingerprint_ChangesOnHeaderProtectionKey(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k"}
	before := inst.fingerprint()
	inst.HeaderProtectionKey = "aBcD...base64hpk=="
	after := inst.fingerprint()
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

func TestInstanceFingerprint_ChangesOnAwgVersion(t *testing.T) {
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k"}
	before := inst.fingerprint()
	inst.AwgVersion = "3"
	after := inst.fingerprint()
	if before == after {
		t.Fatal("fingerprint must change when AwgVersion is set (restart trigger)")
	}
}

func TestNormalizeAwgVersion(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "2"}, {"1.5", "1.5"}, {"2", "2"}, {"3", "3"}, {"garbage", "2"}, {"4", "2"},
	} {
		if got := NormalizeAWGVersion(tc.in); got != tc.want {
			t.Errorf("NormalizeAWGVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
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

func TestNatRulesFor(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", RouteThroughXray: false,
	}
	rules := natRulesFor(inst, "eth0")
	if len(rules) != 3 {
		t.Fatalf("natRulesFor = %d rules, want 3: %+v", len(rules), rules)
	}
	masq := strings.Join(rules[0].spec, " ")
	if rules[0].table != "nat" || rules[0].chain != "POSTROUTING" ||
		!strings.Contains(masq, "-s 10.8.0.0/24") || !strings.Contains(masq, "-o eth0") ||
		!strings.Contains(masq, "MASQUERADE") {
		t.Errorf("rule[0] must MASQUERADE 10.8.0.0/24 out eth0, got %s %s %s", rules[0].table, rules[0].chain, masq)
	}
	fwdIn := strings.Join(rules[1].spec, " ")
	fwdOut := strings.Join(rules[2].spec, " ")
	if !strings.Contains(fwdIn, "-i awg1") || !strings.Contains(fwdOut, "-o awg1") {
		t.Errorf("FORWARD rules must cover both awg1 legs, got %q / %q", fwdIn, fwdOut)
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
	inst := Instance{Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k"}
	before := inst.fingerprint()
	inst.RekeyAfterTime = 120
	after := inst.fingerprint()
	if before == after {
		t.Fatal("fingerprint must change when an AWG3 device timer field changes (restart trigger)")
	}
}

func TestInstanceFingerprint_ChangesOnAdvancedSecurity(t *testing.T) {
	inst := Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k",
		Peers: []PeerSpec{{PublicKey: "p1", PSK: "psk"}},
	}
	before := inst.fingerprint()
	inst.Peers[0].AdvancedSecurity = true
	after := inst.fingerprint()
	if before == after {
		t.Fatal("fingerprint must change when a peer's AdvancedSecurity flag is toggled (restart trigger)")
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
		{"ContentPaddingAddition v3 set", "3", "ContentPaddingAddition = 32", func(i *Instance) { i.ContentPaddingAddition = 32 }, true},
		{"ContentPaddingAddition v2 set", "2", "ContentPaddingAddition = 32", func(i *Instance) { i.ContentPaddingAddition = 32 }, false},
		{"ContentPaddingAddition v3 zero", "3", "ContentPaddingAddition =", func(i *Instance) {}, false},
		{"RekeyAfterTime v3 set", "3", "RekeyAfterTime = 120", func(i *Instance) { i.RekeyAfterTime = 120 }, true},
		{"RekeyAfterTime v2 set", "2", "RekeyAfterTime = 120", func(i *Instance) { i.RekeyAfterTime = 120 }, false},
		{"RekeyAfterTime v3 zero", "3", "RekeyAfterTime =", func(i *Instance) {}, false},
		{"RekeyTimeout v3 set", "3", "RekeyTimeout = 5", func(i *Instance) { i.RekeyTimeout = 5 }, true},
		{"RekeyTimeout v2 set", "2", "RekeyTimeout = 5", func(i *Instance) { i.RekeyTimeout = 5 }, false},
		{"RekeyTimeout v1.5 set", "1.5", "RekeyTimeout = 5", func(i *Instance) { i.RekeyTimeout = 5 }, false},
		{"RejectAfterTime v3 set", "3", "RejectAfterTime = 180", func(i *Instance) { i.RejectAfterTime = 180 }, true},
		{"RejectAfterTime v2 set", "2", "RejectAfterTime = 180", func(i *Instance) { i.RejectAfterTime = 180 }, false},
		{"KeepaliveTimeout v3 set", "3", "KeepaliveTimeout = 10", func(i *Instance) { i.KeepaliveTimeout = 10 }, true},
		{"KeepaliveTimeout v2 set", "2", "KeepaliveTimeout = 10", func(i *Instance) { i.KeepaliveTimeout = 10 }, false},
		{"MaxHandshakeAttempts v3 set", "3", "MaxHandshakeAttempts = 18", func(i *Instance) { i.MaxHandshakeAttempts = 18 }, true},
		{"MaxHandshakeAttempts v2 set", "2", "MaxHandshakeAttempts = 18", func(i *Instance) { i.MaxHandshakeAttempts = 18 }, false},
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

// AdvancedSecurity is the AWG3 peer-level advisory flag, written to [Peer] as
// "AdvancedSecurity = on" only when AwgVersion == "3" and the peer opted in.
// The current kernel ignores it on input, but the renderer must keep it out of
// v1/v2 configs so older kernels never see an unrecognized line.
func TestRenderServerConf_AdvancedSecurityInPeer(t *testing.T) {
	awg3 := true
	SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	for _, tc := range []struct {
		name    string
		version string
		adv     bool
		want    bool
	}{
		{"v3 true", "3", true, true},
		{"v3 false", "3", false, false},
		{"v2 true", "2", true, false},
		{"v1.5 true", "1.5", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst := Instance{
				Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320, Jc: 5,
				AwgVersion: tc.version,
				Peers:      []PeerSpec{{PublicKey: "p1", PSK: "psk", AdvancedSecurity: tc.adv}},
			}
			conf := renderServerConf(inst)
			contains := strings.Contains(conf, "AdvancedSecurity = on")
			if contains != tc.want {
				t.Errorf("want AdvancedSecurity=on in conf=%v, got=%v\nConf:\n%s",
					tc.want, contains, conf)
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
	if inst.ContentPaddingAddition != 32 || inst.RekeyAfterTime != 120 ||
		inst.RekeyTimeout != 5 || inst.RejectAfterTime != 180 ||
		inst.KeepaliveTimeout != 10 || inst.MaxHandshakeAttempts != 18 {
		t.Fatalf("device fields not parsed: %+v", inst)
	}
}

func TestInstanceFromInbound_AdvancedSecurity(t *testing.T) {
	ib := &model.Inbound{
		Id:       10,
		Protocol: model.AWG,
		Settings: `{"privateKey":"k","awgVersion":"3",` +
			`"clients":[{"id":"p1","password":"psk","enable":true,"advancedSecurity":true}]}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if len(inst.Peers) != 1 {
		t.Fatalf("expected 1 enabled peer, got %d", len(inst.Peers))
	}
	if !inst.Peers[0].AdvancedSecurity {
		t.Fatalf("peer AdvancedSecurity not parsed: %+v", inst.Peers[0])
	}
}
