// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// awgInboundSettings renders an AWG inbound's settings with a tunnel address,
// the subnet defaultAwgClients allocates a newly added peer's address from.
func awgInboundSettings(t *testing.T, address string, clients []model.Client) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"address": address, "clients": clients})
	if err != nil {
		t.Fatalf("marshal awg settings: %v", err)
	}
	return string(b)
}

// A copy gets a fresh email and fresh credentials, so it is a new identity —
// it must not ride in on the source's Curve25519 private key.
func TestCopyInboundClients_CopyGetsItsOwnTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	psk, err := wgutil.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("psk: %v", err)
	}
	donor := model.Client{
		Email: "donor@x", SubID: "sub-donor", Enable: true,
		PrivateKey: priv, PublicKey: pub, PreSharedKey: psk,
		AllowedIPs: []string{"10.80.0.2/32"},
	}
	source := mkInbound(t, 21901, model.AWG, awgInboundSettings(t, "10.80.0.1/24", []model.Client{donor}))
	if err := svc.SyncInbound(nil, source.Id, []model.Client{donor}); err != nil {
		t.Fatalf("seed source linkage: %v", err)
	}
	awgTarget := mkInbound(t, 21902, model.AWG, awgInboundSettings(t, "10.81.0.1/24", []model.Client{}))
	vlessTarget := mkInbound(t, 21903, model.VLESS, `{"clients":[]}`)

	copyDonor := func(t *testing.T, targetID int) string {
		t.Helper()
		res, _, err := inboundSvc.CopyInboundClients(targetID, source.Id, []string{donor.Email}, "")
		if err != nil {
			t.Fatalf("CopyInboundClients(-> %d): %v", targetID, err)
		}
		if len(res.Errors) != 0 {
			t.Fatalf("CopyInboundClients(-> %d) reported errors: %v", targetID, res.Errors)
		}
		if len(res.Added) != 1 {
			t.Fatalf("CopyInboundClients(-> %d) added %v, want exactly one email", targetID, res.Added)
		}
		return res.Added[0]
	}

	t.Run("awg copy gets a keypair of its own", func(t *testing.T) {
		copied := copyDonor(t, awgTarget.Id)
		peer := awgPeer(t, inboundSvc, awgTarget.Id, copied)
		if peer.PrivateKey == "" || peer.PublicKey == "" {
			t.Fatalf("copy %q reached inbound %d without a keypair: %+v", copied, awgTarget.Id, peer)
		}
		if peer.PrivateKey == priv {
			t.Errorf("copy %q got the source's private key: source %q, copy %q — two identities, one secret", copied, priv, peer.PrivateKey)
		}
		if peer.PublicKey == pub {
			t.Errorf("copy %q got the source's public key: source %q, copy %q", copied, pub, peer.PublicKey)
		}
		if peer.PreSharedKey == psk {
			t.Errorf("copy %q got the source's PSK: source %q, copy %q", copied, psk, peer.PreSharedKey)
		}
	})

	t.Run("keyless target never sees tunnel keys", func(t *testing.T) {
		copied := copyDonor(t, vlessTarget.Id)
		ib, err := inboundSvc.GetInbound(vlessTarget.Id)
		if err != nil {
			t.Fatalf("GetInbound(%d): %v", vlessTarget.Id, err)
		}
		for _, field := range []string{"privateKey", "publicKey", "preSharedKey"} {
			if strings.Contains(ib.Settings, field) {
				t.Errorf("copy %q leaked %q into VLESS inbound %d settings: %s", copied, field, vlessTarget.Id, ib.Settings)
			}
		}
		for _, secret := range []string{priv, psk} {
			if strings.Contains(ib.Settings, secret) {
				t.Errorf("copy %q leaked the source secret %q into VLESS inbound %d settings", copied, secret, vlessTarget.Id)
			}
		}
	})

	t.Run("source peer keeps its own keys", func(t *testing.T) {
		peer := awgPeer(t, inboundSvc, source.Id, donor.Email)
		if peer.PrivateKey != priv || peer.PublicKey != pub || peer.PreSharedKey != psk {
			t.Errorf("copying rotated the source peer: want priv %q pub %q psk %q, got priv %q pub %q psk %q",
				priv, pub, psk, peer.PrivateKey, peer.PublicKey, peer.PreSharedKey)
		}
	})
}
