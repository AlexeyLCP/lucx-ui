// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// TunnelClientCredentials is the username/password pair a client types into a
// NaiveProxy / mieru / TrustTunnel app. Nothing is stored: the sidecar config
// renderer, the share-link builder and this lookup all re-derive the same pair
// from the panel secret, so the panel is the only place an operator can read
// it back. Reported as issue #45 — the client card showed the unrelated
// settings password, and subId/UUID/auth were tried in vain.
type TunnelClientCredentials struct {
	Protocol string `json:"protocol" example:"trusttunnel"`
	Username string `json:"username" example:"tr9f2c1ab0de"`
	Password string `json:"password" example:"kQ8mZ2rC7wN4tX1vB6yH3sL0pJd"`
}

// TunnelClientCredentials derives the pair for one client on one inbound.
// Returns an error for a protocol whose credentials are not derived, so the
// caller never renders a meaningless empty row.
func (s *ClientService) TunnelClientCredentials(inboundId int, email string) (*TunnelClientCredentials, error) {
	email = strings.TrimSpace(email)
	if inboundId <= 0 || email == "" {
		return nil, common.NewError("inbound id and email are required")
	}

	var inbound model.Inbound
	if err := database.GetDB().First(&inbound, inboundId).Error; err != nil {
		return nil, err
	}
	if !tunnelProtocolDerivesCredentials(inbound.Protocol) {
		return nil, common.NewError("inbound protocol does not use derived credentials:", string(inbound.Protocol))
	}
	if !inboundHasEnabledClient(&inbound, email) {
		return nil, common.NewError("client is not attached to this inbound or is disabled:", email)
	}

	secret, err := (&SettingService{}).GetSecret()
	if err != nil {
		return nil, err
	}
	if len(secret) == 0 {
		return nil, common.NewError("panel secret is empty")
	}

	var pair tunnel.AuthPair
	switch inbound.Protocol {
	case model.Naive:
		pair = tunnel.ClientAuth(secret, email)
	case model.Mieru:
		pair = tunnel.MieruClientAuth(secret, inbound.Id, email)
	case model.TrustTunnel:
		pair = tunnel.TrustTunnelClientAuth(secret, inbound.Id, email)
	}

	return &TunnelClientCredentials{
		Protocol: string(inbound.Protocol),
		Username: pair.User,
		Password: pair.Pass,
	}, nil
}

func tunnelProtocolDerivesCredentials(protocol model.Protocol) bool {
	switch protocol {
	case model.Naive, model.Mieru, model.TrustTunnel:
		return true
	}
	return false
}

// inboundHasEnabledClient mirrors what the sidecar renderers do when they
// build the credentials file: a disabled client has no line there, so handing
// its derived pair to an operator would only produce a failing login.
func inboundHasEnabledClient(inbound *model.Inbound, email string) bool {
	var settings struct {
		Clients []struct {
			Email  string `json:"email"`
			Enable bool   `json:"enable"`
		} `json:"clients"`
	}
	if json.Unmarshal([]byte(inbound.Settings), &settings) != nil {
		return false
	}
	for _, c := range settings.Clients {
		if c.Enable && strings.EqualFold(strings.TrimSpace(c.Email), email) {
			return true
		}
	}
	return false
}
