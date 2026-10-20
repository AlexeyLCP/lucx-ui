// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"encoding/binary"
	"net"
)

// GenerateMinecraftCPS renders the Minecraft Bedrock (RakNet) I1-I5 CPS set —
// GenerateCPS's counterpart for ObfMinecraft (ported from
// phenomenonrt.github.io). Uses the same redraw-vs-budget contract as
// GenerateCPS but with a per-packet builder, since the RakNet literals do not
// draw from the domain pools.
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

// bedrockSession draws the five RakNet packets the generator imitates:
// unconnected ping, open-connection requests 1 and 2, and two game packets.
// Sizes follow the balanced band of phenomenonrt.github.io. The 16-byte
// offline magic and the ID bytes are protocol-correct; timestamps, GUID and
// payload are per-send random spans.
func bedrockSession(serverIP string, serverPort int) [5]Descriptor {
	// RakNet offline magic: the ping and both connection requests key on it.
	magic := []byte{
		0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe, 0xfe, 0xfe,
		0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34, 0x56, 0x78,
	}
	set := [5]Descriptor{}

	// I1: unconnected ping (ID 0x01), 8-byte timestamp, magic, 8-byte GUID.
	set[0] = Descriptor{}.
		Lit([]byte{0x01}).
		Rand(8).
		Lit(magic).
		Rand(8)

	// I2: open connection request 1 (ID 0x05), magic, RakNet protocol 11,
	// 28 zero bytes of MTU probe padding.
	pad := make([]byte, 28)
	set[1] = Descriptor{}.
		Lit([]byte{0x05}).
		Lit(magic).
		Lit([]byte{0x0b}).
		Lit(pad)

	// I3: open connection request 2 (ID 0x07), magic, IPv4 family byte,
	// server address + port, negotiated MTU, 8-byte GUID. Wire size 34 bytes.
	var addr []byte
	addr = append(addr, 0x04) // AF_INET
	addr = append(addr, net.ParseIP(serverIP).To4()...)
	var port2 [2]byte
	binary.BigEndian.PutUint16(port2[:], uint16(serverPort&0xffff))
	addr = append(addr, port2[:]...)
	var mtu2 [2]byte
	binary.BigEndian.PutUint16(mtu2[:], 1400)
	addr = append(addr, mtu2[:]...)
	set[2] = Descriptor{}.
		Lit([]byte{0x07}).
		Lit(magic).
		Lit(addr).
		Rand(8)

	// I4/I5: game packets (ID 0x84) with a 3-byte raknet sequence number and a
	// payload spanning real game sizes (balanced band).
	n4 := 128 + rng.Intn(65)
	n5 := 288 + rng.Intn(129)
	set[3] = Descriptor{}.
		Lit([]byte{0x84}).
		Rand(3).
		Rand(n4)
	set[4] = Descriptor{}.
		Lit([]byte{0x84}).
		Rand(3).
		Rand(n5)

	return set
}
