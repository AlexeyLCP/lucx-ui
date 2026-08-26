// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"encoding/binary"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// materialise expands a descriptor the way the kernel's jp_spec_applymods does
// (kmod/src/junk.c:104-182): literals verbatim, every other segment refilled.
// Test-only — production never needs the bytes, only the descriptor.
func materialise(t *testing.T, d Descriptor) []byte {
	t.Helper()
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"
	var out []byte
	for _, s := range d.segs {
		switch s.kind {
		case segLit:
			out = append(out, s.lit...)
		case segRand:
			b := make([]byte, s.n)
			for i := range b {
				b[i] = byte(rand.Intn(256))
			}
			out = append(out, b...)
		case segRandChars:
			for i := 0; i < s.n; i++ {
				out = append(out, letters[rand.Intn(len(letters))])
			}
		case segRandDigits:
			for i := 0; i < s.n; i++ {
				out = append(out, digits[rand.Intn(len(digits))])
			}
		case segTimestamp:
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(time.Now().Unix()))
			out = append(out, b[:]...)
		default:
			t.Fatalf("materialise: unknown segment kind %v", s.kind)
		}
	}
	return out
}

// The kernel parser is whitespace-strict and case-sensitive, and amneziawg-go
// only tolerates a superset of it, so the kernel's spelling is the portable
// one. A drift here reaches the wire as a silently disabled packet.
func TestDescriptor_RendersKernelStrictSpelling(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  Descriptor
		want string
	}{
		{"literal", Descriptor{}.Lit([]byte{0xAA, 0xBB}), "<b 0xaabb>"},
		{"random", Descriptor{}.Rand(64), "<r 64>"},
		{"random chars", Descriptor{}.RandChars(16), "<rc 16>"},
		{"random digits", Descriptor{}.RandDigits(16), "<rd 16>"},
		{"timestamp", Descriptor{}.Timestamp(), "<t>"},
		{
			"segments keep source order",
			Descriptor{}.Lit([]byte{0x16, 0x03}).Rand(32).Lit([]byte{0x01}),
			"<b 0x1603><r 32><b 0x01>",
		},
		{"empty", Descriptor{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Nothing below this layer checks the packet size: an I-packet over the path
// MTU is IP-fragmented and sent silently (kmod/src/socket.c:85,151). WireLen is
// how the profile builders stay under it, so it must match the real expansion.
func TestDescriptor_WireLenMatchesMaterialisedBytes(t *testing.T) {
	d := Descriptor{}.Lit([]byte{0x16, 0x03, 0x01}).Rand(32).RandChars(8).RandDigits(4).Timestamp().Lit([]byte{0xFF})
	for i := 0; i < 20; i++ {
		if got, want := d.WireLen(), len(materialise(t, d)); got != want {
			t.Fatalf("draw %d: WireLen() = %d, materialised %d bytes", i, got, want)
		}
	}
}

func TestDescriptor_LenCountsDescriptorCharacters(t *testing.T) {
	d := Descriptor{}.Lit([]byte{0x16, 0x03}).Rand(32)
	if got, want := d.Len(), len(d.String()); got != want {
		t.Fatalf("Len() = %d, len(String()) = %d", got, want)
	}
}

// <r -5> parses in both engines and reaches get_random_bytes with a negative
// length (kmod/src/junk.c:112) and make([]byte, negative) in go. The generator
// must not be able to emit one whatever the inputs.
func TestDescriptor_ValidateRejectsUnusableSegments(t *testing.T) {
	for _, tc := range []struct {
		name string
		d    Descriptor
	}{
		{"negative random", Descriptor{}.Rand(-5)},
		{"zero random", Descriptor{}.Rand(0)},
		{"negative random chars", Descriptor{}.RandChars(-1)},
		{"zero random digits", Descriptor{}.RandDigits(0)},
		{"empty literal", Descriptor{}.Lit(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.d.Validate(); err == nil {
				t.Fatalf("Validate() accepted %q", tc.d.String())
			}
		})
	}
}

func TestDescriptor_ValidateAcceptsPortableSet(t *testing.T) {
	d := Descriptor{}.Lit([]byte{0x16}).Rand(1).RandChars(1).RandDigits(1).Timestamp()
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate() rejected a portable descriptor %q: %v", d.String(), err)
	}
	s := d.String()
	if strings.Contains(s, "#") {
		t.Fatal("a '#' truncates the .conf line at amneziawg-tools/src/config.c:628")
	}
	if strings.Contains(s, "  ") || strings.Contains(s, " >") || strings.Contains(s, "< ") {
		t.Fatalf("kernel rejects any extra whitespace: %q", s)
	}
	if strings.Contains(s, "0X") {
		t.Fatal("the 0x prefix must be lowercase (kmod/src/junk.c:18)")
	}
}
