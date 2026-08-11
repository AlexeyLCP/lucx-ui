// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// defaultAwgBase is the tunnel subnet AWG clients are allocated from. It is
// intentionally distinct from WireGuard's 10.0.0.0/24 so an AWG inbound and a
// WireGuard inbound on the same panel don't collide on peer addresses. It also
// deliberately avoids the 10.6/10.7/10.8 ranges (lucx.64): those are the most
// common upstream WireGuard/AmneziaWG server subnets, so an AWG outbound
// (awgo-N) pasted from a provider conf almost always lands there — an inbound
// on the same /24 installs a second connected route and traffic dies (Pattern
// 1e). 10.200.0.0/24 sits far from both.
const defaultAwgBase = "10.200.0.0/24"

// awgAllocationFallback derives the allocation subnet from the inbound's
// tunnel address (e.g. "10.9.0.1/24" → "10.9.0.0/24"), falling back to
// defaultAwgBase when the address is empty or unparseable.
func awgAllocationFallback(serverAddr string) string {
	addr := strings.TrimSpace(serverAddr)
	if addr == "" {
		return defaultAwgBase
	}
	prefix, err := netip.ParsePrefix(addr)
	if err != nil {
		return defaultAwgBase
	}
	return prefix.Masked().String()
}

// awgAllowedIPsStale reports whether a client's stored allowedIPs no longer
// belong to the inbound's current tunnel subnet: every entry is a
// single-host address (/32 or /128) and at least one falls outside the
// subnet. That is the signature of a client detached from an AWG inbound
// and re-attached later — after the subnet changed or from another AWG
// inbound — carrying its old address (lucx.92). The handshake still
// succeeds (keys match), but the server routes a subnet it no longer owns,
// so traffic dies. Custom entries (0.0.0.0/0, ::/0, anything non-host) are
// never treated as stale — operator-managed configs stay untouched.
func awgAllowedIPsStale(allowedIPs []string, serverAddr string) bool {
	if len(allowedIPs) == 0 {
		return false
	}
	subnet, err := netip.ParsePrefix(strings.TrimSpace(serverAddr))
	if err != nil {
		return false
	}
	subnet = subnet.Masked()
	outside := false
	for _, raw := range allowedIPs {
		p, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return false
		}
		if p.Bits() != p.Addr().BitLen() {
			return false
		}
		if !subnet.Contains(p.Addr()) {
			outside = true
		}
	}
	return outside
}

// awgSettingsAddress extracts the tunnel address from an AWG inbound's
// settings JSON ("" when absent or malformed).
func awgSettingsAddress(settings string) string {
	var s struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return ""
	}
	return s.Address
}

// awgSettingsClientIPs returns the single-host client tunnel addresses found in
// an AWG inbound's settings.clients[].allowedIPs. Bare addresses and /32 (or
// /128) entries are returned as bare address strings; network entries such as
// 0.0.0.0/0 or a whole /24 are skipped because they are not a single client
// host. Consumed by the AWG-outbound subnet-conflict guard, which must compare
// a pasted provider tunnel against the addresses clients actually occupy — NOT
// only the inbound's server address, since a legacy wrong-subnet inbound keeps
// its clients in a different /24 than its own (lucx.69).
func awgSettingsClientIPs(settings string) []string {
	var parsed struct {
		Clients []struct {
			AllowedIPs []string `json:"allowedIPs"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return nil
	}
	var out []string
	for _, cl := range parsed.Clients {
		for _, raw := range cl.AllowedIPs {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if p, err := netip.ParsePrefix(raw); err == nil {
				if p.Bits() == p.Addr().BitLen() {
					out = append(out, p.Addr().String())
				}
				continue
			}
			if a, err := netip.ParseAddr(raw); err == nil {
				out = append(out, a.String())
			}
		}
	}
	return out
}

// migrateAwgClientSubnets keeps client tunnel IPs stable when the operator
// changes the inbound Address. Peer AllowedIPs are independent of the server's
// Address/24 (kernel NAT marks by iif; routeThroughXray ignores the subnet),
// so clients do NOT need a re-exported .conf for Address edits or for toggling
// routeThroughXray. Only a client whose single-host AllowedIPs collides with
// the server's own NEW host address is re-allocated (otherwise awg-quick would
// install a peer /32 equal to the interface address).
//
// Pure JSON→JSON; no-op when addresses match / unparseable / no collision.
func migrateAwgClientSubnets(oldAddr, newAddr, settingsJSON string) string {
	oldAddr = strings.TrimSpace(oldAddr)
	newAddr = strings.TrimSpace(newAddr)
	if newAddr == "" || oldAddr == newAddr {
		return settingsJSON
	}
	newP, err := netip.ParsePrefix(newAddr)
	if err != nil || !newP.Addr().Is4() {
		return settingsJSON
	}
	serverHost := newP.Addr().String()
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil || settings == nil {
		return settingsJSON
	}
	clients, ok := settings["clients"].([]any)
	if !ok || len(clients) == 0 {
		return settingsJSON
	}
	used := []string{serverHost}
	for _, it := range clients {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		for _, ip := range anyStringSlice(m["allowedIPs"]) {
			if a, aErr := netip.ParseAddr(strings.TrimSpace(ip)); aErr == nil {
				if a.String() != serverHost {
					used = append(used, a.String())
				}
			} else if p, pErr := netip.ParsePrefix(strings.TrimSpace(ip)); pErr == nil {
				if p.Addr().String() != serverHost {
					used = append(used, p.Addr().String())
				}
			}
		}
	}
	base := newP.Masked().String()
	changed := false
	for _, it := range clients {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		ips := anyStringSlice(m["allowedIPs"])
		if len(ips) != 1 {
			continue
		}
		host := ""
		if a, aErr := netip.ParseAddr(strings.TrimSpace(ips[0])); aErr == nil {
			host = a.String()
		} else if p, pErr := netip.ParsePrefix(strings.TrimSpace(ips[0])); pErr == nil && p.Bits() == p.Addr().BitLen() {
			host = p.Addr().String()
		}
		if host == "" || host != serverHost {
			continue
		}
		addr, err := allocateWireguardAddress(used, base, false)
		if err != nil {
			return settingsJSON
		}
		m["allowedIPs"] = []any{addr}
		used = append(used, addr)
		changed = true
	}
	if !changed {
		return settingsJSON
	}
	out, err := json.Marshal(settings)
	if err != nil {
		return settingsJSON
	}
	return string(out)
}

func anyStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// defaultAwgClients fills in blank AmneziaWG credentials for newly added
// clients, mirroring defaultWireguardClients: a generated Curve25519 keypair
// when none was provided, a derived public key when only a private key was
// given, a fresh PSK when none was provided, and a unique tunnel address
// allocated from the inbound's subnet. It mutates both the typed clients and
// the parallel raw client maps that get persisted into the inbound settings.
// Existing values are never overwritten, so editing a client never rotates its
// keys.
//
// AmneziaWG uses the same Curve25519 base keypair and PSK format as WireGuard;
// only the obfuscation parameters (Jc/S1-S4/H1-H4/I1-I5) are AWG-specific and
// live on the inbound (shared by all peers), not on the client.
//
// serverAddr is the inbound's tunnel address (settings.address, e.g.
// "10.9.0.1/24"): client addresses are allocated from ITS subnet, not from a
// hardcoded pool — otherwise a first client on a non-default tunnel subnet
// would get an address the server never routes (caught live on a 10.9.0.1/24
// inbound whose first client received 10.8.0.2).
func defaultAwgClients(existing, clients []model.Client, interfaceClients []any, serverAddr string) error {
	used := make([]string, 0)
	for i := range existing {
		used = append(used, existing[i].AllowedIPs...)
	}
	// LUCX-HOOK: AWG outbound collision guard — exclude tunnel IPs already
	// claimed by enabled AWG outbounds (awgo-N kernel interfaces). Without
	// this, allocateWireguardAddress can hand a new client the same IP an
	// awgo-N interface owns (e.g. 10.8.0.2), and the kernel then treats the
	// client IP as local → return-path packets go to lo instead of awgN →
	// the client's traffic dies. This is the root cause of "второй клиент не
	// идёт трафик" when an AWG outbound is enabled on the same panel.
	if awgOuts, err := (&AwgOutboundService{}).ActiveOutboundAddresses(); err == nil {
		used = append(used, awgOuts...)
	}
	// END LUCX-HOOK
	// LUCX-HOOK (lucx.63): allocate strictly from the inbound's OWN tunnel
	// subnet, not from wireguardAllocationBase. The latter derives the base from
	// the first already-claimed IP in `used` — which includes awgo-* outbound
	// tunnel IPs appended by the collision guard above. With an active AWG
	// outbound on 10.8.0.x the base became 10.8.0.0/24 even for a 15.11.5.0/24
	// inbound, so the first client got an address the server never routes →
	// awg-quick up installs a colliding /32 → RTNETLINK "File exists" → the
	// interface rolls back ("Device awgN does not exist"). `used` stays an
	// exclusion set; only the base source changes.
	base := awgAllocationFallback(serverAddr)
	for i := range clients {
		c := &clients[i]
		if c.PrivateKey == "" && c.PublicKey == "" {
			priv, pub, err := wgutil.GenerateWireguardKeypair()
			if err != nil {
				return err
			}
			c.PrivateKey = priv
			c.PublicKey = pub
		} else if c.PublicKey == "" && c.PrivateKey != "" {
			pub, err := wgutil.PublicKeyFromPrivate(c.PrivateKey)
			if err != nil {
				return err
			}
			c.PublicKey = pub
		}
		if c.PreSharedKey == "" {
			psk, err := wgutil.GenerateWireguardPSK()
			if err != nil {
				return err
			}
			c.PreSharedKey = psk
		}
		if len(c.AllowedIPs) == 0 {
			// Only allocate when blank — never rotate an existing tunnel IP
			// (client .conf Address would change → forced re-download). Attach
			// paths clear AllowedIPs when they want a fresh IP for a new inbound.
			addr, err := allocateWireguardAddress(used, base, false)
			if err != nil {
				return err
			}
			c.AllowedIPs = []string{addr}
		} else {
			normalized, err := normalizeWireguardAllowedIPs(c.AllowedIPs)
			if err != nil {
				return err
			}
			if len(normalized) == 0 {
				return common.NewError("awg: allowedIPs has no usable entry")
			}
			if hit := wireguardAllowedIPsCollision(normalized, used); hit != "" {
				return common.NewError("awg: allowedIPs entry already used by another client:", hit)
			}
			c.AllowedIPs = normalized
		}
		used = append(used, c.AllowedIPs...)
		if c.KeepAlive.IsZero() {
			c.KeepAlive = "25"
		}

		if i < len(interfaceClients) {
			if m, ok := interfaceClients[i].(map[string]any); ok {
				m["privateKey"] = c.PrivateKey
				m["publicKey"] = c.PublicKey
				m["allowedIPs"] = c.AllowedIPs
				if c.PreSharedKey != "" {
					m["preSharedKey"] = c.PreSharedKey
				}
				m["keepAlive"] = c.KeepAlive.String()
				interfaceClients[i] = m
			}
		}
	}
	return nil
}

// ResolveInboundShareHost picks the host for client Endpoint=/share links,
// mirroring sub.SubService.resolveInboundAddress and frontend resolveShareHost.
// Order depends on inbound.ShareAddrStrategy (node/listen/custom). fallback is
// the panel public host (sub/web domain or public IP) — never an OS hostname.
// nodeAddr is the hosting node address when inbound is node-managed ("" local).
func ResolveInboundShareHost(inbound *model.Inbound, nodeAddr, fallback string) string {
	var listenAddr string
	if inbound != nil {
		listen := strings.TrimSpace(inbound.Listen)
		if listen != "" && listen[0] != '@' && listen[0] != '/' && isShareableListen(listen) {
			listenAddr = listen
		}
	}
	nodeAddr = strings.TrimSpace(nodeAddr)
	fallback = strings.TrimSpace(fallback)
	custom := ""
	strategy := "node"
	if inbound != nil {
		custom = strings.TrimSpace(inbound.ShareAddr)
		if s := strings.TrimSpace(inbound.ShareAddrStrategy); s != "" {
			strategy = s
		}
	}
	var candidates []string
	switch strategy {
	case "listen":
		candidates = []string{listenAddr, nodeAddr, fallback}
	case "custom":
		candidates = []string{custom, nodeAddr, listenAddr, fallback}
	default:
		candidates = []string{nodeAddr, listenAddr, fallback}
	}
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	return ""
}

// isShareableListen reports whether a bind address is usable as a client
// Endpoint host (not loopback / unspecified / unix socket).
func isShareableListen(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return !ip.IsLoopback() && !ip.IsUnspecified()
	}
	return true
}

// formatEndpointHost returns host suitable for Endpoint = host:port (IPv6 bracketed).
func formatEndpointHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "[" + host + "]"
	}
	return host
}

// NodeAddressForInbound returns the node Address when inbound is node-managed.
func NodeAddressForInbound(inbound *model.Inbound) string {
	if inbound == nil || inbound.NodeID == nil {
		return ""
	}
	var n model.Node
	if err := database.GetDB().Select("address").First(&n, *inbound.NodeID).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(n.Address)
}

// BuildAwgClientConf renders a full AmneziaWG client .conf for export (panel QR
// path / Telegram bot). Mirrors frontend buildAwgClientConfig: [Interface]
// with client keypair, tunnel address, DNS, MTU, obfuscation block from
// inboundAwgHints; [Peer] with server public key, PSK, full-tunnel AllowedIPs,
// endpoint, and optional PersistentKeepalive.
func BuildAwgClientConf(inbound *model.Inbound, client *model.Client, endpointHost string) (string, error) {
	if inbound == nil || client == nil {
		return "", common.NewError("awg: missing inbound or client")
	}
	if inbound.Protocol != model.AWG {
		return "", common.NewError("awg: inbound is not AWG")
	}
	priv := strings.TrimSpace(client.PrivateKey)
	if priv == "" {
		return "", common.NewError("awg: client has no private key")
	}
	var s struct {
		PrivateKey string `json:"privateKey"`
		MTU        int    `json:"mtu"`
		DNS        string `json:"dns"`
	}
	_ = json.Unmarshal([]byte(inbound.Settings), &s)
	serverPub := ""
	if sk := strings.TrimSpace(s.PrivateKey); sk != "" {
		if pub, err := wgutil.PublicKeyFromPrivate(sk); err == nil {
			serverPub = pub
		}
	}
	if serverPub == "" {
		return "", common.NewError("awg: cannot derive server public key")
	}
	address := ""
	if len(client.AllowedIPs) > 0 {
		address = strings.TrimSpace(client.AllowedIPs[0])
	}
	if address == "" {
		address = "10.200.0.2/32"
	}
	dns := strings.TrimSpace(s.DNS)
	if dns == "" {
		dns = "1.1.1.1, 1.0.0.1"
	}
	mtu := s.MTU
	if mtu <= 0 {
		mtu = 1320
	}
	host := formatEndpointHost(endpointHost)
	if host == "" {
		host = "127.0.0.1"
	}
	_, obf, _ := inboundAwgHints(inbound.Settings)

	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", priv)
	fmt.Fprintf(&b, "Address = %s\n", address)
	fmt.Fprintf(&b, "DNS = %s\n", dns)
	fmt.Fprintf(&b, "MTU = %d\n", mtu)
	if obf = strings.TrimSpace(obf); obf != "" {
		b.WriteString(obf)
		if !strings.HasSuffix(obf, "\n") {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", serverPub)
	if psk := strings.TrimSpace(client.PreSharedKey); psk != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", psk)
	}
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", host, inbound.Port)
	if !client.KeepAlive.IsZero() {
		fmt.Fprintf(&b, "PersistentKeepalive = %s\n", client.KeepAlive.String())
	}
	return b.String(), nil
}
