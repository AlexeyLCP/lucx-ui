// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package signature

import (
	"bytes"
	"encoding/hex"
	"net"
	"strings"
	"testing"
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
	trunc := fillPackets([][]byte{oversized})
	want := "<b 0x" + strings.Repeat("ff", maxPacketSize) + ">"
	if trunc.I1 != want {
		t.Errorf("packet longer than %d bytes must be truncated, got len(I1)=%d", maxPacketSize, len(trunc.I1))
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

func TestCaptureIPAllowed(t *testing.T) {
	deny := []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "100.64.1.1", "::1"}
	for _, s := range deny {
		if captureIPAllowed(net.ParseIP(s)) {
			t.Errorf("%s must be refused", s)
		}
	}
	if !captureIPAllowed(net.ParseIP("1.1.1.1")) {
		t.Error("1.1.1.1 must be allowed")
	}
}
