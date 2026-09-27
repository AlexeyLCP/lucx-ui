// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func tproxyFixture() Instance {
	return Instance{Id: 7, Ifname: "awg7", Address: "10.8.0.1/24", PrivateKey: "test-only",
		RouteThroughXray: true, XrayRoutingMode: "tproxy", TproxyPort: 51453,
		Peers: []PeerSpec{{PublicKey: "test-peer", AllowedIPs: "10.8.0.2/32"}}}
}

func TestTproxyLegacyAndExplicitModes(t *testing.T) {
	for _, tc := range []struct {
		mode          string
		enabled, want bool
	}{
		{"", false, false}, {"", true, false}, {"tun", true, false}, {"tproxy", false, false}, {"tproxy", true, true},
	} {
		raw, _ := json.Marshal(map[string]any{"privateKey": "test-only", "address": "10.8.0.1/24",
			"routeThroughXray": tc.enabled, "xrayRoutingMode": tc.mode, "tproxyPort": 51453})
		inst, ok := InstanceFromInbound(&model.Inbound{Id: 7, Protocol: model.AWG, Settings: string(raw)})
		if !ok || inst.UsesTproxy() != tc.want {
			t.Fatalf("mode %q enabled %v: wrong effective mode", tc.mode, tc.enabled)
		}
		if inst.TproxyPort != 51453 {
			t.Fatal("port lost in settings parsing")
		}
	}
}

func TestTproxyValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Instance)
	}{
		{"port-zero", func(i *Instance) { i.TproxyPort = 0 }},
		{"port-overflow", func(i *Instance) { i.TproxyPort = 65536 }},
		{"ipv6", func(i *Instance) { i.Address = "fd00::1/64" }},
		{"shell-injection", func(i *Instance) { i.Ifname = "awg7; false" }},
		{"id-overflow", func(i *Instance) { i.Id = 65536 }},
		{"peer-escape", func(i *Instance) { i.Peers[0].AllowedIPs = "10.9.0.1/32" }},
		{"peer-v6", func(i *Instance) { i.Peers[0].AllowedIPs = "fd00::2/128" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := tproxyFixture()
			tc.change(&i)
			if i.validateTproxy() == nil {
				t.Fatal("accepted unsafe parameters")
			}
		})
	}
	if err := tproxyFixture().validateTproxy(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTproxySettings(`{"xrayRoutingMode":"invalid"}`); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if err := ValidateTproxySettings(`{"routeThroughXray":false}`); err != nil {
		t.Fatal("legacy rejected", err)
	}
}

func TestTproxyPlanOrderingAndIsolation(t *testing.T) {
	i := tproxyFixture()
	up, down := tproxyPostUpPostDown(i)
	for _, s := range []string{"ip -4 route replace local default dev lo table 40007", "fwmark 0x40000007/0xffffffff",
		"-p tcp -j TPROXY", "-p udp -j TPROXY", "--on-port 51453", "-d 10.8.0.0/24 -j RETURN", "-i awg7 -j DROP", "ip6tables"} {
		if !strings.Contains(up, s) {
			t.Errorf("missing %s", s)
		}
	}
	if strings.Index(up, "-i awg7 -j DROP") > strings.Index(up, "-I PREROUTING") {
		t.Fatal("guard must precede interception")
	}
	if strings.Contains(up, "-I FORWARD -i awg7 -o awg7 -j ACCEPT") {
		t.Fatal("P2P isolation bypassed")
	}
	if strings.Contains(up, "MASQUERADE") || strings.Contains(up, "dev tun") {
		t.Fatal("direct/TUN leak in TPROXY mode")
	}
	if strings.Contains(down, "route flush") || strings.Contains(down, "iptables -F") {
		t.Fatal("cleanup flushes foreign state")
	}
	if !strings.Contains(down, "while ip -4 rule del pref 10000 iif awg7") {
		t.Fatal("cleanup is not scoped")
	}
	i.P2P = true
	up, _ = tproxyPostUpPostDown(i)
	if !strings.Contains(up, "-I FORWARD -i awg7 -o awg7 -j ACCEPT") {
		t.Fatal("P2P enabled but not allowed")
	}
	if !strings.Contains(up, "-I FORWARD -i awg7 ! -o awg7 -j DROP") {
		t.Fatal("P2P guard must not overlap the hairpin accept during repair")
	}
}

func TestTproxyFingerprintAndSecretsUnchanged(t *testing.T) {
	i := tproxyFixture()
	before := renderServerConf(i)
	i.XrayRoutingMode = "tun"
	tun := renderServerConf(i)
	if deviceFingerprint(before) == deviceFingerprint(tun) {
		t.Fatal("switch does not restart interface")
	}
	_, beforePeers, _ := strings.Cut(before, "\n[Peer]\n")
	_, tunPeers, _ := strings.Cut(tun, "\n[Peer]\n")
	if beforePeers != tunPeers {
		t.Fatal("routing change modified issued peers")
	}
	i.XrayRoutingMode = "tproxy"
	i.TproxyPort++
	if deviceFingerprint(before) == deviceFingerprint(renderServerConf(i)) {
		t.Fatal("port change does not restart interface")
	}
}

func TestTproxyInboundShape(t *testing.T) {
	ib := TproxyInbound("inbound-awg-7", 51453)
	if string(ib.Listen) != `"127.0.0.1"` || ib.Port != 51453 || ib.Protocol != "dokodemo-door" || ib.Tag != "inbound-awg-7" {
		t.Fatal("incorrect listener", ib)
	}
	var settings map[string]any
	if err := json.Unmarshal(ib.Settings, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["followRedirect"] != true || settings["allowedNetwork"] != "tcp,udp" {
		t.Fatal("destination/protocols lost")
	}
	if !strings.Contains(string(ib.StreamSettings), `"tproxy":"tproxy"`) {
		t.Fatal("missing transparent socket")
	}
	if !strings.Contains(string(ib.Sniffing), `"routeOnly":true`) {
		t.Fatal("sniffing rewrites destination")
	}
}
