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
// egress), a single [Peer] (the upstream server), and DNS is NEVER written.
//
// I1-I5 (CPS packets) are NEVER written to the .conf, even when set in Settings.
// The kernel amneziawg module does not accept CPS tags in setconf input —
// `awg setconf awgo-N /dev/fd/63` returns "Invalid argument" and awg-quick
// rolls back the interface, so reconcile fails every 10s (caught live by a
// tester on awgo-2: every reconcile failed with exit status 1). This mirrors
// renderServerConf, which already documents the same constraint for the server
// .conf. I1-I5 stay in Settings JSON for a future userspace CPS sender.
//
// DNS is NEVER written even when set in Settings. With Table = off, the client
// AWG interface does not carry the system default route, so the panel host's
// system DNS must NOT be overwritten through the tunnel. Writing DNS makes
// awg-quick invoke `resolvconf -a awgo-N`, which on a server without a working
// resolvconf backend (systemd-resolved disabled, openresolv absent) fails with
// "Failed to set DNS configuration: Unit dbus-org.freedesktop.resolve1.service
// not found" — awg-quick rolls back the interface and reconcile fails every
// 10s. Xray resolves via the freedom outbound's domainStrategy: UseIP using
// the panel host's existing resolver, so the client .conf does not need DNS.
// Settings.DNS stays in the JSON for forward-compat with a future userspace
// resolver, but is ignored here. (Caught live by tester VladufQa: awgo-3
// reconcile failed every 10s until `systemctl enable systemd-resolved`.)
func renderClientConf(ci ClientInstance) string {
	s := ci.Settings
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", s.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", s.Address)
	fmt.Fprintf(&b, "MTU = %d\n", s.MTU)
	b.WriteString("Table = off\n")
	// DNS deliberately NOT written — see function comment. resolvconf crash
	// on hosts without systemd-resolved/openresolv took down every reconcile.
	awg3ok := NormalizeAWGVersion(s.AwgVersion) == "3" && ModuleSupportsAwg3()
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
		// HeaderProtectionKey (AWG3) is written ONLY when AwgVersion == "3" and
		// the key is non-empty — mirrors renderServerConf. The upstream kernel
		// v3.0.20260731 + tools v3.0.20260730 parse the field; older builds
		// reject it, so version-gating keeps v1/v2 outbounds working. S1-S4
		// >= 12 is required for the kernel to accept the key (enforced by
		// the generator when version "3" is selected). Module-gated so a v3
		// outbound on a host with a v1.x module does not emit the line.
		if awg3ok && s.HeaderProtectionKey != "" {
			fmt.Fprintf(&b, "HeaderProtectionKey = %s\n", s.HeaderProtectionKey)
		}
		// AWG3 device-level timers/padding — 0 = kernel default.
		if awg3ok {
			if s.ContentPaddingAddition > 0 {
				fmt.Fprintf(&b, "ContentPaddingAddition = %d\n", s.ContentPaddingAddition)
			}
			if s.RekeyAfterTime > 0 {
				fmt.Fprintf(&b, "RekeyAfterTime = %d\n", s.RekeyAfterTime)
			}
			if s.RekeyTimeout > 0 {
				fmt.Fprintf(&b, "RekeyTimeout = %d\n", s.RekeyTimeout)
			}
			if s.RejectAfterTime > 0 {
				fmt.Fprintf(&b, "RejectAfterTime = %d\n", s.RejectAfterTime)
			}
			if s.KeepaliveTimeout > 0 {
				fmt.Fprintf(&b, "KeepaliveTimeout = %d\n", s.KeepaliveTimeout)
			}
			if s.MaxHandshakeAttempts > 0 {
				fmt.Fprintf(&b, "MaxHandshakeAttempts = %d\n", s.MaxHandshakeAttempts)
			}
		}
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
