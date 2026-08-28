// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// ClientInstance is the desired runtime configuration of one AWG outbound: a
// kernel interface awgo-{Id} that connects as a client to an upstream AWG
// server. Unlike the server-side Instance, it has no ListenPort and a single
// peer (the upstream), not a peer list.
type ClientInstance struct {
	Id       int
	Ifname   string // "awgo-{Id}", never changes
	Tag      string // editable, used in Xray config as outbound tag
	Settings ClientSettings
}

// ClientSettings holds the parsed settings JSON for an AWG outbound.
type ClientSettings struct {
	PrivateKey string `json:"privateKey"`
	Address    string `json:"address"` // mandatory, e.g. "10.9.0.5/32"
	MTU        int    `json:"mtu"`
	PublicKey  string `json:"publicKey"` // upstream server public key
	PSK        string `json:"psk"`
	Endpoint   string `json:"endpoint"` // "host:port"
	// Keepalive is PersistentKeepalive (peer-level). AWG3 accepts single or
	// range ("25" / "15-25") via u16_range_t. Empty/"0" = off. AwgTimer so a
	// provider .conf range survives ParseConf (mirrors device timers, lucx.74).
	Keepalive  AwgTimer `json:"keepalive"`
	AllowedIPs string   `json:"allowedIPs"`
	DNS        string   `json:"dns"` // optional, only written if non-empty
	Jc         int      `json:"jc"`
	Jmin       int      `json:"jmin"`
	Jmax       int      `json:"jmax"`
	S1         int      `json:"s1"`
	S2         int      `json:"s2"`
	S3         int      `json:"s3"`
	S4         int      `json:"s4"`
	H1         string   `json:"h1"`
	H2         string   `json:"h2"`
	H3         string   `json:"h3"`
	H4         string   `json:"h4"`
	I1         string   `json:"i1"`
	I2         string   `json:"i2"`
	I3         string   `json:"i3"`
	I4         string   `json:"i4"`
	I5         string   `json:"i5"`
	// AWG3 (AmneziaWG 3) header protection key — 32-byte ChaCha20, base64.
	// Written to the .conf only when AwgVersion == "3" (the outbound opts into
	// AWG3); for older versions it stays empty. Upstream kernel v3.0.20260731 +
	// tools v3.0.20260730 parse the field.
	HeaderProtectionKey string `json:"headerProtectionKey"`
	// AWG3 device-level timers/padding. Kept as AwgTimer (a string that also
	// tolerates legacy JSON numbers) so a provider .conf carrying RANGE values
	// ("100-120") survives import verbatim — mirroring the inbound side, where
	// the same fields were converted from int to AwgTimer for exactly this
	// reason (lucx.74; tester VladufQa: "не поддерживает '-', только одно
	// значение"). Empty/"0" = kernel default, omitted by renderClientConf.
	ContentPaddingAddition AwgTimer `json:"contentPaddingAddition"`
	RekeyAfterTime         AwgTimer `json:"rekeyAfterTime"`
	RekeyTimeout           AwgTimer `json:"rekeyTimeout"`
	RejectAfterTime        AwgTimer `json:"rejectAfterTime"`
	KeepaliveTimeout       AwgTimer `json:"keepaliveTimeout"`
	MaxHandshakeAttempts   AwgTimer `json:"maxHandshakeAttempts"`
	// AWG 3.1 device flags. Written only when AwgVersion == "3.1".
	RandomTrailers bool `json:"randomTrailers"`
	DisableCookies bool `json:"disableCookies"`
	// AwgVersion targets the AmneziaWG protocol version for this outbound
	// ("1.5"/"2"/"3"/"3.1"; "" → "2" via normalize). The .conf renderer emits
	// HeaderProtectionKey for IsAwg3Plus and 3.1 flags only for "3.1".
	AwgVersion string `json:"awgVersion"`
}

// ClientInstanceFromOutbound parses an AwgOutbound row into a ClientInstance.
// Returns ok=false if Settings is empty/malformed or a mandatory field
// (Address, PublicKey, Endpoint) is missing.
func ClientInstanceFromOutbound(o *model.AwgOutbound) (ClientInstance, bool) {
	if o == nil {
		return ClientInstance{}, false
	}
	var s ClientSettings
	if err := json.Unmarshal([]byte(o.Settings), &s); err != nil {
		return ClientInstance{}, false
	}
	s.Address = strings.TrimSpace(s.Address)
	s.PublicKey = strings.TrimSpace(s.PublicKey)
	s.Endpoint = strings.TrimSpace(s.Endpoint)
	if s.Address == "" || s.PublicKey == "" || s.Endpoint == "" {
		return ClientInstance{}, false
	}
	if s.AllowedIPs == "" {
		s.AllowedIPs = "0.0.0.0/0, ::/0"
	}
	if s.MTU == 0 {
		// 1420 = 1500 (typical Ethernet) minus WireGuard/AWG overhead — optimal
		// when the panel host reaches the upstream directly; drop to 1320 for a
		// host that itself sits behind an extra encapsulation hop.
		s.MTU = 1420
	}
	return ClientInstance{
		Id:       o.Id,
		Ifname:   "awgo-" + strconv.Itoa(o.Id),
		Tag:      o.Tag,
		Settings: s,
	}, true
}
