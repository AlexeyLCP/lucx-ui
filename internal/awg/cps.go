// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// CPSProfile defines a mimicry profile for CPS packet generation.
type CPSProfile string

const (
	CPSProfileQUIC CPSProfile = "quic"
	CPSProfileSIP  CPSProfile = "sip"
	CPSProfileDNS  CPSProfile = "dns"
)

// GenerateCPS generates CPS (Connection Proxy Signatures) packets for AWG obfuscation.
// level: 1 = no CPS, 2 = I1 only, 3 = I1-I5 full chain.
// profile: "quic", "sip", or "dns".
// Returns up to 5 signature strings (I1-I5), empty strings for unused slots.
func GenerateCPS(level int, profile CPSProfile) (i1, i2, i3, i4, i5 string) {
	if level < 2 {
		return // level 1 = no CPS
	}
	maxSignatures := 5
	if level == 2 {
		maxSignatures = 1 // I1 only
	}

	switch profile {
	case CPSProfileQUIC:
		i1 = generateQUICInitial()
		if maxSignatures >= 5 {
			i2 = generateQUICShortHeader()
			i3 = generateQUICShortHeader()
			i4 = generateQUICShortHeader()
			i5 = generateQUICShortHeader()
		}
	case CPSProfileSIP:
		i1 = generateSIPRegister()
		// SIP profile only uses I1
	case CPSProfileDNS:
		i1 = generateDNSQuery("www.googleapis.com")
		if maxSignatures >= 5 {
			i2 = generateDNSQuery("android.clients.google.com")
			i3 = generateDNSQuery("mtalk.google.com")
			i4 = generateDNSQuery("cloudconfig.googleapis.com")
			i5 = generateDNSQuery("connectivitycheck.gstatic.com")
		}
	}
	return
}

// QUIC Initial packet (~1200 bytes, mimics Chrome QUIC Initial)
func generateQUICInitial() string {
	b := make([]byte, 1200)
	rand.Read(b)

	// QUIC Long Header type = 0xC0 or 0xC3 (Initial packet)
	headerByte := byte(0xC0)
	if randInt(0, 1) == 1 {
		headerByte = 0xC3
	}
	b[0] = headerByte

	// QUIC version (0x00000001 or 0xff000000 + random)
	b[1] = 0x00
	b[2] = 0x00
	b[3] = 0x00
	b[4] = 0x01

	// Random DCID (Destination Connection ID) length
	dcidLen := randInt(8, 20)
	b[5] = byte(dcidLen)
	// Random SCID (Source Connection ID) length
	scidLen := randInt(0, 20)
	offset := 6 + dcidLen
	if offset < len(b) {
		b[offset] = byte(scidLen)
	}

	// Token length = 0 (empty token for Initial)
	tokenOffset := offset + 1 + scidLen
	if tokenOffset < len(b)-10 {
		// Write varint length prefix for token (0 = no token)
		b[tokenOffset] = 0x00
	}

	return hex.EncodeToString(b)
}

// QUIC Short Header packet (40-90 bytes payload, mimics 1-RTT data)
func generateQUICShortHeader() string {
	size := randInt(40, 90)
	b := make([]byte, size)
	rand.Read(b)

	// QUIC Short Header: top bit = 0, bit 6 = fixed (1), bit 5 = spin
	b[0] = 0x40 // 01000000
	if randInt(0, 1) == 1 {
		b[0] |= 0x20 // set spin bit randomly
	}
	if randInt(0, 1) == 1 {
		b[0] |= 0x08 // key phase bit
	}

	// Fill with encrypted-looking random data
	rand.Read(b[1:])

	return hex.EncodeToString(b)
}

// SIP REGISTER request (realistic headers mimicking Linphone/Zoiper)
func generateSIPRegister() string {
	callID := hex.EncodeToString(randomBytes(16))
	branch := "z9hG4bK-" + hex.EncodeToString(randomBytes(8))
	tag := hex.EncodeToString(randomBytes(4))

	userAgents := []string{
		"Linphone/5.2.5 (Linux)",
		"Zoiper 2.10.17",
		"MicroSIP/3.21.3",
		"Bria 6.5.0",
		"PortSIP/16.4",
	}
	ua := userAgents[randInt(0, len(userAgents)-1)]

	return fmt.Sprintf(
		"REGISTER sip:sip.example.com SIP/2.0\r\n"+
			"Via: SIP/2.0/UDP 192.168.1.%d:5060;branch=%s\r\n"+
			"Max-Forwards: 70\r\n"+
			"From: <sip:user%d@sip.example.com>;tag=%s\r\n"+
			"To: <sip:user%d@sip.example.com>\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: %d REGISTER\r\n"+
			"Contact: <sip:user%d@192.168.1.%d:5060>\r\n"+
			"User-Agent: %s\r\n"+
			"Expires: 3600\r\n"+
			"Content-Length: 0\r\n\r\n",
		randInt(1, 254), branch,
		randInt(1, 9999), tag,
		randInt(1, 9999),
		callID,
		randInt(1, 99999),
		randInt(1, 9999), randInt(1, 254),
		ua,
	)
}

// DNS query in wire format with EDNS0
func generateDNSQuery(domain string) string {
	var b []byte

	// DNS Header
	txID := make([]byte, 2)
	rand.Read(txID)
	b = append(b, txID...)   // Transaction ID
	b = append(b, 0x01, 0x00) // Flags: standard query, recursion desired
	b = append(b, 0x00, 0x01) // QDCOUNT = 1
	b = append(b, 0x00, 0x00) // ANCOUNT = 0
	b = append(b, 0x00, 0x00) // NSCOUNT = 0
	b = append(b, 0x00, 0x01) // ARCOUNT = 1 (EDNS0)

	// Question section
	for _, part := range strings.Split(domain, ".") {
		b = append(b, byte(len(part)))
		b = append(b, []byte(part)...)
	}
	b = append(b, 0x00) // Root label

	// QTYPE = A (1), QCLASS = IN (1)
	b = append(b, 0x00, 0x01) // Type A
	b = append(b, 0x00, 0x01) // Class IN

	// EDNS0 OPT-RR
	b = append(b, 0x00)                   // Name = root
	b = append(b, 0x00, 0x29)            // Type = OPT (41)
	udpSize := 1232
	if randInt(0, 1) == 1 {
		udpSize = 4096
	}
	b = append(b, byte(udpSize>>8), byte(udpSize&0xff)) // UDP payload size
	b = append(b, 0x00, 0x00, 0x00, 0x00)               // Extended RCODE + version
	b = append(b, 0x00, 0x00)                             // DO=0, Z=0
	b = append(b, 0x00, 0x00)                             // RDATA length = 0

	return hex.EncodeToString(b)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// DomainPoolByRegion returns domain pools for CPS mimicry profiles by region.
// Domain pools sourced from pumbaX/awg-multi-script.
func DomainPoolByRegion(profile CPSProfile, region string) []string {
	pools := map[CPSProfile]map[string][]string{
		CPSProfileQUIC: {
			"ru":    {"gosuslugi.ru", "update.microsoft.com", "releases.ubuntu.com", "online.sberbank.ru", "na2.ru"},
			"world": {"update.microsoft.com", "releases.ubuntu.com", "cdn.mozilla.net", "www.google.com", "www.cloudflare.com"},
		},
		CPSProfileSIP: {
			"ru":    {"gosuslugi.ru", "sip.telefonica.com", "sip.de", "update.microsoft.com"},
			"world": {"sip.telefonica.com", "sip.de", "update.microsoft.com", "cdn.mozilla.net"},
		},
		CPSProfileDNS: {
			"ru":    {"gosuslugi.ru", "dns.google", "update.microsoft.com", "cloudflare-dns.com"},
			"world": {"dns.google", "cloudflare-dns.com", "update.microsoft.com", "cdn.mozilla.net"},
		},
	}
	if pm, ok := pools[profile]; ok {
		if p, ok2 := pm[region]; ok2 {
			return p
		}
		// Fallback to world
		return pm["world"]
	}
	return []string{"update.microsoft.com"}
}

// PickRandomDomain picks a random domain from the given profile/region pool.
func PickRandomDomain(profile CPSProfile, region string) string {
	pool := DomainPoolByRegion(profile, region)
	if len(pool) == 0 {
		return "update.microsoft.com"
	}
	// Deterministic "random" — use first domain based on region length
	idx := len(region) % len(pool)
	return pool[idx]
}

