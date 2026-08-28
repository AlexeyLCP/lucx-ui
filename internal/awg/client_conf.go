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
	awg3ok := IsAwg3Plus(s.AwgVersion) && ModuleSupportsAwg3()
	awg31ok := IsAwg31(s.AwgVersion) && ModuleSupportsAwg31()
	// Jc gates the junk-packet fields only. S1-S4 pad the handshake on their own
	// and ParseConf keeps them, so an imported Jc = 0 conf must not lose them.
	junk := s.Jc > 0
	if junk {
		fmt.Fprintf(&b, "Jc = %d\n", s.Jc)
		fmt.Fprintf(&b, "Jmin = %d\n", s.Jmin)
		fmt.Fprintf(&b, "Jmax = %d\n", s.Jmax)
	}
	if junk || s.S1 > 0 {
		fmt.Fprintf(&b, "S1 = %d\n", s.S1)
	}
	if junk || s.S2 > 0 {
		fmt.Fprintf(&b, "S2 = %d\n", s.S2)
	}
	if NormalizeAWGVersion(s.AwgVersion) != "1.5" {
		if junk || s.S3 > 0 {
			fmt.Fprintf(&b, "S3 = %d\n", s.S3)
		}
		if junk || s.S4 > 0 {
			fmt.Fprintf(&b, "S4 = %d\n", s.S4)
		}
	}
	// Per field and outside the Jc gate, exactly like renderServerConf: a blank one
	// is skipped ("H1 = " makes setconf reject the file), Jc=0 keeps its headers.
	for i, h := range []string{s.H1, s.H2, s.H3, s.H4} {
		if strings.TrimSpace(h) != "" {
			fmt.Fprintf(&b, "H%d = %s\n", i+1, h)
		}
	}
	// Version- and module-gated like renderServerConf (manager.go:840 carries the
	// full why): an older kernel rejects the line and awg-quick then deletes awgo-N.
	if awg3ok && s.HeaderProtectionKey != "" {
		fmt.Fprintf(&b, "HeaderProtectionKey = %s\n", s.HeaderProtectionKey)
	}
	// AWG3 device-level timers/padding — AwgTimer holds single or range
	// ("lo-hi") values verbatim; IsZero omits the kernel default.
	if awg3ok {
		if !s.ContentPaddingAddition.IsZero() {
			fmt.Fprintf(&b, "ContentPaddingAddition = %s\n", s.ContentPaddingAddition)
		}
		if !s.RekeyAfterTime.IsZero() {
			fmt.Fprintf(&b, "RekeyAfterTime = %s\n", s.RekeyAfterTime)
		}
		if !s.RekeyTimeout.IsZero() {
			fmt.Fprintf(&b, "RekeyTimeout = %s\n", s.RekeyTimeout)
		}
		if !s.RejectAfterTime.IsZero() {
			fmt.Fprintf(&b, "RejectAfterTime = %s\n", s.RejectAfterTime)
		}
		if !s.KeepaliveTimeout.IsZero() {
			fmt.Fprintf(&b, "KeepaliveTimeout = %s\n", s.KeepaliveTimeout)
		}
		if !s.MaxHandshakeAttempts.IsZero() {
			fmt.Fprintf(&b, "MaxHandshakeAttempts = %s\n", s.MaxHandshakeAttempts)
		}
	}
	if awg31ok {
		if s.RandomTrailers {
			b.WriteString("RandomTrailers = on\n")
		}
		if s.DisableCookies {
			b.WriteString("DisableCookies = on\n")
		}
	}
	// In the .conf so the FIRST handshake carries CPS mimicry — a post-up `awg
	// set` landed 20.4 ms late. Budgeted worst-case, like every other renderer.
	if NormalizeAWGVersion(s.AwgVersion) != "1.5" &&
		IBytes(s.I1, s.I2, s.I3, s.I4, s.I5) <= WorstCaseIBytesBudget(s.HeaderProtectionKey != "") {
		for _, f := range []struct{ key, value string }{
			{"I1", s.I1}, {"I2", s.I2}, {"I3", s.I3}, {"I4", s.I4}, {"I5", s.I5},
		} {
			if v := strings.TrimSpace(f.value); v != "" {
				fmt.Fprintf(&b, "%s = %s\n", f.key, v)
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
	if !s.Keepalive.IsZero() {
		fmt.Fprintf(&b, "PersistentKeepalive = %s\n", s.Keepalive)
	}
	return b.String()
}
