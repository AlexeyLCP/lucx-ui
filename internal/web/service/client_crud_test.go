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

// awgPeerPublicKey reads the key the inbound's own settings JSON hands the
// server, which is what a desynced identity makes diverge from the record.
func awgPeerPublicKey(t *testing.T, inboundSvc *InboundService, inboundId int, email string) string {
	t.Helper()
	ib, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		t.Fatalf("GetInbound(%d): %v", inboundId, err)
	}
	var parsed struct {
		Clients []struct {
			Email     string `json:"email"`
			PublicKey string `json:"publicKey"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		t.Fatalf("parse inbound %d settings: %v (%s)", inboundId, err, ib.Settings)
	}
	for _, c := range parsed.Clients {
		if c.Email == email {
			return c.PublicKey
		}
	}
	t.Fatalf("inbound %d settings carry no client %q: %s", inboundId, email, ib.Settings)
	return ""
}

// Guards the "one identity, one keypair" rule (awg-cps-facts.md §6): a second
// AWG inbound must reuse a known email's Curve25519 pair, not mint a new one.
func TestAddClient_ReusesStoredKeypairForKnownIdentity(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ibA := mkInbound(t, 21101, model.AmneziaWG, `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)
	ibB := mkInbound(t, 21102, model.AmneziaWG, `{"server":{"subnetIp":"10.9.1.0","subnetCidr":24},"clients":[]}`)

	// AmneziaWG mints a keypair but never a PSK (unlike AWG), so the first
	// attach has to supply one or there is nothing for the re-add to carry.
	seedPSK, err := wgutil.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("psk: %v", err)
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "demo@x", SubID: "sub-demo", Enable: true, PreSharedKey: seedPSK},
		InboundIds: []int{ibA.Id},
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	first := lookupClientRecord(t, "demo@x")
	if first.PrivateKey == "" || first.PublicKey == "" || first.PreSharedKey != seedPSK {
		t.Fatalf("first inbound did not store a keypair and the supplied PSK: %+v", first)
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
	// A blank PSK never clears the record column, so only inbound B's own
	// settings show whether the second peer was actually handed the PSK.
	ibBSaved, err := inboundSvc.GetInbound(ibB.Id)
	if err != nil {
		t.Fatalf("GetInbound(ibB): %v", err)
	}
	if !strings.Contains(ibBSaved.Settings, first.PreSharedKey) {
		t.Fatalf("inbound %d peer carries no PSK, so it desyncs from inbound %d: %s", ibB.Id, ibA.Id, ibBSaved.Settings)
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

// Same "one identity, one keypair" rule on the bulk path, which ImportClients
// also delegates to: a re-add must reuse the stored keys, not mint fresh ones.
func TestBulkCreateClient_ReusesStoredKeypairForKnownIdentity(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ibA := mkInbound(t, 21501, model.AWG, `{"address":"10.80.0.1/24","clients":[]}`)
	ibB := mkInbound(t, 21502, model.AWG, `{"address":"10.81.0.1/24","clients":[]}`)
	ibC := mkInbound(t, 21503, model.AWG, `{"address":"10.82.0.1/24","clients":[]}`)

	bulkAdd := func(c model.Client, inboundId int) {
		t.Helper()
		res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{{Client: c, InboundIds: []int{inboundId}}})
		if err != nil {
			t.Fatalf("BulkCreate(%s -> inbound %d): %v", c.Email, inboundId, err)
		}
		if res.Created != 1 {
			t.Fatalf("BulkCreate(%s -> inbound %d) created = %d, want 1 (skipped: %+v)", c.Email, inboundId, res.Created, res.Skipped)
		}
	}

	bulkAdd(model.Client{Email: "demo@x", SubID: "sub-demo", Enable: true}, ibA.Id)
	first := lookupClientRecord(t, "demo@x")
	if first.PrivateKey == "" || first.PublicKey == "" || first.PreSharedKey == "" {
		t.Fatalf("first bulk add minted an incomplete key set: %+v", first)
	}

	bulkAdd(model.Client{Email: "demo@x", SubID: "sub-demo", Enable: true}, ibB.Id)
	second := lookupClientRecord(t, "demo@x")
	if second.PrivateKey != first.PrivateKey {
		t.Errorf("private key rotated on bulk re-add of a known identity: first %q, second %q", first.PrivateKey, second.PrivateKey)
	}
	if second.PublicKey != first.PublicKey {
		t.Errorf("public key rotated on bulk re-add of a known identity: first %q, second %q", first.PublicKey, second.PublicKey)
	}
	if second.PreSharedKey != first.PreSharedKey {
		t.Errorf("PSK rotated on bulk re-add of a known identity: first %q, second %q", first.PreSharedKey, second.PreSharedKey)
	}
	pubA := awgPeerPublicKey(t, inboundSvc, ibA.Id, "demo@x")
	pubB := awgPeerPublicKey(t, inboundSvc, ibB.Id, "demo@x")
	if pubA != pubB {
		t.Errorf("one identity, two peer public keys: inbound %d holds %q, inbound %d holds %q — one of the two servers waits for a key the subscription never hands out", ibA.Id, pubA, ibB.Id, pubB)
	}

	bulkAdd(model.Client{Email: "stranger@x", SubID: "sub-stranger", Enable: true}, ibA.Id)
	stranger := lookupClientRecord(t, "stranger@x")
	if stranger.PrivateKey == "" || stranger.PublicKey == "" || stranger.PreSharedKey == "" {
		t.Errorf("previously unknown identity got no key set of its own: %+v", stranger)
	}
	if stranger.PrivateKey == first.PrivateKey || stranger.PublicKey == first.PublicKey || stranger.PreSharedKey == first.PreSharedKey {
		t.Errorf("previously unknown identity was handed the known identity's keys: %+v", stranger)
	}

	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	psk, err := wgutil.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("psk: %v", err)
	}
	bulkAdd(model.Client{
		Email: "demo@x", SubID: "sub-demo", Enable: true,
		PrivateKey: priv, PublicKey: pub, PreSharedKey: psk,
	}, ibC.Id)
	if got := awgPeerPublicKey(t, inboundSvc, ibC.Id, "demo@x"); got != pub {
		t.Errorf("a supplied public key was overwritten by the stored one: sent %q, inbound %d holds %q", pub, ibC.Id, got)
	}
	third := lookupClientRecord(t, "demo@x")
	if third.PrivateKey != priv {
		t.Errorf("a supplied private key was overwritten by the stored one: sent %q, stored %q", priv, third.PrivateKey)
	}
	if third.PreSharedKey != psk {
		t.Errorf("a supplied PSK was overwritten by the stored one: sent %q, stored %q", psk, third.PreSharedKey)
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

// Attach reuses an existing identity, whose ClientRecord already holds the
// tunnel keypair and PSK; a keyless target must not be handed them.
func TestAttach_KeylessInboundNeverSeesTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	homeIb := mkInbound(t, 21401, model.AWG, `{"address":"10.70.0.1/24","clients":[]}`)
	vlessIb := mkInbound(t, 21402, model.VLESS, `{"clients":[]}`)
	awgIb := mkInbound(t, 21403, model.AWG, `{"address":"10.71.0.1/24","clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "attach-mixed@x", SubID: "sub-attach-mixed", Enable: true},
		InboundIds: []int{homeIb.Id},
	}); err != nil {
		t.Fatalf("AWG Create: %v", err)
	}
	identity := lookupClientRecord(t, "attach-mixed@x")
	if identity.PrivateKey == "" || identity.PreSharedKey == "" {
		t.Fatalf("AWG create did not mint a keypair and PSK: %+v", identity)
	}

	// Keyless target first, tunnel target second: a strip applied to the shared
	// clientWire instead of the per-inbound copy would empty the AWG one below.
	if _, err := svc.Attach(inboundSvc, identity.Id, []int{vlessIb.Id, awgIb.Id}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	vless, err := inboundSvc.GetInbound(vlessIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(vless): %v", err)
	}
	for _, leak := range []string{"privateKey", "publicKey", "preSharedKey"} {
		if strings.Contains(vless.Settings, leak) {
			t.Errorf("attached VLESS inbound settings must not carry the identity's tunnel key %q, got:\n%s", leak, vless.Settings)
		}
	}

	awg, err := inboundSvc.GetInbound(awgIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(awg): %v", err)
	}
	if !strings.Contains(awg.Settings, identity.PrivateKey) {
		t.Errorf("attached AWG inbound settings must still carry the identity's tunnel key, got:\n%s", awg.Settings)
	}
	if !strings.Contains(awg.Settings, identity.PreSharedKey) {
		t.Errorf("attached AWG inbound settings must still carry the identity's PSK, got:\n%s", awg.Settings)
	}
}

// BulkAttach's sibling of the fix above: ToClient copies the identity's tunnel
// keypair and PSK onto every target, keyless ones included.
func TestBulkAttach_KeylessInboundNeverSeesTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	homeIb := mkInbound(t, 21501, model.AWG, `{"address":"10.80.0.1/24","clients":[]}`)
	vlessIb := mkInbound(t, 21502, model.VLESS, `{"clients":[]}`)
	awgIb := mkInbound(t, 21503, model.AWG, `{"address":"10.81.0.1/24","clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "bulk-attach-mixed@x", SubID: "sub-bulk-attach-mixed", Enable: true},
		InboundIds: []int{homeIb.Id},
	}); err != nil {
		t.Fatalf("AWG Create: %v", err)
	}
	identity := lookupClientRecord(t, "bulk-attach-mixed@x")
	if identity.PrivateKey == "" || identity.PreSharedKey == "" {
		t.Fatalf("AWG create did not mint a keypair and PSK: %+v", identity)
	}

	// Keyless target first, tunnel target second — see the Attach sibling above.
	res, _, err := svc.BulkAttach(inboundSvc, []string{"bulk-attach-mixed@x"}, []int{vlessIb.Id, awgIb.Id})
	if err != nil {
		t.Fatalf("BulkAttach: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("BulkAttach errors: %v", res.Errors)
	}
	if len(res.Attached) != 2 {
		t.Fatalf("BulkAttach attached = %v, want both targets", res.Attached)
	}

	vless, err := inboundSvc.GetInbound(vlessIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(vless): %v", err)
	}
	for _, leak := range []string{"privateKey", "publicKey", "preSharedKey"} {
		if strings.Contains(vless.Settings, leak) {
			t.Errorf("bulk-attached VLESS inbound settings must not carry the identity's tunnel key %q, got:\n%s", leak, vless.Settings)
		}
	}

	awg, err := inboundSvc.GetInbound(awgIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(awg): %v", err)
	}
	if !strings.Contains(awg.Settings, identity.PrivateKey) {
		t.Errorf("bulk-attached AWG inbound settings must still carry the identity's tunnel key, got:\n%s", awg.Settings)
	}
	if !strings.Contains(awg.Settings, identity.PreSharedKey) {
		t.Errorf("bulk-attached AWG inbound settings must still carry the identity's PSK, got:\n%s", awg.Settings)
	}
}

// The keyless copy of an identity is stripped of its PSK, so the merge onto
// the shared row must not read that blank as "the operator cleared it".
func TestCreateClient_KeylessAttachKeepsIdentityPSK(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	serverPriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	awgIb := mkInbound(t, 21401, model.AWG,
		`{"privateKey":"`+serverPriv+`","address":"10.70.0.1/24","clients":[]}`)
	vlessIb := mkInbound(t, 21402, model.VLESS, `{"clients":[]}`)

	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	psk, err := wgutil.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("psk: %v", err)
	}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: "psk-mixed@x", SubID: "sub-psk-mixed", Enable: true,
			PrivateKey: priv, PublicKey: pub, PreSharedKey: psk,
		},
		InboundIds: []int{awgIb.Id, vlessIb.Id},
	}); err != nil {
		t.Fatalf("Create across AWG + VLESS: %v", err)
	}

	if got := lookupClientRecord(t, "psk-mixed@x").PreSharedKey; got != psk {
		t.Fatalf("identity PSK = %q, want %q — the keyless attach erased the shared secret", got, psk)
	}

	// The subscription server reads the normalized row, not the AWG inbound's
	// settings, so an erased column silently drops PresharedKey from the .conf.
	subClients, err := svc.ListForInboundBySubId(nil, awgIb.Id, "sub-psk-mixed")
	if err != nil {
		t.Fatalf("ListForInboundBySubId: %v", err)
	}
	if len(subClients) != 1 {
		t.Fatalf("subscription sees %d clients on the AWG inbound, want 1", len(subClients))
	}
	awgIn, err := inboundSvc.GetInbound(awgIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(awg): %v", err)
	}
	conf, err := BuildAwgClientConf(awgIn, &subClients[0], "203.0.113.9")
	if err != nil {
		t.Fatalf("BuildAwgClientConf: %v", err)
	}
	if !strings.Contains(conf, "PresharedKey = "+psk) {
		t.Fatalf("client .conf lost PresharedKey while the server peer keeps it, got:\n%s", conf)
	}
}
