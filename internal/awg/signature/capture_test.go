// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package signature

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
)

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"google.com", "google.com"},
		{"  google.com  ", "google.com"},
		{"https://google.com", "google.com"},
		{"https://google.com/path?q=1", "google.com"},
		{"https://google.com:443/path", "google.com"},
		{"google.com:443", "google.com"},
		{"google.com/path", "google.com"},
		{"sub.example.co.uk", "sub.example.co.uk"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeDomain(tt.in); got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFillPackets(t *testing.T) {
	two := fillPackets([][]byte{{0xAA}, {0xBB, 0xCC}})
	if two.I1 != "<b 0xaa>" || two.I2 != "<b 0xbbcc>" {
		t.Errorf("I1/I2 wrong: %q %q", two.I1, two.I2)
	}
	if two.I3 != "" || two.I4 != "" || two.I5 != "" {
		t.Errorf("I3-I5 must stay empty with 2 packets, got %q %q %q", two.I3, two.I4, two.I5)
	}

	oversized := bytes.Repeat([]byte{0xFF}, 2000)
	dropped := fillPackets([][]byte{oversized})
	if dropped.I1 != "" {
		t.Errorf("a packet that busts the netlink budget must be dropped whole, got len(I1)=%d", len(dropped.I1))
	}

	six := make([][]byte, 6)
	for i := range six {
		six[i] = []byte{byte(i + 1)}
	}
	filled := fillPackets(six)
	if filled.I5 != "<b 0x05>" {
		t.Errorf("I5 = %q, want <b 0x05> (5th packet)", filled.I5)
	}
	if strings.Contains(filled.I5, "06") {
		t.Errorf("6th packet must be dropped, I5 = %q", filled.I5)
	}
}

func TestAppendVarint(t *testing.T) {
	tests := []struct {
		v    int
		want []byte
	}{
		{0, []byte{0x00}},
		{63, []byte{0x3F}},
		{64, []byte{0x40, 0x40}},
		{16383, []byte{0x7F, 0xFF}},
		{16384, []byte{0x80, 0x00, 0x40, 0x00}},
	}
	for _, tt := range tests {
		if got := appendVarint(tt.v); !bytes.Equal(got, tt.want) {
			t.Errorf("appendVarint(%d) = %x, want %x", tt.v, got, tt.want)
		}
	}
}

func TestHkdfExpandLabel_Deterministic(t *testing.T) {
	secret := bytes.Repeat([]byte{0x42}, 32)
	a := hkdfExpandLabel(secret, "quic key", nil, 16)
	b := hkdfExpandLabel(secret, "quic key", nil, 16)
	if !bytes.Equal(a, b) {
		t.Errorf("same inputs must give same key: %x vs %x", a, b)
	}
	if len(a) != 16 {
		t.Errorf("len = %d, want 16", len(a))
	}
	c := hkdfExpandLabel(secret, "quic iv", nil, 16)
	if bytes.Equal(a, c) {
		t.Errorf("different labels must give different keys")
	}
}

func TestBuildTLSClientHello_Structure(t *testing.T) {
	host := "example.com"
	msg, err := buildTLSClientHello(host)
	if err != nil {
		t.Fatalf("buildTLSClientHello: %v", err)
	}
	if msg[0] != 0x01 {
		t.Errorf("handshake type = 0x%02x, want 0x01 (ClientHello)", msg[0])
	}
	declared := int(msg[1])<<16 | int(msg[2])<<8 | int(msg[3])
	if declared != len(msg)-4 {
		t.Errorf("declared length %d != actual body %d", declared, len(msg)-4)
	}
	if !bytes.Contains(msg, []byte(host)) {
		t.Errorf("ClientHello must carry SNI host %q", host)
	}
	if !bytes.Contains(msg, []byte("h3")) {
		t.Errorf("ClientHello must advertise ALPN h3")
	}
	if len(msg) > 1100 {
		t.Errorf("ClientHello must fit a 1200-byte QUIC Initial with margin, got %d", len(msg))
	}
}

func TestBuildQUICInitial_Structure(t *testing.T) {
	pkt, err := buildQUICInitial("example.com")
	if err != nil {
		t.Fatalf("buildQUICInitial: %v", err)
	}
	if len(pkt) < 1200 {
		t.Errorf("QUIC Initial must be padded to >=1200 bytes, got %d", len(pkt))
	}
	if pkt[0]&0x80 == 0 {
		t.Errorf("first byte 0x%02x: long-header bit must be set (header protection masks only low 4 bits)", pkt[0])
	}
	if pkt[0]&0x40 == 0 {
		t.Errorf("first byte 0x%02x: fixed bit must be set", pkt[0])
	}
	if !bytes.Equal(pkt[1:5], []byte{0x00, 0x00, 0x00, 0x01}) {
		t.Errorf("version = %s, want QUIC v1 (0x00000001)", hex.EncodeToString(pkt[1:5]))
	}
	if pkt[5] != 8 {
		t.Errorf("DCID length = %d, want 8", pkt[5])
	}
}

func TestCapture_EmptyDomain(t *testing.T) {
	if _, err := Capture("  "); err == nil {
		t.Error("Capture with blank domain must fail")
	}
}

// A captured packet is only useful whole: half a QUIC Initial is a malformed
// packet on the wire. An oversized capture must lose whole packets, never
// bytes, and what survives must still fit what `awg show` can read back.
func TestFillPackets_KeepsWholePacketsWithinNetlinkBudget(t *testing.T) {
	budget := awg.IBytesBudget(awg.BaselineIfname, false)
	packet := func(b byte) []byte { return bytes.Repeat([]byte{b}, 1200) }
	res := fillPackets([][]byte{packet(1), packet(2), packet(3), packet(4), packet(5)})

	if got := awg.IBytes(res.I1, res.I2, res.I3, res.I4, res.I5); got > budget {
		t.Fatalf("captured set is %d bytes, budget %d — the interface comes up unreadable", got, budget)
	}
	if res.I1 == "" {
		t.Fatal("the first packet fits on its own and must be kept")
	}
	for n, f := range map[string]string{"I1": res.I1, "I2": res.I2, "I3": res.I3, "I4": res.I4, "I5": res.I5} {
		if f == "" {
			continue
		}
		payload := strings.TrimSuffix(strings.TrimPrefix(f, "<b 0x"), ">")
		if len(payload) != 2*1200 {
			t.Fatalf("%s carries %d hex chars — a packet was truncated, not dropped", n, len(payload))
		}
	}
}

// A capture small enough to fit must survive whole — the budget must not cost
// packets it does not have to.
func TestFillPackets_KeepsAllFiveWhenTheyFit(t *testing.T) {
	packet := func(b byte) []byte { return bytes.Repeat([]byte{b}, 300) }
	res := fillPackets([][]byte{packet(1), packet(2), packet(3), packet(4), packet(5)})
	for n, f := range map[string]string{"I1": res.I1, "I2": res.I2, "I3": res.I3, "I4": res.I4, "I5": res.I5} {
		if f == "" {
			t.Fatalf("%s dropped although the whole set fits the budget", n)
		}
	}
}

// A captured reply can be larger than the path carries as UDP payload; replaying
// it would leave the wire fragmented. It fits the netlink budget comfortably, so
// only the packet-size rule can drop it.
func TestFillPackets_DropsAPacketThatWouldFragment(t *testing.T) {
	big := bytes.Repeat([]byte{0xAA}, awg.MaxIPacketBytes+50)
	if n := awg.IBytes("<b 0x"+strings.Repeat("aa", len(big))+">", "", "", "", ""); n > awg.IBytesBudget(awg.BaselineIfname, false) {
		t.Fatalf("test packet is %d netlink bytes; it must fit the budget so the size rule is what drops it", n)
	}
	if res := fillPackets([][]byte{big}); res.I1 != "" {
		t.Fatalf("kept a %d-byte packet; the path carries %d", len(big), awg.MaxIPacketBytes)
	}
}
