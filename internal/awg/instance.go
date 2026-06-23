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

// Instance is the desired runtime configuration of one AWG inbound: the kernel
// interface, its obfuscation parameters, and the set of peers that should be
// present. The manager drives the running kernel interface toward this state.
type Instance struct {
	Id      int
	Tag     string
	Listen  string
	Port    int
	Ifname  string // e.g. "awg1"
	MTU     int
	DNS     string
	PrivateKey string
	// Obfuscation (matches AWGParams fields; sourced from inbound.Settings).
	Jc   int
	Jmin int
	Jmax int
	S1   int
	S2   int
	S3   int
	S4   int
	H1   string
	H2   string
	H3   string
	H4   string
	I1   string
	I2   string
	I3   string
	I4   string
	I5   string
	// Peers expected on the interface. Each entry maps to one [Peer] in the
	// generated .conf and is reconciled against the kernel state.
	Peers []PeerSpec
	// RouteThroughXray, when set, tells the Xray config builder to inject a
	// TUN inbound for this AWG interface so decrypted packets flow through
	// Xray's routing rules. Mirrors mtproto's RouteThroughXray.
	RouteThroughXray bool
	OutboundTag      string
}

// PeerSpec is one desired peer on an AWG interface.
type PeerSpec struct {
	PublicKey  string // client Curve25519 public key (stored as Client.ID)
	PSK        string // PresharedKey (stored as Client.Password)
	Keepalive  int    // PersistentKeepalive, 0 = off
	AllowedIPs string // comma-separated CIDRs; default "0.0.0.0/0, ::/0"
}

func (inst Instance) bindTo() string {
	listen := inst.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	return listen
}

// fingerprint changes whenever any value that ends up in the generated .conf
// changes, so ensureLocked restarts awg-quick when the operator edits a setting.
func (inst Instance) fingerprint() string {
	parts := []string{
		inst.Ifname,
		strconv.Itoa(inst.Port),
		inst.PrivateKey,
		strconv.Itoa(inst.MTU),
		inst.DNS,
		strconv.Itoa(inst.Jc),
		strconv.Itoa(inst.Jmin),
		strconv.Itoa(inst.Jmax),
		strconv.Itoa(inst.S1),
		strconv.Itoa(inst.S2),
		strconv.Itoa(inst.S3),
		strconv.Itoa(inst.S4),
		inst.H1,
		inst.H2,
		inst.H3,
		inst.H4,
		inst.I1,
		inst.I2,
		inst.I3,
		inst.I4,
		inst.I5,
		strconv.FormatBool(inst.RouteThroughXray),
		inst.OutboundTag,
	}
	for _, p := range inst.Peers {
		parts = append(parts, p.PublicKey, p.PSK, strconv.Itoa(p.Keepalive), p.AllowedIPs)
	}
	return strings.Join(parts, "|")
}

// InstanceFromInbound derives a desired Instance from an AWG inbound. Returns
// false when the inbound is not a usable AWG inbound (wrong protocol, missing
// server key, etc.).
func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.AWG {
		return Instance{}, false
	}
	var s struct {
		PrivateKey string `json:"privateKey"`
		MTU        int    `json:"mtu"`
		DNS        string `json:"dns"`
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
		RouteThroughXray bool   `json:"routeThroughXray"`
		OutboundTag      string `json:"outboundTag"`
		Clients          []struct {
			ID       string `json:"id"`
			Password string `json:"password"`
			Enable   bool   `json:"enable"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
		return Instance{}, false
	}
	if s.PrivateKey == "" {
		return Instance{}, false
	}
	inst := Instance{
		Id:         ib.Id,
		Tag:        ib.Tag,
		Listen:     ib.Listen,
		Port:       ib.Port,
		Ifname:     ifnameFor(ib.Id),
		MTU:        orDefault(s.MTU, 1320),
		DNS:        s.DNS,
		PrivateKey: s.PrivateKey,
		Jc:         s.Jc,
		Jmin:       s.Jmin,
		Jmax:       s.Jmax,
		S1:         s.S1,
		S2:         s.S2,
		S3:         s.S3,
		S4:         s.S4,
		H1:         s.H1,
		H2:         s.H2,
		H3:         s.H3,
		H4:         s.H4,
		I1:         s.I1,
		I2:         s.I2,
		I3:         s.I3,
		I4:         s.I4,
		I5:         s.I5,
		RouteThroughXray: s.RouteThroughXray,
		OutboundTag:      s.OutboundTag,
	}
	for _, c := range s.Clients {
		if c.ID == "" || c.Password == "" || !c.Enable {
			continue
		}
		inst.Peers = append(inst.Peers, PeerSpec{
			PublicKey:  c.ID,
			PSK:        c.Password,
			Keepalive:  25,
			AllowedIPs: "0.0.0.0/0, ::/0",
		})
	}
	return inst, true
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// ifnameFor returns the canonical AWG interface name for an inbound id.
// Linux limits interface names to 15 chars; "awg" + id fits for id < 10^12.
func ifnameFor(id int) string {
	return "awg" + strconv.Itoa(id)
}