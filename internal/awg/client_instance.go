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
	Keepalive  int    `json:"keepalive"`
	AllowedIPs string `json:"allowedIPs"`
	DNS        string `json:"dns"` // optional, only written if non-empty
	Jc         int    `json:"jc"`
	Jmin       int    `json:"jmin"`
	Jmax       int    `json:"jmax"`
	S1         int    `json:"s1"`
	S2         int    `json:"s2"`
	S3         int    `json:"s3"`
	S4         int    `json:"s4"`
	H1         string `json:"h1"`
	H2         string `json:"h2"`
	H3         string `json:"h3"`
	H4         string `json:"h4"`
	I1         string `json:"i1"`
	I2         string `json:"i2"`
	I3         string `json:"i3"`
	I4         string `json:"i4"`
	I5         string `json:"i5"`
	// AWG3 (AmneziaWG 3) header protection key — 32-byte ChaCha20, base64.
	// Written to the .conf only when AwgVersion == "3" (the outbound opts into
	// AWG3); for older versions it stays empty. Upstream kernel v3.0.20260731 +
	// tools v3.0.20260730 parse the field.
	HeaderProtectionKey string `json:"headerProtectionKey"`
	// AWG3 device-level timers/padding (0 = kernel default). Written to .conf
	// only when > 0 and AwgVersion == "3".
	ContentPaddingAddition int `json:"contentPaddingAddition"`
	RekeyAfterTime         int `json:"rekeyAfterTime"`
	RekeyTimeout           int `json:"rekeyTimeout"`
	RejectAfterTime        int `json:"rejectAfterTime"`
	KeepaliveTimeout       int `json:"keepaliveTimeout"`
	MaxHandshakeAttempts   int `json:"maxHandshakeAttempts"`
	// AwgVersion targets the AmneziaWG protocol version for this outbound
	// ("1.5"/"2"/"3"; "" → "2" via normalize). The .conf renderer emits
	// HeaderProtectionKey only for version "3".
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
		s.MTU = 1320
	}
	return ClientInstance{
		Id:       o.Id,
		Ifname:   "awgo-" + strconv.Itoa(o.Id),
		Tag:      o.Tag,
		Settings: s,
	}, true
}

// fingerprint returns a stable string that changes whenever any value that
// ends up in the generated .conf changes, so EnsureClient can detect when to
// restart awg-quick. Mirrors Instance.fingerprint.
func (ci ClientInstance) fingerprint() string {
	s := ci.Settings
	parts := []string{
		ci.Ifname,
		s.PrivateKey,
		s.Address,
		strconv.Itoa(s.MTU),
		s.PublicKey,
		s.PSK,
		s.Endpoint,
		strconv.Itoa(s.Keepalive),
		s.AllowedIPs,
		s.DNS,
		strconv.Itoa(s.Jc),
		strconv.Itoa(s.Jmin),
		strconv.Itoa(s.Jmax),
		strconv.Itoa(s.S1),
		strconv.Itoa(s.S2),
		strconv.Itoa(s.S3),
		strconv.Itoa(s.S4),
		s.H1, s.H2, s.H3, s.H4,
		s.I1, s.I2, s.I3, s.I4, s.I5,
		s.HeaderProtectionKey,
		s.AwgVersion,
		strconv.Itoa(s.ContentPaddingAddition),
		strconv.Itoa(s.RekeyAfterTime),
		strconv.Itoa(s.RekeyTimeout),
		strconv.Itoa(s.RejectAfterTime),
		strconv.Itoa(s.KeepaliveTimeout),
		strconv.Itoa(s.MaxHandshakeAttempts),
	}
	return strings.Join(parts, "|")
}
