// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// Guards the "one identity, one keypair" rule (awg-cps-facts.md §6): a second
// AWG inbound must reuse a known email's Curve25519 pair, not mint a new one.
func TestAddClient_ReusesStoredKeypairForKnownIdentity(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ibA := mkInbound(t, 21101, model.AmneziaWG, `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)
	ibB := mkInbound(t, 21102, model.AmneziaWG, `{"server":{"subnetIp":"10.9.1.0","subnetCidr":24},"clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "demo@x", SubID: "sub-demo", Enable: true},
		InboundIds: []int{ibA.Id},
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	first := lookupClientRecord(t, "demo@x")
	if first.PrivateKey == "" || first.PublicKey == "" {
		t.Fatalf("first inbound did not mint a keypair: %+v", first)
	}
	derivedPub, err := wgutil.PublicKeyFromPrivate(first.PrivateKey)
	if err != nil {
		t.Fatalf("derive public key from stored private key: %v", err)
	}
	if derivedPub != first.PublicKey {
		t.Fatalf("stored public key %q does not derive from stored private key (got %q)", first.PublicKey, derivedPub)
	}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "demo@x", SubID: "sub-demo", Enable: true},
		InboundIds: []int{ibB.Id},
	}); err != nil {
		t.Fatalf("second Create: %v", err)
	}

	second := lookupClientRecord(t, "demo@x")
	if second.PrivateKey != first.PrivateKey {
		t.Fatalf("private key rotated on second inbound: first %q, second %q", first.PrivateKey, second.PrivateKey)
	}
	if second.PublicKey != first.PublicKey {
		t.Fatalf("public key rotated on second inbound: first %q, second %q", first.PublicKey, second.PublicKey)
	}
	if second.PreSharedKey != first.PreSharedKey {
		t.Fatalf("PSK rotated on second inbound: first %q, second %q", first.PreSharedKey, second.PreSharedKey)
	}

	listB, err := svc.ListForInbound(nil, ibB.Id)
	if err != nil {
		t.Fatalf("ListForInbound ibB: %v", err)
	}
	if len(listB) != 1 || listB[0].PublicKey != first.PublicKey {
		t.Fatalf("inbound B settings carry a different public key than inbound A: %+v", listB)
	}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "stranger@x", SubID: "sub-stranger", Enable: true},
		InboundIds: []int{ibA.Id},
	}); err != nil {
		t.Fatalf("stranger Create: %v", err)
	}
	stranger := lookupClientRecord(t, "stranger@x")
	if stranger.PrivateKey == "" || stranger.PrivateKey == first.PrivateKey {
		t.Fatalf("previously unknown identity did not mint its own fresh keypair: %+v", stranger)
	}

	ibV1 := mkInbound(t, 21103, model.VLESS, `{"clients":[]}`)
	ibV2 := mkInbound(t, 21104, model.VLESS, `{"clients":[]}`)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "plain@x", SubID: "sub-plain", Enable: true},
		InboundIds: []int{ibV1.Id},
	}); err != nil {
		t.Fatalf("keyless first Create: %v", err)
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "plain@x", SubID: "sub-plain", Enable: true},
		InboundIds: []int{ibV2.Id},
	}); err != nil {
		t.Fatalf("keyless second Create: %v", err)
	}
	plain := lookupClientRecord(t, "plain@x")
	if plain.PrivateKey != "" || plain.PublicKey != "" || plain.PreSharedKey != "" {
		t.Fatalf("reuse block populated key columns for a keyless protocol: %+v", plain)
	}
}

// An identity's AWG keypair/PSK decrypt a tunnel; they must not ride along
// into a later-attached keyless protocol's settings JSON (export/backup leak).
func TestCreateClient_KeylessInboundNeverSeesTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	awgIb := mkInbound(t, 21201, model.AWG, `{"address":"10.50.0.1/24","clients":[]}`)
	vlessIb := mkInbound(t, 21202, model.VLESS, `{"clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "mixed@x", SubID: "sub-mixed", Enable: true},
		InboundIds: []int{awgIb.Id},
	}); err != nil {
		t.Fatalf("AWG Create: %v", err)
	}
	identity := lookupClientRecord(t, "mixed@x")
	if identity.PrivateKey == "" || identity.PublicKey == "" {
		t.Fatalf("AWG attach did not mint a keypair: %+v", identity)
	}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "mixed@x", SubID: "sub-mixed", Enable: true},
		InboundIds: []int{vlessIb.Id},
	}); err != nil {
		t.Fatalf("VLESS Create: %v", err)
	}

	vless, err := inboundSvc.GetInbound(vlessIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(vless): %v", err)
	}
	for _, leak := range []string{"privateKey", "publicKey", "preSharedKey"} {
		if strings.Contains(vless.Settings, leak) {
			t.Errorf("VLESS inbound settings must not carry the identity's tunnel key %q, got:\n%s", leak, vless.Settings)
		}
	}

	awg, err := inboundSvc.GetInbound(awgIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(awg): %v", err)
	}
	if !strings.Contains(awg.Settings, `"privateKey"`) {
		t.Errorf("AWG inbound settings must still carry its client's tunnel key, got:\n%s", awg.Settings)
	}
}

// BulkCreate's sibling of the fix above: fillProtocolDefaults mutates the one
// shared prep[idx].client for every target, so a minted or supplied key rides along into a keyless one.
func TestBulkCreateClient_KeylessInboundNeverSeesTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	awgIb := mkInbound(t, 21301, model.AWG, `{"address":"10.60.0.1/24","clients":[]}`)
	vlessIb := mkInbound(t, 21302, model.VLESS, `{"clients":[]}`)

	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}

	payloads := []ClientCreatePayload{
		{
			Client: model.Client{
				Email:      "bulk-mixed@x",
				SubID:      "sub-bulk-mixed",
				Enable:     true,
				PrivateKey: priv,
				PublicKey:  pub,
			},
			InboundIds: []int{awgIb.Id, vlessIb.Id},
		},
	}

	result, _, err := svc.BulkCreate(inboundSvc, payloads)
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if result.Created != 1 {
		t.Fatalf("BulkCreate created = %d, want 1 (skipped: %+v)", result.Created, result.Skipped)
	}

	vless, err := inboundSvc.GetInbound(vlessIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(vless): %v", err)
	}
	for _, leak := range []string{"privateKey", "publicKey", "preSharedKey"} {
		if strings.Contains(vless.Settings, leak) {
			t.Errorf("bulk-created VLESS inbound settings must not carry the identity's tunnel key %q, got:\n%s", leak, vless.Settings)
		}
	}

	awg, err := inboundSvc.GetInbound(awgIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(awg): %v", err)
	}
	if !strings.Contains(awg.Settings, `"privateKey"`) {
		t.Errorf("bulk-created AWG inbound settings must still carry its client's tunnel key, got:\n%s", awg.Settings)
	}
}
