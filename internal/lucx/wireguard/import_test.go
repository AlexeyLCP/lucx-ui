// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"strings"
	"testing"
)

func TestImportStandardWireGuardConfigPreservesPeers(t *testing.T) {
	text := "[Interface]\nPrivateKey = " + testKey(1) + "\nAddress = 10.90.0.1/24\nListenPort = 51820\nMTU = 1280\n\n[Peer]\nPublicKey = " + testKey(2) + "\nPresharedKey = " + testKey(3) + "\nAllowedIPs = 10.90.0.2/32\nPersistentKeepalive = 25\n"
	preview, err := ImportConfig(text, 32101)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Enable || preview.Port != 51820 {
		t.Fatal("import must be disabled and preserve port")
	}
	preview.Id = 7
	inst, err := Parse(preview)
	if err != nil {
		t.Fatal(err)
	}
	if inst.PrivateKey != testKey(1) || len(inst.Peers) != 1 || inst.Peers[0].PublicKey != testKey(2) || inst.Peers[0].PSK != testKey(3) || inst.Peers[0].Keepalive != 25 {
		t.Fatal("import changed issued credentials")
	}
	if strings.Contains(preview.Settings, `"privateKey"`) {
		t.Fatal("server config cannot recover client private keys")
	}
	for _, hook := range []string{"PostUp = touch /tmp/unsafe", "PreDown = echo unsafe", "SaveConfig = true", "Jc = 4"} {
		if _, err := ImportConfig(strings.Replace(text, "MTU = 1280", "MTU = 1280\n"+hook, 1), 32101); err == nil {
			t.Fatalf("unsafe import field accepted: %s", hook)
		}
	}
}
