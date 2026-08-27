// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// CPSResult holds the I1-I5 packet strings in the AmneziaWG CPS format
// ("<b 0xHEX>" for TLS/QUIC/SIP, "<r 2><b 0xHEX>" for DNS). Empty fields are
// omitted from the .conf.
type CPSResult struct {
	I1, I2, I3, I4, I5 string
}

// ErrCPSBudgetExceeded means no draw of the requested set fit maxIBytes.
var ErrCPSBudgetExceeded = errors.New("awg cps: I1-I5 exceed the netlink read budget")

// generateAttempts bounds the redraw loop: a full quic set clears a 3500-byte
// budget on ~1.7% of draws, so 16 rolls refused it most of the time. At ~0.11ms
// a draw this costs ~110ms in the worst case, which only a refusal pays.
const generateAttempts = 1024

// IBytes is what the kernel actually charges for I1-I5: each is a
// NUL-terminated netlink string attribute costing NLA_ALIGN(4+len+1).
func (r CPSResult) IBytes() int {
	n := 0
	for _, v := range []string{r.I1, r.I2, r.I3, r.I4, r.I5} {
		if v != "" {
			n += (len(v) + 8) &^ 3
		}
	}
	return n
}

// GenerateCPS redraws until the set fits maxIBytes (awg.IBytesBudget for the
// target interface). It refuses rather than shedding fields: a set the operator
// cannot save beats one that silently carries fewer packets than it asked for.
func GenerateCPS(profile MimicryProfile, region Region, domain string, browser BrowserProfile, onlyI1 bool, maxIBytes int) (CPSResult, error) {
	smallest := 0
	for try := 0; try < generateAttempts; try++ {
		r, err := generateCPSOnce(profile, region, domain, browser, onlyI1)
		if err != nil {
			return r, err
		}
		got := r.IBytes()
		if got <= maxIBytes {
			return r, nil
		}
		if smallest == 0 || got < smallest {
			smallest = got
		}
	}
	return CPSResult{}, fmt.Errorf("%w: smallest of %d draws was %d bytes, budget %d — pick a lighter mimicry profile, or turn the full I1-I5 set off",
		ErrCPSBudgetExceeded, generateAttempts, smallest, maxIBytes)
}

func generateCPSOnce(profile MimicryProfile, region Region, domain string, browser BrowserProfile, onlyI1 bool) (CPSResult, error) {
	dom := SelectDomain(profile, region, domain)
	var set [5]Descriptor
	switch profile {
	case ProfileTLS:
		set = tlsSession(dom, browser)
	case ProfileSIP:
		set = sipSession(dom)
	case ProfileQUIC:
		var err error
		if set, err = quicSession(dom, browser); err != nil {
			return CPSResult{}, err
		}
	case ProfileDNS:
		i1 := dnsPacket(dom)
		if onlyI1 {
			return CPSResult{I1: i1}, nil
		}
		pool := DomainPool(ProfileDNS, region)
		return CPSResult{
			I1: i1,
			I2: dnsPacket(pool[rng.Intn(len(pool))]),
			I3: dnsPacket(pool[rng.Intn(len(pool))]),
			I4: dnsPacket(pool[rng.Intn(len(pool))]),
			I5: dnsPacket(pool[rng.Intn(len(pool))]),
		}, nil
	default:
		return CPSResult{}, fmt.Errorf("awg cps: unknown profile %q", profile)
	}
	return fromSession(set, onlyI1)
}

// fromSession renders a session, refusing anything the descriptor layer would
// not stand behind rather than letting it reach a .conf.
func fromSession(set [5]Descriptor, onlyI1 bool) (CPSResult, error) {
	var r CPSResult
	out := [5]*string{&r.I1, &r.I2, &r.I3, &r.I4, &r.I5}
	for i, d := range set {
		if onlyI1 && i > 0 {
			break
		}
		if err := d.Validate(); err != nil {
			return CPSResult{}, err
		}
		*out[i] = d.String()
	}
	return r, nil
}

// dnsTag wraps a DNS packet with the AmneziaWG "<r 2><b 0xHEX>" prefix — the
// leading <r 2> is a 2-byte random tag the AWG kernel expects on DNS-shaped
// CPS packets (it distinguishes them from TLS-shaped ones in the parser).
func dnsTag(b []byte) string {
	return "<r 2><b 0x" + hex.EncodeToString(b) + ">"
}

// randomBytes returns n cryptographically-strong random bytes.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = crand.Read(b)
	return b
}

// randomHex returns a hex string of length 2*n (n random bytes).
func randomHex(n int) string {
	return hex.EncodeToString(randomBytes(n))
}

// randLowerAlphaNum returns a random lowercase alphanumeric string of length n.
func randLowerAlphaNum(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}

// writeLen16 writes a 2-byte big-endian length/value. Accepts int or uint16
// for ergonomics — CPS packet/extension lengths fit in 16 bits.
func writeLen16(b *bytes.Buffer, v any) {
	var u uint16
	switch x := v.(type) {
	case int:
		u = uint16(x)
	case uint16:
		u = x
	case int64:
		u = uint16(x)
	default:
		panic(fmt.Sprintf("writeLen16: unsupported type %T", v))
	}
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], u)
	b.Write(buf[:])
}

// writeUint24BE writes a 3-byte big-endian value (TLS record length uses 24 bits).
func writeUint24BE(b *bytes.Buffer, v int) {
	b.Write([]byte{byte(v >> 16), byte(v >> 8), byte(v)})
}

// greaseValue returns one of the GREASE extension values Chrome sprinkles
// through ClientHello to keep middleboxes tolerant of unknown values.
func greaseValue() uint16 {
	grease := []uint16{0x0A0A, 0x1A1A, 0x2A2A, 0x3A3A, 0x4A4A, 0x5A5A, 0x6A6A, 0x7A7A, 0x8A8A, 0x9A9A, 0xAAAA, 0xBABA, 0xCACA, 0xDADA, 0xEAEA, 0xFAFA}
	return grease[rng.Intn(len(grease))]
}

// ---- TLS ClientHello (browser-shaped) ----

// buildChromeHello builds a Chrome-shaped TLS 1.2 ClientHello handshake body:
// GREASE cipher group, Chrome extension order, compress_certificate, ALPS,
// random padding 0..48. Ported from the original pumbaX buildTLSClientHello.
func buildChromeHello(host string) []byte {
	var hs bytes.Buffer
	writeLen16(&hs, 0x0303)
	hs.Write(randomBytes(32))
	sid := randomBytes(32)
	hs.WriteByte(byte(len(sid)))
	hs.Write(sid)
	ciphers := []uint16{
		0x1301, 0x1302, 0x1303, 0xC02B, 0xC02F, 0xC02C, 0xC030,
		0xCCA9, 0xCCA8, 0xC013, 0xC014, 0x009C, 0x009D, 0x002F, 0x0035,
	}
	var cs bytes.Buffer
	cs.Write([]byte{0x00, 0x00})
	for _, c := range ciphers {
		writeLen16(&cs, c)
	}
	grease := greaseValue()
	cs.Bytes()[0] = byte(grease >> 8)
	cs.Bytes()[1] = byte(grease)
	writeLen16(&hs, cs.Len())
	hs.Write(cs.Bytes())
	hs.WriteByte(0x01)
	hs.WriteByte(0x00)

	var ext bytes.Buffer
	// Chrome draws its two GREASE extension types distinct. Letting them collide
	// makes a duplicate extension type, which strict parsers reject outright.
	greaseExt1, greaseExt2 := greaseValue(), greaseValue()
	for greaseExt2 == greaseExt1 {
		greaseExt2 = greaseValue()
	}
	writeLen16(&ext, greaseExt1)
	writeLen16(&ext, 0)
	writeServerNameExt(&ext, host)
	writeLen16(&ext, 0x0017)
	writeLen16(&ext, 0)
	writeLen16(&ext, 0xFF01)
	writeLen16(&ext, 1)
	ext.WriteByte(0x00)
	writeSupportedGroupsExt(&ext, true)
	writeLen16(&ext, 0x000B)
	writeLen16(&ext, 2)
	ext.WriteByte(1)
	ext.WriteByte(0x00)
	writeLen16(&ext, 0x0023)
	writeLen16(&ext, 0)
	writeALPNExt(&ext)
	writeLen16(&ext, 0x0005)
	// RFC 6066 status_request: OCSP, then two empty lists. A 1-byte body is
	// short by four and every strict parser stops there.
	writeLen16(&ext, 5)
	ext.WriteByte(0x01)
	writeLen16(&ext, 0)
	writeLen16(&ext, 0)
	writeSigAlgsExt(&ext, chromeSigAlgs)
	writeLen16(&ext, 0x0012)
	writeLen16(&ext, 0)
	writeSupportedVersionsExt(&ext, true)
	writeKeyShareExt(&ext, true)
	writeLen16(&ext, 0x002D)
	writeLen16(&ext, 2)
	ext.WriteByte(1)
	ext.WriteByte(0x01)
	writeLen16(&ext, 0x001B)
	writeLen16(&ext, 3)
	// RFC 8879: one byte of list length, then the brotli algorithm id.
	ext.WriteByte(0x02)
	writeLen16(&ext, 0x0002)
	writeLen16(&ext, 0x4469)
	writeLen16(&ext, 4)
	writeLen16(&ext, 2)
	ext.WriteByte(0x68)
	ext.WriteByte(0x32)
	writeLen16(&ext, greaseExt2)
	writeLen16(&ext, 0)
	pad := rng.Intn(49)
	writeLen16(&ext, 0x0015)
	writeLen16(&ext, pad)
	for i := 0; i < pad; i++ {
		ext.WriteByte(0x00)
	}
	writeLen16(&hs, ext.Len())
	hs.Write(ext.Bytes())
	return wrapHandshake(hs.Bytes())
}

// buildFirefoxHello builds a Firefox-shaped TLS 1.2 ClientHello (NSS library,
// Firefox 120+). Key differences from Chrome: no GREASE anywhere, NSS cipher
// ordering with older ECDHE CBC suites, delegated_credentials extension,
// padding to 512-byte boundary, no compress_certificate/ALPS.
func buildFirefoxHello(host string) []byte {
	var hs bytes.Buffer
	writeLen16(&hs, 0x0303)
	hs.Write(randomBytes(32))
	sid := randomBytes(32)
	hs.WriteByte(byte(len(sid)))
	hs.Write(sid)
	ciphers := []uint16{
		0x1301, 0x1302, 0x1303,
		0xC02B, 0xC02C, 0xC02F, 0xC030,
		0xCCA9, 0xCCA8,
		0xC008, 0xC009, 0xC00A,
		0xC013, 0xC014, 0xC012,
		0x009C, 0x009D, 0x002F, 0x0035, 0x000A,
	}
	var cs bytes.Buffer
	for _, c := range ciphers {
		writeLen16(&cs, c)
	}
	writeLen16(&hs, cs.Len())
	hs.Write(cs.Bytes())
	hs.WriteByte(0x01)
	hs.WriteByte(0x00)

	var ext bytes.Buffer
	writeServerNameExt(&ext, host)
	writeLen16(&ext, 0x0017)
	writeLen16(&ext, 0)
	writeLen16(&ext, 0xFF01)
	writeLen16(&ext, 1)
	ext.WriteByte(0x00)
	writeSupportedGroupsExt(&ext, false)
	writeLen16(&ext, 0x000B)
	writeLen16(&ext, 2)
	ext.WriteByte(1)
	ext.WriteByte(0x00)
	writeLen16(&ext, 0x0023)
	writeLen16(&ext, 0)
	writeALPNExt(&ext)
	writeLen16(&ext, 0x0005)
	// RFC 6066 status_request: OCSP, then two empty lists. A 1-byte body is
	// short by four and every strict parser stops there.
	writeLen16(&ext, 5)
	ext.WriteByte(0x01)
	writeLen16(&ext, 0)
	writeLen16(&ext, 0)
	writeKeyShareExt(&ext, false)
	writeSupportedVersionsExt(&ext, false)
	writeSigAlgsExt(&ext, firefoxSigAlgs)
	writeLen16(&ext, 0x0012)
	writeLen16(&ext, 0)
	writeDelegatedCredentialsExt(&ext)
	writeLen16(&ext, 0x002D)
	writeLen16(&ext, 2)
	ext.WriteByte(1)
	ext.WriteByte(0x01)
	padTo512(&ext, hs.Len())
	writeLen16(&hs, ext.Len())
	hs.Write(ext.Bytes())
	return wrapHandshake(hs.Bytes())
}

// buildSafariHello builds a Safari-shaped TLS 1.2 ClientHello (Apple
// SecureTransport, Safari 16). Key differences from Chrome: no GREASE, Apple
// cipher ordering with legacy DHE/CBC suites, supported_versions includes
// TLS 1.1, no compress_certificate/ALPS/padding.
func buildSafariHello(host string) []byte {
	var hs bytes.Buffer
	writeLen16(&hs, 0x0303)
	hs.Write(randomBytes(32))
	sid := randomBytes(32)
	hs.WriteByte(byte(len(sid)))
	hs.Write(sid)
	ciphers := []uint16{
		0x1301, 0x1302, 0x1303,
		0xC02C, 0xC02B, 0xC030, 0xC02F,
		0xCCA9, 0xCCA8,
		0xC024, 0xC023, 0xC00A, 0xC009, 0xC008,
		0xC028, 0xC027,
		0xC014, 0xC013, 0xC012,
		0x009D, 0x009C, 0x003D, 0x003C,
		0x0035, 0x002F, 0x00FF,
	}
	var cs bytes.Buffer
	for _, c := range ciphers {
		writeLen16(&cs, c)
	}
	writeLen16(&hs, cs.Len())
	hs.Write(cs.Bytes())
	hs.WriteByte(0x01)
	hs.WriteByte(0x00)

	var ext bytes.Buffer
	writeServerNameExt(&ext, host)
	writeLen16(&ext, 0x000B)
	writeLen16(&ext, 2)
	ext.WriteByte(1)
	ext.WriteByte(0x00)
	writeLen16(&ext, 0x0017)
	writeLen16(&ext, 0)
	writeLen16(&ext, 0xFF01)
	writeLen16(&ext, 1)
	ext.WriteByte(0x00)
	writeSupportedGroupsExtSafari(&ext)
	writeLen16(&ext, 0x0023)
	writeLen16(&ext, 0)
	writeLen16(&ext, 0x0005)
	// RFC 6066 status_request: OCSP, then two empty lists. A 1-byte body is
	// short by four and every strict parser stops there.
	writeLen16(&ext, 5)
	ext.WriteByte(0x01)
	writeLen16(&ext, 0)
	writeLen16(&ext, 0)
	writeSigAlgsExt(&ext, safariSigAlgs)
	writeSupportedVersionsExtSafari(&ext)
	writeLen16(&ext, 0x002D)
	writeLen16(&ext, 2)
	ext.WriteByte(1)
	ext.WriteByte(0x01)
	writeKeyShareExtSafari(&ext)
	writeALPNExt(&ext)
	writeLen16(&ext, 0x0012)
	writeLen16(&ext, 0)
	writeLen16(&hs, ext.Len())
	hs.Write(ext.Bytes())
	return wrapHandshake(hs.Bytes())
}

// wrapHandshake wraps a ClientHello body in the handshake header: type 0x01,
// 24-bit length, body.
func wrapHandshake(body []byte) []byte {
	var rec bytes.Buffer
	rec.WriteByte(0x01)
	writeUint24BE(&rec, len(body))
	rec.Write(body)
	return rec.Bytes()
}

// padTo512 adds a padding extension (0x0015) that pads the total ClientHello
// (handshake header + body so far + extensions so far) up to the next 512-byte
// boundary. Firefox pads to 512 to avoid length-based fingerprinting.
func padTo512(ext *bytes.Buffer, bodyLen int) {
	const target = 512
	// Estimate total: 4 (handshake header) + bodyLen + 2 (ext length) + ext.Len() + 4 (padding ext header) + pad
	current := 4 + bodyLen + 2 + ext.Len() + 4
	pad := target - (current % target)
	if pad < 0 {
		pad += target
	}
	if pad > 0xFFFF {
		pad = 0xFFFF
	}
	writeLen16(ext, 0x0015)
	writeLen16(ext, pad)
	for i := 0; i < pad; i++ {
		ext.WriteByte(0x00)
	}
}

// writeDelegatedCredentialsExt writes the delegated_credentials extension
// (0x0022) with a Firefox-compatible signature-algorithm list.
func writeDelegatedCredentialsExt(b *bytes.Buffer) {
	algs := []uint16{0x0403, 0x0503, 0x0604, 0x0804, 0x0805, 0x0806, 0x0401, 0x0501, 0x0201}
	var list bytes.Buffer
	for _, a := range algs {
		writeLen16(&list, a)
	}
	writeLen16(b, 0x0022)
	writeLen16(b, list.Len()+2)
	writeLen16(b, list.Len())
	b.Write(list.Bytes())
}

// writeSupportedGroupsExtSafari writes supported_groups with x25519,
// secp256r1, secp384r1, secp521r1 (Safari adds secp521r1) and no GREASE.
func writeSupportedGroupsExtSafari(b *bytes.Buffer) {
	groups := []uint16{0x001D, 0x0017, 0x0018, 0x0019}
	var list bytes.Buffer
	for _, g := range groups {
		writeLen16(&list, g)
	}
	writeLen16(b, 0x000A)
	writeLen16(b, list.Len()+2)
	writeLen16(b, list.Len())
	b.Write(list.Bytes())
}

// writeSupportedVersionsExtSafari writes supported_versions with TLS 1.3,
// 1.2, and 1.1 (Safari still advertises 0x0302), no GREASE.
func writeSupportedVersionsExtSafari(b *bytes.Buffer) {
	var list bytes.Buffer
	writeLen16(&list, 0x0304)
	writeLen16(&list, 0x0303)
	writeLen16(&list, 0x0302)
	writeLen16(b, 0x002B)
	writeLen16(b, list.Len()+1)
	b.WriteByte(byte(list.Len()))
	b.Write(list.Bytes())
}

// writeKeyShareExtSafari writes key_share with x25519 and secp256r1 (Safari
// sends both), no GREASE.
func writeKeyShareExtSafari(b *bytes.Buffer) {
	var ks bytes.Buffer
	writeLen16(&ks, 0x001D)
	writeLen16(&ks, 32)
	ks.Write(randomBytes(32))
	writeLen16(&ks, 0x0017)
	writeLen16(&ks, 65) // secp256r1 uncompressed point: 1 prefix + 32x + 32y
	ks.WriteByte(0x04)
	ks.Write(randomBytes(64))
	writeLen16(b, 0x0033)
	writeLen16(b, ks.Len()+2)
	writeLen16(b, ks.Len())
	b.Write(ks.Bytes())
}

// Signature-algorithm lists per browser. Chrome uses a trimmed list; Firefox
// uses the NSS ordering (eCDSSA-P256-SHA256 first); Safari uses Apple's
// ordering with SHA1 fallbacks.
var (
	chromeSigAlgs  = []uint16{0x0403, 0x0804, 0x0401, 0x0203, 0x0201, 0x0804, 0x0805, 0x0806, 0x0201}
	firefoxSigAlgs = []uint16{0x0403, 0x0503, 0x0604, 0x0804, 0x0805, 0x0806, 0x0401, 0x0501, 0x0201, 0x0203}
	safariSigAlgs  = []uint16{0x0403, 0x0804, 0x0401, 0x0503, 0x0805, 0x0501, 0x0806, 0x0601, 0x0201}
)

func writeServerNameExt(b *bytes.Buffer, host string) {
	// server_name extension: type 0x0000, server_name list, host_name.
	var name bytes.Buffer
	name.WriteByte(0x00) // host_name type
	writeLen16(&name, len(host))
	name.WriteString(host)
	var list bytes.Buffer
	writeLen16(&list, len(name.Bytes()))
	list.Write(name.Bytes())
	writeLen16(b, 0x0000)
	// list already carries the server_name_list length; wrapping it in another
	// one made the extension unreadable to every TLS parser.
	writeLen16(b, list.Len())
	b.Write(list.Bytes())
}

func writeSupportedGroupsExt(b *bytes.Buffer, grease bool) {
	groups := []uint16{0x001D, 0x0017, 0x0018} // x25519, secp256r1, secp384r1
	var list bytes.Buffer
	if grease {
		writeLen16(&list, greaseValue())
	}
	for _, g := range groups {
		writeLen16(&list, g)
	}
	writeLen16(b, 0x000A)
	writeLen16(b, list.Len()+2)
	writeLen16(b, list.Len())
	b.Write(list.Bytes())
}

func writeALPNExt(b *bytes.Buffer) {
	// ALPN: h2 (2 bytes + len) + http/1.1
	var protos bytes.Buffer
	for _, p := range []string{"h2", "http/1.1"} {
		protos.WriteByte(byte(len(p)))
		protos.WriteString(p)
	}
	writeLen16(b, 0x0010)
	writeLen16(b, protos.Len()+2)
	writeLen16(b, protos.Len())
	b.Write(protos.Bytes())
}

func writeSigAlgsExt(b *bytes.Buffer, algs []uint16) {
	var list bytes.Buffer
	for _, a := range algs {
		writeLen16(&list, a)
	}
	writeLen16(b, 0x000D)
	writeLen16(b, list.Len()+2)
	writeLen16(b, list.Len())
	b.Write(list.Bytes())
}

func writeSupportedVersionsExt(b *bytes.Buffer, grease bool) {
	var list bytes.Buffer
	if grease {
		writeLen16(&list, greaseValue())
	}
	writeLen16(&list, 0x0304)
	writeLen16(&list, 0x0303)
	writeLen16(b, 0x002B)
	writeLen16(b, list.Len()+1)
	b.WriteByte(byte(list.Len()))
	b.Write(list.Bytes())
}

func writeKeyShareExt(b *bytes.Buffer, grease bool) {
	var ks bytes.Buffer
	if grease {
		// A GREASE share carries one byte, as Chrome sends it; a zero-length
		// key_exchange is rejected outright by strict parsers.
		writeLen16(&ks, greaseValue())
		writeLen16(&ks, 1)
		ks.WriteByte(0x00)
	}
	writeLen16(&ks, 0x001D) // x25519
	writeLen16(&ks, 32)
	ks.Write(randomBytes(32))
	writeLen16(b, 0x0033)
	writeLen16(b, ks.Len()+2)
	writeLen16(b, ks.Len())
	b.Write(ks.Bytes())
}

// ---- DNS query (EDNS0) ----

// dnsPacket builds a DNS query (flags 0x0100, RD, 1 query, EDNS0 OPT) for the
// given domain and returns it as a "<r 2><b 0xHEX>" CPS tag. Ported from
// pumbaX gen_dns.
func dnsPacket(domain string) string {
	var b bytes.Buffer
	// ID (random 16-bit)
	writeLen16(&b, uint16(rng.Intn(65536)))
	// flags: 0x0100 (RD)
	writeLen16(&b, 0x0100)
	// counts: 1 query, 0 answer, 0 authority, 1 additional (OPT)
	writeLen16(&b, 1)
	writeLen16(&b, 0)
	writeLen16(&b, 0)
	writeLen16(&b, 1)
	// Question: QNAME (label-encoded), qtype (A/AAAA/MX weighted), qclass IN
	writeQName(&b, domain)
	// Weighted qtype: A 60%, AAAA 30%, MX 10%
	r := rng.Intn(100)
	qtype := uint16(1) // A
	if r >= 60 && r < 90 {
		qtype = 28 // AAAA
	} else if r >= 90 {
		qtype = 15 // MX
	}
	writeLen16(&b, qtype)
	writeLen16(&b, 1) // IN
	// EDNS0 OPT RR: name=0 (root), type=0x0029, class=udp_size, TTL=DO bit, rdlen=0
	b.WriteByte(0x00) // root name
	writeLen16(&b, 0x0029)
	udpSize := 1232
	if rng.Intn(2) == 0 {
		udpSize = 4096
	}
	writeLen16(&b, uint16(udpSize))
	// TTL: DO bit is in the high 16 bits (0x8000), low 16 = 0
	writeLen16(&b, 0x8000)
	writeLen16(&b, 0x0000)
	writeLen16(&b, 0) // rdlen 0
	return dnsTag(b.Bytes())
}

func writeQName(b *bytes.Buffer, domain string) {
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			continue
		}
		b.WriteByte(byte(len(label)))
		b.WriteString(label)
	}
	b.WriteByte(0x00) // root
}

// ---- SIP REGISTER ----

func randomPrivateIP() string {
	switch rng.Intn(3) {
	case 0:
		return fmt.Sprintf("10.%d.%d.%d", rng.Intn(256), rng.Intn(256), rng.Intn(254)+1)
	case 1:
		return fmt.Sprintf("172.%d.%d.%d", rng.Intn(16)+16, rng.Intn(256), rng.Intn(254)+1)
	default:
		return fmt.Sprintf("192.168.%d.%d", rng.Intn(256), rng.Intn(254)+1)
	}
}

// ---- QUIC Initial (plain, no crypto lib) ----

// writeVarint writes a QUIC variable-length integer (RFC 9000 §16) using the
// minimal encoding for the value.
func writeVarint(b *bytes.Buffer, v int) {
	switch {
	case v < 64:
		b.WriteByte(byte(v))
	case v < 16384:
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(v)|0x4000)
		b.Write(buf[:])
	case v < 1073741824:
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(v)|0x80000000)
		b.Write(buf[:])
	default:
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(v)|0xC000000000000000)
		b.Write(buf[:])
	}
}
