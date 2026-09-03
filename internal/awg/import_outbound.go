// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func BuildOutbound(c ImportCandidate) (*model.AwgOutbound, error) {
	peer := ServerPeer{}
	for _, p := range c.Conf.Peers {
		if strings.TrimSpace(p.Endpoint) != "" && strings.TrimSpace(p.PublicKey) != "" {
			peer = p
			break
		}
	}
	if c.Conf.PrivateKey == "" || c.Conf.Address == "" || peer.Endpoint == "" || peer.PublicKey == "" {
		return nil, errors.New("awg exit conf missing private key, address, endpoint or peer public key")
	}
	s := ClientSettings{
		PrivateKey:             c.Conf.PrivateKey,
		Address:                c.Conf.Address,
		MTU:                    c.Conf.MTU,
		PublicKey:              peer.PublicKey,
		PSK:                    peer.PSK,
		Endpoint:               peer.Endpoint,
		Keepalive:              peer.Keepalive,
		AllowedIPs:             peer.AllowedIPs,
		DNS:                    c.Conf.DNS,
		Jc:                     c.Conf.Jc,
		Jmin:                   c.Conf.Jmin,
		Jmax:                   c.Conf.Jmax,
		S1:                     c.Conf.S1,
		S2:                     c.Conf.S2,
		S3:                     c.Conf.S3,
		S4:                     c.Conf.S4,
		H1:                     c.Conf.H1,
		H2:                     c.Conf.H2,
		H3:                     c.Conf.H3,
		H4:                     c.Conf.H4,
		I1:                     c.Conf.I1,
		I2:                     c.Conf.I2,
		I3:                     c.Conf.I3,
		I4:                     c.Conf.I4,
		I5:                     c.Conf.I5,
		HeaderProtectionKey:    c.Conf.HeaderProtectionKey,
		ContentPaddingAddition: c.Conf.ContentPaddingAddition,
		RekeyAfterTime:         c.Conf.RekeyAfterTime,
		RekeyTimeout:           c.Conf.RekeyTimeout,
		RejectAfterTime:        c.Conf.RejectAfterTime,
		KeepaliveTimeout:       c.Conf.KeepaliveTimeout,
		MaxHandshakeAttempts:   c.Conf.MaxHandshakeAttempts,
		RandomTrailers:         c.Conf.RandomTrailers,
		DisableCookies:         c.Conf.DisableCookies,
		AwgVersion:             c.Conf.AwgVersion,
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	remark := c.Ifname
	if remark == "" {
		remark = "imported-exit"
	}
	return &model.AwgOutbound{
		Remark:   remark,
		Enable:   false,
		Settings: string(raw),
	}, nil
}
