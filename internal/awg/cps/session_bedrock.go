// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"encoding/binary"
	"net"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
)

// bedrockDecoyMTU is the generator's recommended tunnel MTU (payload fits
// almost everywhere). I2/I3 are sized from it, not from the inbound's MTU.
const bedrockDecoyMTU = 1280

// GenerateMinecraftCPS renders the Minecraft Bedrock (RakNet) I1-I5 CPS set —
// GenerateCPS's counterpart for ObfMinecraft, ported from
// phenomenonrt.github.io/generatorM (balanced). Uses the same redraw-vs-budget
// contract as GenerateCPS but with a per-packet builder, since the RakNet
// literals do not draw from the domain pools.
func GenerateMinecraftCPS(serverIP string, serverPort int, onlyI1 bool, maxIBytes int) (CPSResult, error) {
	ip, port := sanitizeBedrockEndpoint(serverIP, serverPort)
	return generateWithBudget(onlyI1, maxIBytes, func(onlyI1 bool) (CPSResult, error) {
		return fromSession(bedrockSession(ip, port), onlyI1)
	})
}

// sanitizeBedrockEndpoint validates the endpoint the RakNet decoys name.
// I3 (Open Connection Request 2) embeds the server IPv4 + port verbatim, like
// the source generator: an invalid/empty address falls back to the TEST-NET-1
// placeholder 192.0.2.1 and the default Bedrock/RakNet UDP port 19132.
func sanitizeBedrockEndpoint(serverIP string, serverPort int) (string, int) {
	ip := net.ParseIP(serverIP)
	if ip == nil || ip.To4() == nil {
		ip = net.ParseIP("192.0.2.1")
	}
	if serverPort <= 0 || serverPort > 65535 {
		serverPort = 19132
	}
	return ip.String(), serverPort
}

// bedrockSession draws the five RakNet packets generatorM imitates.
// I2 is an MTU probe (UDP payload = MTU-28, zero pad). I3 stores the server
// IPv4 bitwise-NOT, the way RakNet serializes SystemAddress, and the same
// client GUID as I1. I4/I5 are frame-sets (seq, unreliable, bit-length).
func bedrockSession(serverIP string, serverPort int) [5]Descriptor {
	magic := []byte{
		0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe, 0xfe, 0xfe,
		0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34, 0x56, 0x78,
	}
	guid := randomBytes(8)
	request1 := bedrockDecoyMTU - 28
	if request1 < 548 {
		request1 = 548
	}
	if request1 > awg.MaxIPacketBytes {
		request1 = awg.MaxIPacketBytes
	}
	set := [5]Descriptor{}

	// I1: unconnected ping. Timestamp stays random; GUID is shared with I3.
	set[0] = Descriptor{}.
		Lit([]byte{0x01}).
		Rand(8).
		Lit(magic).
		Lit(guid)

	// I2: open connection request 1. 18 bytes of header, the rest zero pad.
	set[1] = Descriptor{}.
		Lit([]byte{0x05}).
		Lit(magic).
		Lit([]byte{0x0b}).
		Lit(make([]byte, request1-18))

	// I3: open connection request 2. Advertised MTU is the I2 UDP size + 28.
	set[2] = Descriptor{}.
		Lit([]byte{0x07}).
		Lit(magic).
		Lit([]byte{0x04}).
		Lit(raknetIPv4(serverIP)).
		Lit(u16be(serverPort)).
		Lit(u16be(request1 + 28)).
		Lit(guid)

	capN := bedrockDecoyMTU - 40
	if capN > awg.MaxIPacketBytes-7 {
		capN = awg.MaxIPacketBytes - 7
	}
	set[3] = raknetFrame(0, bedrockSpan(128, 64, capN))
	set[4] = raknetFrame(1, bedrockSpan(288, 128, capN))
	return set
}

// raknetIPv4 is RakNet's SystemAddress form: each IPv4 octet bitwise-NOT,
// no endian swap (192.0.2.1 → 3f ff fd fe).
func raknetIPv4(serverIP string) []byte {
	v4 := net.ParseIP(serverIP).To4()
	out := make([]byte, 4)
	for i, b := range v4 {
		out[i] = ^b
	}
	return out
}

func u16be(n int) []byte {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(n))
	return b[:]
}

func u24le(n int) []byte {
	return []byte{byte(n), byte(n >> 8), byte(n >> 16)}
}

func bedrockSpan(base, extra, capN int) int {
	n := base + rng.Intn(extra+1)
	if n > capN {
		n = capN
	}
	if n < 1 {
		n = 1
	}
	return n
}

func raknetFrame(seq, n int) Descriptor {
	return Descriptor{}.
		Lit([]byte{0x84}).
		Lit(u24le(seq)).
		Lit([]byte{0x00}).
		Lit(u16be(n * 8)).
		Rand(n)
}
