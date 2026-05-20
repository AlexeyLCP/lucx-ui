// Copyright (c) 2025 LucX-UI Project.

package awg

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// AddClient registers a peer in AWG kernel via awg addconf.
// Obfuscation comes from inbound.Settings (not regenerated).
func (m *AWGManager) AddClient(awgID int, client *model.Client) error {
	iface := fmt.Sprintf("awg%d", awgID)
	peerConf := fmt.Sprintf("[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = 0.0.0.0/0, ::/0\nPersistentKeepalive = 25\n",
		client.ID, client.Password)
	cmd := exec.Command("awg", "addconf", iface, "/dev/stdin")
	cmd.Stdin = strings.NewReader(peerConf)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("awg addconf: %w\n%s", err, string(out))
	}
	logAWG("AddClient: inbound=%d email=%s", awgID, client.Email)
	return nil
}

// DeleteClient removes a peer from AWG kernel.
func (m *AWGManager) DeleteClient(awgID int, publicKey string) error {
	iface := fmt.Sprintf("awg%d", awgID)
	exec.Command("awg", "set", iface, "peer", publicKey, "remove").Run()
	logAWG("DeleteClient: inbound=%d key=%s...", awgID, publicKey[:min(16, len(publicKey))])
	return nil
}

// EnsureFirstClientExists creates a default client if the inbound has no clients.
// Uses GenKey/DerivePubkey for proper Curve25519 keys.
// Obfuscation comes from inbound.Settings (already stored).
func (m *AWGManager) EnsureFirstClientExists(awg *model.Inbound) error {
	clients, _ := m.InboundService.GetClients(awg)
	if len(clients) > 0 {
		return nil
	}

	privKey := GenKey()
	pubKey := DerivePubkey(privKey)
	if pubKey == "" {
		return fmt.Errorf("DerivePubkey returned empty")
	}
	psk := GenPSK()

	defaultClient := model.Client{
		ID:         pubKey,
		Password:   psk,
		PrivateKey: privKey,
		Email:      fmt.Sprintf("default-%d", awg.Id),
		Enable:     true,
	}

	clientSettings := fmt.Sprintf(
		`{"clients":[{"id":"%s","password":"%s","privateKey":"%s","email":"%s","enable":true,"expiryTime":0,"tgId":"","subId":"","comment":""}]}`,
		defaultClient.ID, defaultClient.Password, defaultClient.PrivateKey, defaultClient.Email,
	)

	clientInbound := &model.Inbound{Id: awg.Id, Settings: clientSettings}
	if _, err := m.InboundService.AddInboundClient(clientInbound); err != nil {
		return fmt.Errorf("add default client: %w", err)
	}

	// Register in kernel
	_ = m.AddClient(awg.Id, &defaultClient)

	logAWG("EnsureFirstClientExists: inbound=%d email=%s", awg.Id, defaultClient.Email)
	return nil
}

func (m *AWGManager) ListClients(awgID int) ([]model.Client, error) {
	awg, err := m.InboundService.GetInbound(awgID)
	if err != nil {
		return nil, err
	}
	return m.InboundService.GetClients(awg)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
