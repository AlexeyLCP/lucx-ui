// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"net/netip"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// defaultAwgBase is the tunnel subnet AWG clients are allocated from. It is
// intentionally distinct from WireGuard's 10.0.0.0/24 so an AWG inbound and a
// WireGuard inbound on the same panel don't collide on peer addresses.
const defaultAwgBase = "10.8.0.0/24"

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
	base := wireguardAllocationBase(used, awgAllocationFallback(serverAddr))
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
			addr, err := allocateWireguardAddress(used, base)
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

		if i < len(interfaceClients) {
			if m, ok := interfaceClients[i].(map[string]any); ok {
				m["privateKey"] = c.PrivateKey
				m["publicKey"] = c.PublicKey
				m["allowedIPs"] = c.AllowedIPs
				if c.PreSharedKey != "" {
					m["preSharedKey"] = c.PreSharedKey
				}
				if c.KeepAlive > 0 {
					m["keepAlive"] = c.KeepAlive
				}
				interfaceClients[i] = m
			}
		}
	}
	return nil
}
