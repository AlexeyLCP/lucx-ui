// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// ErrDuplicateOutboundTag is returned when an AWG outbound's Tag collides
// with an existing Xray outbound, AWG outbound, or system tag (direct/block/api).
var ErrDuplicateOutboundTag = errors.New("awg-outbound: duplicate outbound tag")

// AwgOutboundService handles CRUD for client-mode AmneziaWG outbounds.
type AwgOutboundService struct{}

// defaultAwgOutboundSettings generates a fresh client keypair and returns the
// Settings JSON for a new AWG outbound (operator still must fill Address,
// PublicKey, Endpoint upstream-side). PrivateKey is generated via
// x/crypto/curve25519 (same path as defaultAwgClients), not random bytes. Both
// the private and the derived public key are stored so the operator can copy
// the public key when adding this client as a peer on the upstream server.
func defaultAwgOutboundSettings() string {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		return "{}"
	}
	s := map[string]any{
		"privateKey": priv,
		"publicKey":  pub,
		"mtu":        1320,
		"keepalive":  25,
		"allowedIPs": "0.0.0.0/0, ::/0",
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// checkTagUnique returns ErrDuplicateOutboundTag if tag is already used by
// another AWG outbound (other than ignoreId), or matches a system tag.
// Caller is responsible for cross-checking against XrayConfig outbounds when
// injecting — this only checks the AWG table + system tags.
func checkTagUnique(tag string, ignoreId int) error {
	if tag == "direct" || tag == "block" || tag == "api" {
		return fmt.Errorf("%w: tag %q is reserved", ErrDuplicateOutboundTag, tag)
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.AwgOutbound{}).
		Where("tag = ? AND id != ?", tag, ignoreId).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: tag %q already used by another AWG outbound", ErrDuplicateOutboundTag, tag)
	}
	return nil
}

// AddOutbound persists a new AWG outbound row. If Settings is empty, fills in
// a default keypair via defaultAwgOutboundSettings. Tag uniqueness is enforced.
// When the operator supplied a non-empty Tag it is kept; otherwise the Tag is
// auto-generated as "awgo-{Id}" after the row is created (the Id isn't known
// until after Create) and persisted with an Update.
func (s *AwgOutboundService) AddOutbound(o *model.AwgOutbound) (*model.AwgOutbound, error) {
	tag := strings.TrimSpace(o.Tag)
	if tag == "" {
		// Defer tag uniqueness check until after auto-generation, since the
		// auto-generated "awgo-{Id}" tag is unique by construction (Id is the
		// primary key). We still re-check below for safety in case of races.
		o.Tag = ""
	} else {
		o.Tag = tag
		if err := checkTagUnique(o.Tag, 0); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(o.Settings) == "" {
		o.Settings = defaultAwgOutboundSettings()
	}
	db := database.GetDB()
	if err := db.Create(o).Error; err != nil {
		return nil, err
	}
	if o.Tag == "" {
		o.Tag = "awgo-" + strconv.Itoa(o.Id)
		if err := db.Model(o).Update("tag", o.Tag).Error; err != nil {
			return nil, err
		}
		// Re-run uniqueness against the now-final, auto-generated tag. This is
		// almost always a no-op (awgo-{Id} is unique), but guards against a
		// hand-edited DB where another row already holds "awgo-{Id}".
		if err := checkTagUnique(o.Tag, o.Id); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (s *AwgOutboundService) DelOutbound(id int) error {
	db := database.GetDB()
	return db.Delete(&model.AwgOutbound{}, id).Error
}

func (s *AwgOutboundService) UpdateOutbound(o *model.AwgOutbound) error {
	if err := checkTagUnique(o.Tag, o.Id); err != nil {
		return err
	}
	db := database.GetDB()
	return db.Save(o).Error
}

func (s *AwgOutboundService) SetOutboundEnable(id int, enable bool) error {
	db := database.GetDB()
	return db.Model(&model.AwgOutbound{}).Where("id = ?", id).Update("enable", enable).Error
}

func (s *AwgOutboundService) GetOutbounds() ([]*model.AwgOutbound, error) {
	db := database.GetDB()
	var out []*model.AwgOutbound
	err := db.Order("id ASC").Find(&out).Error
	return out, err
}

func (s *AwgOutboundService) GetOutbound(id int) (*model.AwgOutbound, error) {
	db := database.GetDB()
	o := &model.AwgOutbound{}
	if err := db.First(o, id).Error; err != nil {
		return nil, err
	}
	return o, nil
}

// ParseConf parses a pasted awg-quick .conf and returns ClientSettings. Used by
// the "Paste .conf" UI drawer to autofill the form. Tolerates whitespace and
// lines without values. Does NOT validate mandatory fields (caller does).
// Exported so the controller (Task 6) can call service.ParseConf.
func ParseConf(text string) (awg.ClientSettings, error) {
	var s awg.ClientSettings
	section := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch section {
		case "interface":
			switch key {
			case "PrivateKey":
				s.PrivateKey = val
			case "Address":
				s.Address = val
			case "MTU":
				s.MTU, _ = strconv.Atoi(val)
			case "DNS":
				s.DNS = val
			case "Jc":
				s.Jc, _ = strconv.Atoi(val)
			case "Jmin":
				s.Jmin, _ = strconv.Atoi(val)
			case "Jmax":
				s.Jmax, _ = strconv.Atoi(val)
			case "S1":
				s.S1, _ = strconv.Atoi(val)
			case "S2":
				s.S2, _ = strconv.Atoi(val)
			case "S3":
				s.S3, _ = strconv.Atoi(val)
			case "S4":
				s.S4, _ = strconv.Atoi(val)
			case "H1":
				s.H1 = val
			case "H2":
				s.H2 = val
			case "H3":
				s.H3 = val
			case "H4":
				s.H4 = val
			case "I1":
				s.I1 = val
			case "I2":
				s.I2 = val
			case "I3":
				s.I3 = val
			case "I4":
				s.I4 = val
			case "I5":
				s.I5 = val
			}
		case "peer":
			switch key {
			case "PublicKey":
				s.PublicKey = val
			case "PresharedKey":
				s.PSK = val
			case "Endpoint":
				s.Endpoint = val
			case "AllowedIPs":
				s.AllowedIPs = val
			case "PersistentKeepalive":
				s.Keepalive, _ = strconv.Atoi(val)
			}
		}
	}
	return s, nil
}