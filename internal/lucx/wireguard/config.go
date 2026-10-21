// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"fmt"
	"net/netip"
	"strings"
)

const owner = "lucx-kernel-wireguard"

func RenderConfig(i Instance) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n[Interface]\nPrivateKey = %s\nListenPort = %d\nAddress = %s\nMTU = %d\nTable = off\n", owner, i.PrivateKey, i.Port, i.Address, i.MTU)
	if i.FwMark != 0 {
		fmt.Fprintf(&b, "FwMark = 0x%x\n", i.FwMark)
	}
	for _, p := range i.Peers {
		fmt.Fprintf(&b, "\n[Peer]\nPublicKey = %s\n", p.PublicKey)
		if p.PSK != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", p.PSK)
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", "))
		if p.Keepalive > 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.Keepalive)
		}
	}
	return b.String()
}

func RenderFirewall(i Instance) string {
	prefix, _ := netip.ParsePrefix(i.Address)
	return fmt.Sprintf(`table inet %s {
 comment "%s"
 chain prerouting {
  type filter hook prerouting priority mangle + 2; policy accept;
  iifname "%s" meta nfproto ipv4 ip daddr %s return
  iifname "%s" meta l4proto { tcp, udp } tproxy ip to 127.0.0.1:%d meta mark set %s accept
  iifname "%s" counter drop
 }
 chain forward {
  type filter hook forward priority filter; policy accept;
  iifname "%s" counter drop comment "%s"
 }
 chain input {
  type filter hook input priority filter; policy accept;
  iifname "%s" meta nfproto ipv6 counter drop
 }
}
`, i.FirewallTable(), owner, i.Name(), prefix.Masked(), i.Name(), i.TproxyPort, i.Mark(), i.Name(), i.Name(), owner, i.Name())
}
