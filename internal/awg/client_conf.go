// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"strings"
)

// renderClientConf builds the awg-quick .conf for a client AWG instance. Unlike
// renderServerConf, this has no ListenPort, no peers[], Table = off (so awg-quick
// does NOT override the system default route — Xray's sockopt.interface handles
// egress), a single [Peer] (the upstream server), and DNS is omitted by default
// (Xray resolves via domainStrategy: UseIP; operator can opt in via Advanced).
func renderClientConf(ci ClientInstance) string {
	s := ci.Settings
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", s.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", s.Address)
	fmt.Fprintf(&b, "MTU = %d\n", s.MTU)
	b.WriteString("Table = off\n")
	if dns := strings.TrimSpace(s.DNS); dns != "" {
		fmt.Fprintf(&b, "DNS = %s\n", dns)
	}
	if s.Jc > 0 {
		fmt.Fprintf(&b, "Jc = %d\n", s.Jc)
		fmt.Fprintf(&b, "Jmin = %d\n", s.Jmin)
		fmt.Fprintf(&b, "Jmax = %d\n", s.Jmax)
		fmt.Fprintf(&b, "S1 = %d\n", s.S1)
		fmt.Fprintf(&b, "S2 = %d\n", s.S2)
		fmt.Fprintf(&b, "S3 = %d\n", s.S3)
		fmt.Fprintf(&b, "S4 = %d\n", s.S4)
		fmt.Fprintf(&b, "H1 = %s\n", s.H1)
		fmt.Fprintf(&b, "H2 = %s\n", s.H2)
		fmt.Fprintf(&b, "H3 = %s\n", s.H3)
		fmt.Fprintf(&b, "H4 = %s\n", s.H4)
	}
	if s.I1 != "" {
		fmt.Fprintf(&b, "I1 = %s\n", s.I1)
		fmt.Fprintf(&b, "I2 = %s\n", s.I2)
		fmt.Fprintf(&b, "I3 = %s\n", s.I3)
		fmt.Fprintf(&b, "I4 = %s\n", s.I4)
		fmt.Fprintf(&b, "I5 = %s\n", s.I5)
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", s.PublicKey)
	if psk := strings.TrimSpace(s.PSK); psk != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", psk)
	}
	fmt.Fprintf(&b, "Endpoint = %s\n", s.Endpoint)
	fmt.Fprintf(&b, "AllowedIPs = %s\n", s.AllowedIPs)
	if s.Keepalive > 0 {
		fmt.Fprintf(&b, "PersistentKeepalive = %d\n", s.Keepalive)
	}
	return b.String()
}
