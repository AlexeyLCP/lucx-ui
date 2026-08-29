// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import "bytes"

// tlsSession builds one client's side of a TLS session: the ClientHello, then
// the records that would follow it. Five ClientHellos to five hosts was never
// a shape a real client produces.
func tlsSession(host string, browser BrowserProfile) [5]Descriptor {
	var ch []byte
	switch browser {
	case BrowserFirefox:
		ch = buildFirefoxHello(host)
	case BrowserSafari:
		ch = buildSafariHello(host)
	default:
		ch = buildChromeHello(host)
	}
	var rec bytes.Buffer
	rec.WriteByte(0x16)
	rec.Write([]byte{0x03, 0x01})
	writeLen16(&rec, len(ch))
	rec.Write(ch)

	return [5]Descriptor{
		helloDescriptor(rec.Bytes()),
		tlsChangeCipherSpecFinished(),
		tlsAppData(96, 140),
		tlsAppData(28, 56),
		tlsAppData(24, 48),
	}
}

// helloDescriptor turns a rendered ClientHello into a template whose random and
// session id are refilled per send: both are unverifiable, so freezing them
// only leaves a replay signature. Their offsets are fixed by the TLS layout —
// record header 5, handshake header 4, legacy_version 2.
func helloDescriptor(rec []byte) Descriptor {
	const randOff = 5 + 4 + 2
	if len(rec) < randOff+33 {
		return Descriptor{}.Lit(rec)
	}
	sidLen := int(rec[randOff+32])
	sidOff := randOff + 33
	if len(rec) < sidOff+sidLen {
		return Descriptor{}.Lit(rec)
	}
	d := Descriptor{}.Lit(rec[:randOff]).Rand(32).Lit(rec[randOff+32 : sidOff])
	if sidLen > 0 {
		d = d.Rand(sidLen)
	}
	return d.Lit(rec[sidOff+sidLen:])
}

// tlsChangeCipherSpecFinished is the middlebox-compatibility CCS followed by
// the encrypted Finished — ciphertext, so the whole body is a hole.
func tlsChangeCipherSpecFinished() Descriptor {
	n := 48 + rng.Intn(17)
	var h bytes.Buffer
	h.Write([]byte{0x14, 0x03, 0x03, 0x00, 0x01, 0x01})
	h.Write([]byte{0x17, 0x03, 0x03})
	writeLen16(&h, n)
	return Descriptor{}.Lit(h.Bytes()).Rand(n)
}

// tlsAppData is one application_data record of a length drawn in [lo,hi]. The
// length must be literal because it is on the wire; the body never is.
func tlsAppData(lo, hi int) Descriptor {
	n := lo + rng.Intn(hi-lo+1)
	var h bytes.Buffer
	h.Write([]byte{0x17, 0x03, 0x03})
	writeLen16(&h, n)
	return Descriptor{}.Lit(h.Bytes()).Rand(n)
}
