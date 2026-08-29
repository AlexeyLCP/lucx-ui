// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"bytes"

	"github.com/mhsanaei/3x-ui/v3/internal/awg/signature"
)

// quicSession builds one client's QUIC session. The Initial stays literal
// because its keys derive from the connection id it carries in the clear, so
// it is the one packet an observer can decrypt — and must therefore be real.
func quicSession(host string, browser BrowserProfile) ([5]Descriptor, error) {
	var ch []byte
	switch browser {
	case BrowserFirefox:
		ch = buildFirefoxHello(host)
	case BrowserSafari:
		ch = buildSafariHello(host)
	default:
		ch = buildChromeHello(host)
	}
	dcid := randomBytes(8)
	initial, err := signature.BuildQUICInitial(dcid, ch)
	if err != nil {
		return [5]Descriptor{}, err
	}
	return [5]Descriptor{
		Descriptor{}.Lit(initial),
		quicHandshakePacket(dcid, 1, 96+rng.Intn(45)),
		quicShortHeaderPacket(dcid, 2, 16+rng.Intn(17)),
		quicShortHeaderPacket(dcid, 3, 16+rng.Intn(17)),
		quicShortHeaderPacket(dcid, 4, 16+rng.Intn(17)),
	}, nil
}

// quicHandshakePacket is the long-header Handshake that follows the Initial.
// Its keys come from the handshake secrets, which an observer never sees, so
// the payload is a hole.
func quicHandshakePacket(dcid []byte, pn uint32, n int) Descriptor {
	var h bytes.Buffer
	h.WriteByte(0xE3) // long header, Handshake, 4-byte packet number
	h.Write([]byte{0x00, 0x00, 0x00, 0x01})
	h.WriteByte(byte(len(dcid)))
	h.Write(dcid)
	h.WriteByte(0x00) // no source connection id
	writeVarint(&h, 4+n)
	h.Write([]byte{byte(pn >> 24), byte(pn >> 16), byte(pn >> 8), byte(pn)})
	return Descriptor{}.Lit(h.Bytes()).Rand(n)
}

// quicShortHeaderPacket is a 1-RTT packet: the connection id keeps the session
// coherent, the packet numbers advance, and the body is application ciphertext.
func quicShortHeaderPacket(dcid []byte, pn uint16, n int) Descriptor {
	var h bytes.Buffer
	h.WriteByte(0x41) // short header, 2-byte packet number
	h.Write(dcid)
	h.Write([]byte{byte(pn >> 8), byte(pn)})
	return Descriptor{}.Lit(h.Bytes()).Rand(n)
}
