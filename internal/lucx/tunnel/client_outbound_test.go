// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"
)

func TestParseNaiveLink(t *testing.T) {
	p, s, err := ParseSidecarLink("naive+https://alice:s3cret@n.example.org:8443")
	if err != nil {
		t.Fatal(err)
	}
	if p != SidecarProtocolNaive {
		t.Fatalf("protocol = %q", p)
	}
	if s.Host != "n.example.org" || s.Port != 8443 || s.User != "alice" || s.Pass != "s3cret" {
		t.Fatalf("parsed = %+v", s)
	}
}

func TestParseMieruLink(t *testing.T) {
	raw := "mierus://baozi:manlianpenfen@1.2.3.4?handshake-mode=HANDSHAKE_NO_WAIT&mtu=1400&multiplexing=MULTIPLEXING_HIGH&port=6666&protocol=TCP&profile=default"
	p, s, err := ParseSidecarLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p != SidecarProtocolMieru {
		t.Fatalf("protocol = %q", p)
	}
	if s.Host != "1.2.3.4" || s.User != "baozi" || s.Port != 6666 || s.MTU != 1400 {
		t.Fatalf("parsed = %+v", s)
	}
	if s.HandshakeMode != "HANDSHAKE_NO_WAIT" || s.Multiplexing != "MULTIPLEXING_HIGH" {
		t.Fatalf("client fields = %+v", s)
	}
}

func TestParseTrustTunnelLink(t *testing.T) {
	raw := "tt://u:p@vpn.example.com:443?security=tls&sni=vpn.example.com&alpn=h2&client_random_prefix=3eb5d634/ffffffff"
	p, s, err := ParseSidecarLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p != SidecarProtocolTrustTunnel {
		t.Fatalf("protocol = %q", p)
	}
	if s.Host != "vpn.example.com" || s.Port != 443 || s.User != "u" || s.Prefix != "3eb5d634/ffffffff" {
		t.Fatalf("parsed = %+v", s)
	}
	if strings.Contains(s.Prefix, "%2F") {
		t.Fatal("prefix must keep raw slash")
	}
}

func TestParseTrustTunnelTLVRejected(t *testing.T) {
	_, _, err := ParseSidecarLink("tt://?AAAA")
	if err == nil {
		t.Fatal("TLV deep-link must be rejected")
	}
}

func TestSidecarManageKeyDoesNotShareInboundPrefix(t *testing.T) {
	if strings.HasPrefix(SidecarManageKey(SidecarProtocolNaive, 5), "naive-") {
		t.Fatal("naiveout key must not use inbound naive- prefix")
	}
	if strings.HasPrefix(SidecarManageKey(SidecarProtocolMieru, 5), "mieru-") {
		t.Fatal("mieruout key must not use inbound mieru- prefix")
	}
	if strings.HasPrefix(SidecarManageKey(SidecarProtocolTrustTunnel, 5), "trusttunnel-") {
		t.Fatal("ttout key must not use inbound trusttunnel- prefix")
	}
}

func TestRenderNaiveClientJSON(t *testing.T) {
	got, err := RenderSidecarConfig(SidecarProtocolNaive, SidecarSettings{
		SocksPort: 39111, Host: "n.example.org", Port: 8443, User: "alice", Pass: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"listen": "socks://127.0.0.1:39111"`) {
		t.Fatalf("listen missing:\n%s", got)
	}
	if !strings.Contains(got, `"proxy": "https://alice:s3cret@n.example.org:8443"`) {
		t.Fatalf("proxy missing:\n%s", got)
	}
}

func TestRenderMieruClientJSON(t *testing.T) {
	got, err := RenderSidecarConfig(SidecarProtocolMieru, SidecarSettings{
		SocksPort: 39222, Host: "1.2.3.4", Port: 6666, User: "baozi", Pass: "pw", MTU: 1400,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"socks5Port": 39222`) {
		t.Fatalf("socks5Port missing:\n%s", got)
	}
	if !strings.Contains(got, `"rpcPort": 0`) {
		t.Fatalf("rpcPort must be 0:\n%s", got)
	}
	if !strings.Contains(got, `"ipAddress": "1.2.3.4"`) {
		t.Fatalf("ipAddress missing:\n%s", got)
	}
}

func TestRenderTrustTunnelClientTOML(t *testing.T) {
	got, err := RenderSidecarConfig(SidecarProtocolTrustTunnel, SidecarSettings{
		SocksPort: 39333, Host: "vpn.example.com", Port: 443, User: "u", Pass: "p",
		SNI: "vpn.example.com", ALPN: "h2", Prefix: "aa/bb",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "[listener.tun]") {
		t.Fatal("must not render TUN listener")
	}
	if !strings.Contains(got, "[listener.socks]") || !strings.Contains(got, `address = "127.0.0.1:39333"`) {
		t.Fatalf("socks listener missing:\n%s", got)
	}
	if !strings.Contains(got, "killswitch_enabled = false") {
		t.Fatal("killswitch must be off for SOCKS mode")
	}
}

func TestClientBinaryNamesDoNotCollide(t *testing.T) {
	if Naive.BinaryName() == NaiveClient.BinaryName() {
		t.Fatal("naive inbound/client binary names collided")
	}
	if Mieru.BinaryName() == MieruClient.BinaryName() {
		t.Fatal("mieru inbound/client binary names collided")
	}
	if TrustTunnel.BinaryName() == TrustTunnelClient.BinaryName() {
		t.Fatal("trusttunnel inbound/client binary names collided")
	}
	if !strings.HasPrefix(NaiveClient.BinaryName(), "naive-client-") {
		t.Fatalf("NaiveClient.BinaryName = %q", NaiveClient.BinaryName())
	}
}

func TestParseSidecarSettingsRequiresSocksPort(t *testing.T) {
	s := SidecarSettings{Host: "h", User: "u", SocksPort: 0}
	if s.Valid() {
		t.Fatal("socksPort 0 must be invalid")
	}
}
