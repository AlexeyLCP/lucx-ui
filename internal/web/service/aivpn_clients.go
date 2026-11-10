// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// AivpnService is the panel side of the aivpn core's management API. The
// core owns the client DB and mints every connection key; this wrapper only
// relays list/add/toggle/remove through the local unix socket. No key
// material is ever generated or rotated here (Rule 0).
type AivpnService struct{}

func (s *AivpnService) apiFor(inboundID int) (*tunnel.AivpnAPI, error) {
	if !(&TunnelService{}).AivpnRunning() {
		return nil, common.NewError("AIVPN core is not running")
	}
	if inboundID > 0 && (&TunnelService{}).AivpnInbound(inboundID) == nil {
		return nil, common.NewError("aivpn inbound not found or disabled")
	}
	socket := tunnel.AivpnSocketPath(tunnel.AivpnKey)
	if _, err := os.Stat(socket); err != nil {
		return nil, common.NewError("AIVPN management socket not found")
	}
	return tunnel.NewAivpnAPI(socket), nil
}

// AivpnAlive reports whether one local enabled aivpn inbound exists and its
// management API answers.
func (s *AivpnService) AivpnAlive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	api, err := s.apiFor(0)
	if err != nil {
		return false
	}
	_, err = api.Status(ctx)
	return err == nil
}

func (s *AivpnService) Clients(inboundID int) ([]tunnel.AivpnClient, error) {
	api, err := s.apiFor(inboundID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, err := api.ListClients(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]tunnel.AivpnClient, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" && strings.TrimSpace(row.ID) == "" {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *AivpnService) Add(inboundID int, name string) (tunnel.AivpnClient, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return tunnel.AivpnClient{}, common.NewError("client name is required")
	}
	if len(name) > 80 {
		return tunnel.AivpnClient{}, common.NewError("client name is too long")
	}
	api, err := s.apiFor(inboundID)
	if err != nil {
		return tunnel.AivpnClient{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return api.AddClient(ctx, name)
}

// Key returns the connection key of one client. The core re-derives it on
// every call — this is the reprint path only (Rule 0: the core owns the
// minting, the panel never generates or stores key material).
func (s *AivpnService) Key(inboundID int, id string) (string, error) {
	api, err := s.apiFor(inboundID)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return api.ClientKey(ctx, id)
}

func (s *AivpnService) SetEnabled(inboundID int, id string, enabled bool) error {
	api, err := s.apiFor(inboundID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return api.SetEnabled(ctx, id, enabled)
}

func (s *AivpnService) Remove(inboundID int, id string) error {
	api, err := s.apiFor(inboundID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return api.RemoveClient(ctx, id)
}
