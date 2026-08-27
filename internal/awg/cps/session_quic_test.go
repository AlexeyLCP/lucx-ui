// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

var testQUICv1Salt = []byte{0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17, 0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a}

func testExpandLabel(t *testing.T, secret []byte, label string, n int) []byte {
	t.Helper()
	full := "tls13 " + label
	var info bytes.Buffer
	binary.Write(&info, binary.BigEndian, uint16(n))
	info.WriteByte(byte(len(full)))
	info.WriteString(full)
	info.WriteByte(0)
	out, err := hkdf.Expand(sha256.New, secret, info.String(), n)
	if err != nil {
		t.Fatalf("hkdf expand: %v", err)
	}
	return out
}

// openInitial does what a DPI does to a QUIC Initial: derive the keys from the
// DCID carried in the packet itself (RFC 9001 §5.2), strip header protection,
// and AEAD-open it. A packet that fails here is not a QUIC Initial to anyone.
func openInitial(t *testing.T, pkt []byte) []byte {
	t.Helper()
	if len(pkt) < 32 {
		t.Fatalf("packet too short: %d", len(pkt))
	}
	p := 1 + 4
	dcidLen := int(pkt[p])
	p++
	dcid := pkt[p : p+dcidLen]
	p += dcidLen
	scidLen := int(pkt[p])
	p += 1 + scidLen
	tokenLen := int(pkt[p]) // built as a single zero byte
	if tokenLen != 0 {
		t.Fatalf("unexpected token length byte %#x", pkt[p])
	}
	p++
	switch pkt[p] >> 6 {
	case 0:
		p++
	case 1:
		p += 2
	default:
		p += 4
	}
	pnOffset := p

	secret, err := hkdf.Extract(sha256.New, dcid, testQUICv1Salt)
	if err != nil {
		t.Fatalf("hkdf extract: %v", err)
	}
	client := testExpandLabel(t, secret, "client in", 32)
	key := testExpandLabel(t, client, "quic key", 16)
	iv := testExpandLabel(t, client, "quic iv", 12)
	hp := testExpandLabel(t, client, "quic hp", 16)

	if len(pkt) < pnOffset+20 {
		t.Fatalf("no room for a header-protection sample")
	}
	hpBlock, err := aes.NewCipher(hp)
	if err != nil {
		t.Fatal(err)
	}
	mask := make([]byte, 16)
	hpBlock.Encrypt(mask, pkt[pnOffset+4:pnOffset+20])

	hdr := append([]byte(nil), pkt[:pnOffset+4]...)
	hdr[0] ^= mask[0] & 0x0F
	pnLen := int(hdr[0]&0x03) + 1
	for i := 0; i < pnLen; i++ {
		hdr[pnOffset+i] ^= mask[1+i]
	}
	hdr = hdr[:pnOffset+pnLen]

	var pn uint64
	for _, b := range hdr[pnOffset:] {
		pn = pn<<8 | uint64(b)
	}
	nonce := append([]byte(nil), iv...)
	for i := 0; i < 8; i++ {
		nonce[len(nonce)-1-i] ^= byte(pn >> (8 * i))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gcm.Open(nil, nonce, pkt[pnOffset+pnLen:], hdr)
	if err != nil {
		t.Fatalf("Initial did not authenticate with keys derived from its own DCID: %v", err)
	}
	return plain
}

// The Initial is the one packet an observer can decrypt, because its keys come
// from the DCID in the clear. Getting it right is the whole reason it stays a
// literal instead of becoming a hole.
func TestQUICSession_InitialDecryptsAndCarriesTheSNI(t *testing.T) {
	for _, br := range []BrowserProfile{BrowserChrome, BrowserFirefox, BrowserSafari} {
		t.Run(string(br), func(t *testing.T) {
			set, err := quicSession("example.com", br)
			if err != nil {
				t.Fatal(err)
			}
			pkt := materialise(t, set[0])
			if len(pkt) != 1200 {
				t.Fatalf("Initial is %d bytes; RFC 9000 requires at least 1200", len(pkt))
			}
			plain := openInitial(t, pkt)
			// BuildQUICInitial seals the packet number into the payload as well as
			// the header, so the frames open with PADDING. Legal, but not what a
			// browser emits — see the follow-up note in the track file.
			i := 0
			for i < len(plain) && plain[i] == 0x00 {
				i++
			}
			if i >= len(plain) || plain[i] != 0x06 {
				t.Fatalf("no CRYPTO frame in the decrypted payload: %#v", plain[:min(12, len(plain))])
			}
			// The CRYPTO frame carries the handshake message; wrap it back into
			// a record so a real TLS stack can read the SNI out of it.
			off := i + 1 + 1
			switch plain[off] >> 6 {
			case 0:
				off++
			case 1:
				off += 2
			default:
				off += 4
			}
			ch := plain[off:]
			hsLen := int(ch[1])<<16 | int(ch[2])<<8 | int(ch[3])
			var rec bytes.Buffer
			rec.WriteByte(0x16)
			rec.Write([]byte{0x03, 0x01})
			writeLen16(&rec, hsLen+4)
			rec.Write(ch[:hsLen+4])
			chi := clientHelloInfo(t, rec.Bytes())
			if chi == nil {
				t.Fatal("the ClientHello inside the Initial does not parse")
			}
			if chi.ServerName != "example.com" {
				t.Fatalf("ServerName = %q", chi.ServerName)
			}
		})
	}
}

// Coherence: one session means one connection id across every packet.
func TestQUICSession_AllPacketsShareOneConnectionID(t *testing.T) {
	set, err := quicSession("example.com", BrowserChrome)
	if err != nil {
		t.Fatal(err)
	}
	initial := materialise(t, set[0])
	dcidLen := int(initial[5])
	dcid := initial[6 : 6+dcidLen]
	for n := 1; n < 5; n++ {
		pkt := materialise(t, set[n])
		if !bytes.Contains(pkt, dcid) {
			t.Fatalf("I%d does not carry the session's connection id", n+1)
		}
	}
}

// I1 must not vary — its ciphertext is what authenticates it. I2-I5 are
// encrypted with keys no observer holds, so they vary freely.
func TestQUICSession_ContinuationsVaryButInitialDoesNot(t *testing.T) {
	set, err := quicSession("example.com", BrowserChrome)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(materialise(t, set[0]), materialise(t, set[0])) {
		t.Fatal("the Initial varies between sends; it would stop authenticating")
	}
	for n := 1; n < 5; n++ {
		if bytes.Equal(materialise(t, set[n]), materialise(t, set[n])) {
			t.Fatalf("I%d does not vary between sends", n+1)
		}
	}
}

// The embedded ClientHello is what makes the Initial browser-shaped, and a
// sealed payload has no long zero runs — the old plaintext one padded with
// ~1700 of them, which no real client shows.
func TestQUICSession_InitialIsBrowserShapedAndSealed(t *testing.T) {
	seen := map[string]bool{}
	for _, br := range []BrowserProfile{BrowserChrome, BrowserFirefox, BrowserSafari} {
		set, err := quicSession("example.com", br)
		if err != nil {
			t.Fatal(err)
		}
		pkt := materialise(t, set[0])
		seen[string(pkt)] = true
		run, longest := 0, 0
		for _, b := range pkt {
			if b == 0x00 {
				run++
				if run > longest {
					longest = run
				}
			} else {
				run = 0
			}
		}
		if longest > 32 {
			t.Fatalf("%s: %d consecutive zero bytes inside an AEAD-sealed packet", br, longest)
		}
	}
	if len(seen) != 3 {
		t.Fatal("the browser profiles produced identical Initials")
	}
}
