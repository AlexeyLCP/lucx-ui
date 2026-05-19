// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// =============================================================================
// Client Management
// =============================================================================

// AddClient adds a peer to an existing AWG interface (kernel-level).
// The client must already exist in the DB (added via InboundService.AddInboundClient).
func (m *AWGManager) AddClient(awgID int, client *model.Client) error {
	iface := fmt.Sprintf("awg%d", awgID)

	awg, err := m.InboundService.GetInbound(awgID)
	if err != nil {
		return fmt.Errorf("get inbound: %w", err)
	}

	// Determine next client IP
	var settings map[string]interface{}
	json.Unmarshal([]byte(awg.Settings), &settings)
	clients, _ := settings["clients"].([]interface{})
	nextOctet := 2 + len(clients)
	clientIP := fmt.Sprintf("10.%d.0.%d/32", awgID%255, nextOctet)

	// Register with kernel
	cmd := exec.Command("awg", "set", iface,
		"peer", client.ID,
		"preshared-key", client.Password,
		"allowed-ips", clientIP,
		"persistent-keepalive", "25",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("awg set peer: %w\n%s", err, string(out))
	}

	logAWG("AddClient: inbound=%d email=%s ip=%s", awgID, client.Email, clientIP)
	return nil
}

// RemoveClient removes a peer from an AWG interface by public key.
func (m *AWGManager) DeleteClient(awgID int, publicKey string) error {
	iface := fmt.Sprintf("awg%d", awgID)
	cmd := exec.Command("awg", "set", iface, "peer", publicKey, "remove")
	if _, err := cmd.CombinedOutput(); err != nil {
		logAWG("DeleteClient: inbound=%d key=%s... warning: %v", awgID, publicKey[:min(16, len(publicKey))], err)
		// Non-fatal — kernel state may already be cleaned
	}
	logAWG("DeleteClient: inbound=%d key=%s... ok", awgID, publicKey[:min(16, len(publicKey))])
	return nil
}

// EnsureFirstClientExists creates a default client if the inbound has no clients.
// Follows pumbaX convention: server + first peer in one step.
// Uses "default" as the client email.
func (m *AWGManager) EnsureFirstClientExists(awg *model.Inbound) error {
	clients, _ := m.InboundService.GetClients(awg)
	if len(clients) > 0 {
		return nil // already has clients
	}

	privKey := GenKey()
	pubKey := DerivePubkey(privKey)
	defaultClient := model.Client{
		ID:         pubKey,
		Password:   GenPSK(),
		PrivateKey: privKey,
		Email:      fmt.Sprintf("default-%d", awg.Id),
		Enable:     true,
		ExpiryTime: 0,
	}

	clientSettings := fmt.Sprintf(
		`{"clients":[{"id":"%s","password":"%s","privateKey":"%s","email":"%s","enable":true,"expiryTime":0,"tgId":"","subId":"","comment":""}]}`,
		defaultClient.ID, defaultClient.Password, defaultClient.PrivateKey, defaultClient.Email,
	)

	clientInbound := &model.Inbound{Id: awg.Id, Settings: clientSettings}
	if _, err := m.InboundService.AddInboundClient(clientInbound); err != nil {
		return fmt.Errorf("add default client: %w", err)
	}

	// Also register with kernel
	_ = m.AddClient(awg.Id, &defaultClient)

	logAWG("EnsureFirstClientExists: inbound=%d email=%s created", awg.Id, defaultClient.Email)
	return nil
}

// ListClients returns all clients for an AWG inbound.
func (m *AWGManager) ListClients(awgID int) ([]model.Client, error) {
	awg, err := m.InboundService.GetInbound(awgID)
	if err != nil {
		return nil, err
	}
	return m.InboundService.GetClients(awg)
}

// EnableClient enables or disables a client in the kernel interface.
func (m *AWGManager) EnableClient(awgID int, publicKey string, enable bool) error {
	if enable {
		// Re-add the peer (idempotent — awg set updates existing peer)
		awg, err := m.InboundService.GetInbound(awgID)
		if err != nil {
			return err
		}
		clients, _ := m.InboundService.GetClients(awg)
		for _, c := range clients {
			if c.ID == publicKey {
				return m.AddClient(awgID, &c)
			}
		}
		return fmt.Errorf("client %s... not found", publicKey[:16])
	}
	return m.DeleteClient(awgID, publicKey)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
