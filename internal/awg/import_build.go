// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// BuiltInbound is the inbound row plus how many peers lack a private key.
type BuiltInbound struct {
	Inbound      *model.Inbound
	MissingKeys  int
	CurrentIface string
}

// BuildInbound turns a discovered candidate into an AWG inbound. Keys, IPs,
// port and obfuscation are copied as-is. routeThroughXray stays off so kernel
// NAT of the existing setup is preserved.
func BuildInbound(c ImportCandidate, userId int, reservedEmails map[string]struct{}) (BuiltInbound, error) {
	used := map[string]struct{}{}
	for e := range reservedEmails {
		if e != "" {
			used[strings.ToLower(e)] = struct{}{}
		}
	}
	clients := make([]map[string]any, 0, len(c.Conf.Peers))
	missing := 0
	for i, p := range c.Conf.Peers {
		email := p.Name
		if i < len(c.Peers) && c.Peers[i].Email != "" {
			email = c.Peers[i].Email
		}
		email = sanitizeEmail(email, p.AllowedIPs, p.PublicKey, used)
		ips := p.AllowedIPs
		if p.Suspended && p.OrigIPs != "" {
			ips = p.OrigIPs
		}
		allowed := splitAllowed(ips)
		row := map[string]any{
			"email":        email,
			"publicKey":    p.PublicKey,
			"preSharedKey": p.PSK,
			"allowedIPs":   allowed,
			"enable":       !p.Suspended,
			"limitIp":      0,
			"totalGB":      0,
			"expiryTime":   parseUnixExpiry(p.Expiry),
			"tgId":         0,
			"subId":        randomSubID(),
			"comment":      p.Name,
			"reset":        0,
			"keepAlive":    string(p.Keepalive),
		}
		if k, ok := c.Keys[p.PublicKey]; ok && k.PrivateKey != "" {
			row["privateKey"] = k.PrivateKey
		} else {
			missing++
		}
		clients = append(clients, row)
	}
	mtu := c.Conf.MTU
	if mtu == 0 {
		mtu = 1420
	}
	dns := c.Conf.DNS
	if dns == "" {
		dns = "1.1.1.1"
	}
	settings := map[string]any{
		"privateKey":             c.Conf.PrivateKey,
		"address":                normalizeImportedAddress(c.Conf.Address),
		"mtu":                    mtu,
		"dns":                    dns,
		"jc":                     c.Conf.Jc,
		"jmin":                   c.Conf.Jmin,
		"jmax":                   c.Conf.Jmax,
		"s1":                     c.Conf.S1,
		"s2":                     c.Conf.S2,
		"s3":                     c.Conf.S3,
		"s4":                     c.Conf.S4,
		"h1":                     c.Conf.H1,
		"h2":                     c.Conf.H2,
		"h3":                     c.Conf.H3,
		"h4":                     c.Conf.H4,
		"i1":                     c.Conf.I1,
		"i2":                     c.Conf.I2,
		"i3":                     c.Conf.I3,
		"i4":                     c.Conf.I4,
		"i5":                     c.Conf.I5,
		"headerProtectionKey":    c.Conf.HeaderProtectionKey,
		"contentPaddingAddition": string(c.Conf.ContentPaddingAddition),
		"rekeyAfterTime":         string(c.Conf.RekeyAfterTime),
		"rekeyTimeout":           string(c.Conf.RekeyTimeout),
		"rejectAfterTime":        string(c.Conf.RejectAfterTime),
		"keepaliveTimeout":       string(c.Conf.KeepaliveTimeout),
		"maxHandshakeAttempts":   string(c.Conf.MaxHandshakeAttempts),
		"randomTrailers":         c.Conf.RandomTrailers,
		"disableCookies":         c.Conf.DisableCookies,
		"awgVersion":             c.Conf.AwgVersion,
		"routeThroughXray":       false,
		"outboundTag":            "",
		"clients":                clients,
	}
	if pub := PublicKeyOf(c.Conf.PrivateKey); pub != "" {
		settings["publicKey"] = pub
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return BuiltInbound{}, err
	}
	ib := &model.Inbound{
		UserId:         userId,
		Port:           c.Port,
		Protocol:       model.AWG,
		Remark:         importRemark(c),
		Enable:         !c.DropOnImport,
		Settings:       string(raw),
		StreamSettings: `{}`,
		Sniffing:       `{}`,
	}
	return BuiltInbound{Inbound: ib, MissingKeys: missing, CurrentIface: c.Ifname}, nil
}

func importRemark(c ImportCandidate) string {
	base := c.Ifname
	if base == "" {
		base = "awg"
	}
	if c.Port > 0 {
		return "imported-" + base + "-" + strconv.Itoa(c.Port)
	}
	return "imported-" + base
}

func normalizeImportedAddress(addr string) string {
	p, err := netip.ParsePrefix(strings.TrimSpace(addr))
	if err != nil {
		return addr
	}
	if p.Bits() >= 32 || p.Addr() != p.Masked().Addr() {
		return addr
	}
	next := p.Addr().Next()
	if !p.Contains(next) {
		return addr
	}
	return netip.PrefixFrom(next, p.Bits()).String()
}

func splitAllowed(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func randomSubID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "import0000000000"
	}
	return hex.EncodeToString(b[:])
}
