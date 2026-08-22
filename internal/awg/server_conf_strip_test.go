// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"
)

func TestStripAwgQuick_DropsQuickKeysKeepsProtocol(t *testing.T) {
	yes := true
	SetModuleSupportsAwg3(&yes)
	SetModuleSupportsAwg31(&yes)
	t.Cleanup(func() {
		SetModuleSupportsAwg3(nil)
		SetModuleSupportsAwg31(nil)
	})
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 443, PrivateKey: "server-priv",
		Address: "10.200.0.1/24", MTU: 1320,
		Jc: 4, Jmin: 10, Jmax: 50, S1: 20, S2: 30, S3: 15, S4: 12,
		H1: "1-100", H2: "101-200", H3: "201-300", H4: "301-400",
		AwgVersion: "3.1", HeaderProtectionKey: "hpk-key",
		RekeyAfterTime: "120", RekeyTimeout: "5",
		RandomTrailers: true, DisableCookies: true,
		RouteThroughXray: false,
		Peers: []PeerSpec{
			{PublicKey: "peer-pub", PSK: "psk-key", AllowedIPs: "10.200.0.2/32"},
		},
	}
	conf := renderServerConf(inst)
	stripped := stripAwgQuick(conf)

	for _, drop := range []string{
		"Address =", "MTU =", "PostUp =", "PostDown =",
		"DNS =", "Table =", "SaveConfig =",
	} {
		if strings.Contains(stripped, drop) {
			t.Errorf("stripAwgQuick must drop %q, got:\n%s", drop, stripped)
		}
	}
	for _, keep := range []string{
		"PrivateKey = server-priv",
		"ListenPort = 443",
		"Jc = 4",
		"HeaderProtectionKey = hpk-key",
		"RekeyAfterTime = 120",
		"RekeyTimeout = 5",
		"RandomTrailers = on",
		"DisableCookies = on",
		"[Peer]",
		"PublicKey = peer-pub",
		"PresharedKey = psk-key",
		"AllowedIPs = 10.200.0.2/32",
		xuiManagedMarker,
	} {
		if !strings.Contains(stripped, keep) {
			t.Errorf("stripAwgQuick must keep %q, got:\n%s", keep, stripped)
		}
	}
	if !strings.Contains(conf, "Address = 10.200.0.1/24") {
		t.Fatal("renderServerConf must still write Address for awg-quick up")
	}
}

func TestStripAwgQuick_DoesNotTouchPeerSection(t *testing.T) {
	conf := "[Interface]\nAddress = 10.0.0.1/24\n\n[Peer]\nPublicKey = p\nAllowedIPs = 10.0.0.2/32\n"
	got := stripAwgQuick(conf)
	if strings.Contains(got, "Address =") {
		t.Errorf("Interface Address must be stripped, got:\n%s", got)
	}
	if !strings.Contains(got, "AllowedIPs = 10.0.0.2/32") {
		t.Errorf("Peer AllowedIPs must stay, got:\n%s", got)
	}
}
