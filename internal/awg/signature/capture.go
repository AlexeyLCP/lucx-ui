// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package signature

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
)

// CaptureResult holds the I1-I5 packet strings captured from a real host.
// I1 is the QUIC Initial we send (carrying a TLS ClientHello with SNI=domain);
// I2-I5 are the server's reply packets read off the same UDP socket.
type CaptureResult struct {
	I1, I2, I3, I4, I5 string
}

// QUIC v1 Initial salt (RFC 9001).
var quicV1Salt = []byte{0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17, 0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a}

const (
	maxPackets  = 5 // I1-I5
	quicPort    = "443"
	readTimeout = 5 * time.Second
	minPackets  = 2 // need at least our Initial + 1 reply
)

// randomBytes returns n cryptographically-strong random bytes.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

// Capture sends a QUIC v1 Initial (carrying a TLS ClientHello with SNI=domain)
// to the host on UDP 443, reads the server's reply packets, and returns them
// as AmneziaWG CPS strings (I1 = what we sent, I2-I5 = replies). Ported from
// hoaxisr/awg-manager internal/signature/capture.go — pure Go, no libpcap.
//
// Returns an error when the host doesn't speak QUIC (no reply within the
// timeout) — AWG can only mimic QUIC-fronted hosts. TLS capture is not
// supported (TLS signatures are incompatible with AWG and crash it, per
// hoaxisr).
func Capture(domain string) (CaptureResult, error) {
	host := normalizeDomain(domain)
	if host == "" {
		return CaptureResult{}, errors.New("signature: empty domain")
	}
	ip, err := resolveHost(host)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("signature: resolve %q: %w", host, err)
	}
	packets, err := captureQUIC(host, ip)
	if err != nil {
		return CaptureResult{}, err
	}
	return fillPackets(packets), nil
}

// normalizeDomain strips scheme/path/port from the user input, leaving the
// bare domain (google.com from https://google.com:443/path).
func normalizeDomain(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			return u.Hostname()
		}
	}
	if i := strings.LastIndexByte(s, ':'); i > 0 && !strings.Contains(s[i+1:], ".") {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '/'); i > 0 {
		s = s[:i]
	}
	return s
}

// resolveHost resolves a domain to a single IPv4 (AWG QUIC fronting is IPv4).
func resolveHost(domain string) (string, error) {
	resolver := net.Resolver{}
	ips, err := resolver.LookupIPAddr(context.Background(), domain)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			return ip.IP.To4().String(), nil
		}
	}
	if len(ips) > 0 {
		return ips[0].IP.String(), nil
	}
	return "", errors.New("no IP found")
}

// captureQUIC dials UDP 443, sends a QUIC Initial (with a TLS ClientHello
// SNI=host), then reads reply packets. Returns the sent Initial first, then
// up to maxPackets-1 replies.
func captureQUIC(host, ip string) ([][]byte, error) {
	initial, err := buildQUICInitial(host)
	if err != nil {
		return nil, fmt.Errorf("signature: build QUIC initial: %w", err)
	}
	conn, err := (&net.Dialer{Timeout: readTimeout}).DialContext(context.Background(), "udp", net.JoinHostPort(ip, quicPort))
	if err != nil {
		return nil, fmt.Errorf("signature: dial %s:443: %w", ip, err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))

	if _, err := conn.Write(initial); err != nil {
		return nil, fmt.Errorf("signature: write initial: %w", err)
	}
	packets := [][]byte{initial}
	buf := make([]byte, 2048)
	for len(packets) < maxPackets {
		n, err := conn.Read(buf)
		if err != nil {
			break // timeout / closed: keep what we have
		}
		if n <= 0 {
			continue
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		packets = append(packets, pkt)
	}
	if len(packets) < minPackets {
		return nil, fmt.Errorf("signature: %s did not reply on QUIC 443 (got %d packet(s)) — host may not support QUIC", host, len(packets))
	}
	return packets, nil
}

// fillPackets converts raw packet bytes into AmneziaWG CPS "<b 0xHEX>" strings,
// keeping whole packets while the set stays inside what `awg show` can read
// back. A packet that no longer fits ends the set: truncating one would put a
// malformed packet on the wire, a worse signature than a shorter burst.
func fillPackets(packets [][]byte) CaptureResult {
	res := CaptureResult{}
	fields := [5]*string{&res.I1, &res.I2, &res.I3, &res.I4, &res.I5}
	budget := awg.IBytesBudget(awg.BaselineIfname, false)
	for i, pkt := range packets {
		if i >= maxPackets {
			break
		}
		*fields[i] = "<b 0x" + hex.EncodeToString(pkt) + ">"
		if awg.IBytes(res.I1, res.I2, res.I3, res.I4, res.I5) > budget {
			*fields[i] = ""
			break
		}
	}
	return res
}

// buildQUICInitial assembles a QUIC v1 Long-Header Initial packet carrying a
// TLS 1.3 ClientHello (SNI=host), encrypts the CRYPTO frame payload with
// AES-128-GCM (initial keys derived from the random DCID via HKDF-SHA256 per
// RFC 9001 §5.2), and applies header protection (RFC 9001 §5.4). Ported from
// hoaxisr/awg-manager internal/signature/capture.go.
func buildQUICInitial(host string) ([]byte, error) {
	// buildTLSClientHello returns the ClientHello handshake message (starts with
	// type 0x01), which is what the CRYPTO frame carries — no record to strip.
	chPayload, err := buildTLSClientHello(host)
	if err != nil {
		return nil, err
	}
	dcid := make([]byte, 8)
	if _, err := rand.Read(dcid); err != nil {
		return nil, err
	}
	return BuildQUICInitial(dcid, chPayload)
}

// BuildQUICInitial assembles a genuine QUIC v1 Initial around an already-built
// ClientHello: AEAD-sealed and header-protected under keys derived from dcid,
// so an observer that derives the same keys can decrypt it like a real one.
func BuildQUICInitial(dcid, chPayload []byte) ([]byte, error) {
	// Derive initial keys from dcid (RFC 9001 §5.2).
	initialSecret, _ := hkdf.Extract(sha256.New, dcid, quicV1Salt)
	clientSecret := hkdfExpandLabel(initialSecret, "client in", nil, 32)
	clientKey := hkdfExpandLabel(clientSecret, "quic key", nil, 16) // AES-128
	clientIV := hkdfExpandLabel(clientSecret, "quic iv", nil, 12)
	clientHP := hkdfExpandLabel(clientSecret, "quic hp", nil, 16) // header protection

	// CRYPTO frame: type 0x06, offset 0x00, var-int length, ClientHello payload.
	var crypto bytes.Buffer
	crypto.WriteByte(0x06)
	crypto.WriteByte(0x00) // offset = 0 (1-byte var-int form)
	crypto.Write(appendVarint(len(chPayload)))
	crypto.Write(chPayload)

	const pnLen = 4
	pn := []byte{0x00, 0x00, 0x00, 0x00} // packet number 0

	// Unprotected header (AAD), following hoaxisr byte order:
	// 0xC3 (long | fixed | initial | pn_len=4), QUIC v1, dcid_len(8) + dcid,
	// scid_len(0), token_len(0), length var-int, packet number.
	block, _ := aes.NewCipher(clientKey)
	gcm, _ := cipher.NewGCM(block)
	// RFC 9001 §5.3: the AEAD seals the frames alone. The packet number is
	// associated data only — sealing it too opens the frames with PADDING.
	plaintext := crypto.Bytes()
	// Pad plaintext so the full packet reaches 1200 bytes (QUIC minimum Initial).
	// headerEstimate = 1(flags) + 4(version) + 1(dcid_len) + 8(dcid) + 1(scid_len) +
	//                   1(token_len) + 2(length var-int, our sizes fit 2 bytes) + 4(pn)
	headerEstimate := 1 + 4 + 1 + len(dcid) + 1 + 1 + 2 + pnLen
	minPayload := 1200 - headerEstimate - gcm.Overhead() // subtract 16-byte AEAD tag
	if len(plaintext) < minPayload {
		pad := make([]byte, minPayload-len(plaintext))
		plaintext = append(plaintext, pad...)
	}
	// lengthVal covers pn + padded plaintext + AEAD tag (16).
	lengthVal := pnLen + len(plaintext) + gcm.Overhead()

	var header bytes.Buffer
	header.WriteByte(0xC3)
	header.Write([]byte{0x00, 0x00, 0x00, 0x01}) // QUIC v1
	header.WriteByte(byte(len(dcid)))
	header.Write(dcid)
	header.WriteByte(0x00) // SCID length = 0 (hoaxisr sends no SCID)
	header.WriteByte(0x00) // token length = 0
	header.Write(appendVarint(lengthVal))
	header.Write(pn) // unprotected pn (masked later)

	// AES-128-GCM: nonce = clientIV XOR packet_number (pn=0 → nonce = IV).
	nonce := make([]byte, 12)
	copy(nonce, clientIV)
	ciphertext := gcm.Seal(nil, nonce, plaintext, header.Bytes())

	// Header protection (RFC 9001 §5.4): mask = AES-ECB(clientHP, sample),
	// sample = first 16 bytes of ciphertext. XOR mask into the form byte (low
	// 4 bits for long headers) and the 4 pn bytes.
	protected := append(header.Bytes(), ciphertext...)
	pnOffset := header.Len() - pnLen
	if len(ciphertext) >= 16 {
		mask := aesECB(clientHP, ciphertext[:16])
		// Form byte (position 0) is masked on its low 4 bits (long header);
		// the pn bytes (pnOffset..pnOffset+pnLen) are masked by mask[1..pnLen].
		// NOTE: the form byte is the first byte of the header, NOT pnOffset.
		protected[0] ^= mask[0] & 0x0F
		for i := 0; i < pnLen; i++ {
			protected[pnOffset+i] ^= mask[1+i]
		}
	}
	return protected, nil
}

// buildTLSClientHello builds a minimal TLS 1.3 ClientHello handshake message
// (NOT wrapped in a TLS record — QUIC's CRYPTO frame carries the handshake
// message directly) for the given SNI host. Kept small (~250 bytes) so it fits
// inside a single 1200-byte QUIC Initial (the QUIC minimum), which a real QUIC
// server (google/cloudflare) accepts. A crypto/tls-driven ClientHello would be
// ~1480 bytes (session_ticket, key_share, full extension set) and overflow the
// 1200-byte Initial, so we build a minimal Chrome-like ClientHello by hand:
// SNI, supported_versions (TLS 1.3), supported_groups (x25519), key_share
// (x25519 32B), signature_algorithms, ALPN (h3), padded.
func buildTLSClientHello(host string) ([]byte, error) {
	// Handshake body (ClientHello fields after the type + 24-bit length).
	var hs bytes.Buffer
	// legacy_version = 0x0303 (TLS 1.2, carried in TLS 1.3 for compat).
	writeUint16(&hs, 0x0303)
	// random (32 bytes).
	hs.Write(randomBytes(32))
	// legacy_session_id (32 random bytes + 1-byte length) — Chrome keeps a
	// non-empty session id for middlebox compat.
	sid := randomBytes(32)
	hs.WriteByte(byte(len(sid)))
	hs.Write(sid)
	// legacy_cipher_suites: TLS_AES_128_GCM_SHA256 (0x1301) +
	// TLS_AES_256_GCM_SHA384 (0x1302) + TLS_CHACHA20_POLY1305_SHA256 (0x1303).
	ciphers := []uint16{0x1301, 0x1302, 0x1303}
	writeUint16(&hs, len(ciphers)*2)
	for _, c := range ciphers {
		writeUint16(&hs, int(c))
	}
	// legacy_compression_methods: 1 byte (null only).
	hs.WriteByte(0x01)
	hs.WriteByte(0x00)

	// Extensions.
	var ext bytes.Buffer
	// server_name (SNI).
	writeUint16(&ext, 0x0000)
	var sni bytes.Buffer
	sni.WriteByte(0x00) // host_name type
	writeUint16(&sni, len(host))
	sni.WriteString(host)
	writeUint16(&ext, 2+len(sni.Bytes())) // 2-byte server_name list length + name
	writeUint16(&ext, len(sni.Bytes()))
	ext.Write(sni.Bytes())
	// supported_versions (0x002b): TLS 1.3 (0x0304).
	writeUint16(&ext, 0x002B)
	writeUint16(&ext, 3)
	ext.WriteByte(2) // 2 bytes of versions
	writeUint16(&ext, 0x0304)
	// supported_groups (0x000a): x25519 (0x001d).
	writeUint16(&ext, 0x000A)
	writeUint16(&ext, 3)
	ext.WriteByte(2)
	writeUint16(&ext, 0x001D)
	// signature_algorithms (0x000d): ecdsa_secp256r1_sha256 (0x0403),
	// rsa_pss_rsae_sha256 (0x0804), rsa_pkcs1_sha256 (0x0401).
	algs := []uint16{0x0403, 0x0804, 0x0401}
	writeUint16(&ext, 0x000D)
	writeUint16(&ext, 2+len(algs)*2)
	writeUint16(&ext, len(algs)*2)
	for _, a := range algs {
		writeUint16(&ext, int(a))
	}
	// key_share (0x0033): x25519 32-byte random.
	var ks bytes.Buffer
	writeUint16(&ks, 0x001D) // x25519
	writeUint16(&ks, 32)
	ks.Write(randomBytes(32))
	writeUint16(&ext, 0x0033)
	writeUint16(&ext, 2+len(ks.Bytes())) // 2-byte list length + entry
	writeUint16(&ext, len(ks.Bytes()))
	ext.Write(ks.Bytes())
	// ALPN (0x0010): h3.
	var protos bytes.Buffer
	protos.WriteByte(2) // length of "h3"
	protos.WriteString("h3")
	writeUint16(&ext, 0x0010)
	writeUint16(&ext, 2+len(protos.Bytes()))
	writeUint16(&ext, len(protos.Bytes()))
	ext.Write(protos.Bytes())
	// psk_key_exchange_modes (0x002d): psk_dhe_ke (1).
	writeUint16(&ext, 0x002D)
	writeUint16(&ext, 2)
	ext.WriteByte(1)
	ext.WriteByte(0x01)

	// Append extensions block.
	writeUint16(&hs, ext.Len())
	hs.Write(ext.Bytes())

	// Wrap in a handshake header: type 0x01 (ClientHello), 24-bit length.
	var msg bytes.Buffer
	msg.WriteByte(0x01)
	writeUint24(&msg, hs.Len())
	msg.Write(hs.Bytes())
	return msg.Bytes(), nil
}

// writeUint16 writes a big-endian uint16 (accepts int or uint16).
func writeUint16(b *bytes.Buffer, v int) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(v))
	b.Write(buf[:])
}

// writeUint24 writes a 3-byte big-endian value (TLS handshake length).
func writeUint24(b *bytes.Buffer, v int) {
	b.Write([]byte{byte(v >> 16), byte(v >> 8), byte(v)})
}

// hkdfExpandLabel implements HKDF-Expand-Label (RFC 8446 §7.1) with the TLS
// 1.3 "tls13 " label prefix.
func hkdfExpandLabel(secret []byte, label string, context []byte, length int) []byte {
	fullLabel := "tls13 " + label
	var info bytes.Buffer
	_ = binary.Write(&info, binary.BigEndian, uint16(length))
	info.WriteByte(byte(len(fullLabel)))
	info.WriteString(fullLabel)
	info.WriteByte(byte(len(context)))
	info.Write(context)
	out, _ := hkdf.Expand(sha256.New, secret, info.String(), length)
	return out
}

// appendVarint writes a QUIC variable-length integer (RFC 9000 §16) using the
// minimal encoding.
func appendVarint(v int) []byte {
	switch {
	case v < 64:
		return []byte{byte(v)}
	case v < 16384:
		return []byte{byte(v>>8) | 0x40, byte(v)}
	case v < 1073741824:
		return []byte{byte(v>>24) | 0x80, byte(v >> 16), byte(v >> 8), byte(v)}
	default:
		return []byte{byte(v>>56) | 0xC0, byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
}

// aesECB encrypts a single 16-byte block with AES-ECB (QUIC header protection mask).
func aesECB(key, block []byte) []byte {
	c, _ := aes.NewCipher(key)
	out := make([]byte, len(block))
	c.Encrypt(out, block)
	return out
}
