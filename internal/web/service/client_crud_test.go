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

// awgPeer reads the peer the inbound's own settings JSON hands the server. The
// record column hides a loss: a blank incoming key never clears a stored one.
func awgPeer(t *testing.T, inboundSvc *InboundService, inboundId int, email string) model.Client {
	t.Helper()
	ib, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		t.Fatalf("GetInbound(%d): %v", inboundId, err)
	}
	var parsed struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		t.Fatalf("parse inbound %d settings: %v (%s)", inboundId, err, ib.Settings)
	}
	for _, c := range parsed.Clients {
		if c.Email == email {
			return c
		}
	}
	t.Fatalf("inbound %d settings carry no client %q: %s", inboundId, email, ib.Settings)
	return model.Client{}
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

// One call naming two tunnel inbounds must mint ONE keypair: the subscription
// is built from the record, so a second pair leaves that peer never handshaking.
func TestCreateClient_TwoTunnelInboundsShareOneKeypair(t *testing.T) {
	const awgSettings = `{"address":"10.90.0.1/24","clients":[]}`
	const amneziaSettings = `{"server":{"subnetIp":"10.91.1.0","subnetCidr":24},"clients":[]}`
	cases := []struct {
		name                 string
		protoA, protoB       model.Protocol
		settingsA, settingsB string
	}{
		{"two AWG inbounds", model.AWG, model.AWG, awgSettings, `{"address":"10.92.0.1/24","clients":[]}`},
		{"two AmneziaWG inbounds", model.AmneziaWG, model.AmneziaWG, amneziaSettings, `{"server":{"subnetIp":"10.93.1.0","subnetCidr":24},"clients":[]}`},
		{"two WireGuard inbounds", model.WireGuard, model.WireGuard, wgServerSettings(), wgServerSettings()},
		{"AWG plus AmneziaWG", model.AWG, model.AmneziaWG, awgSettings, amneziaSettings},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			ibA := mkInbound(t, 21701, tc.protoA, tc.settingsA)
			ibB := mkInbound(t, 21702, tc.protoB, tc.settingsB)

			if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
				Client:     model.Client{Email: "twin@x", SubID: "sub-twin", Enable: true},
				InboundIds: []int{ibA.Id, ibB.Id},
			}); err != nil {
				t.Fatalf("Create across two tunnel inbounds: %v", err)
			}

			peerA := awgPeer(t, inboundSvc, ibA.Id, "twin@x")
			peerB := awgPeer(t, inboundSvc, ibB.Id, "twin@x")
			record := lookupClientRecord(t, "twin@x")
			if peerA.PublicKey == "" || peerB.PublicKey == "" {
				t.Fatalf("one of the two tunnel inbounds got no public key at all: %+v / %+v", peerA, peerB)
			}
			if peerA.PublicKey != peerB.PublicKey || record.PublicKey != peerA.PublicKey {
				t.Errorf("one Create minted a keypair per inbound: inbound %d holds public key %q, inbound %d holds %q, the client record holds %q — the subscription carries the record's key, so the other inbound's handshake silently never completes",
					ibA.Id, peerA.PublicKey, ibB.Id, peerB.PublicKey, record.PublicKey)
			}
			if peerA.PrivateKey != peerB.PrivateKey {
				t.Errorf("private key differs between inbound %d (%q) and inbound %d (%q)", ibA.Id, peerA.PrivateKey, ibB.Id, peerB.PrivateKey)
			}
		})
	}
}

// Minting once before the loop must not leak into a keyless target's settings:
// the per-inbound copy is what clearForeignTunnelKeys strips, not the identity.
func TestCreateClient_MintedKeypairSkipsKeylessInboundInSameCall(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	awgIb := mkInbound(t, 21711, model.AWG, `{"address":"10.92.0.1/24","clients":[]}`)
	vlessIb := mkInbound(t, 21712, model.VLESS, `{"clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "minted-mixed@x", SubID: "sub-minted-mixed", Enable: true},
		InboundIds: []int{awgIb.Id, vlessIb.Id},
	}); err != nil {
		t.Fatalf("Create across AWG + VLESS: %v", err)
	}

	peer := awgPeer(t, inboundSvc, awgIb.Id, "minted-mixed@x")
	if peer.PrivateKey == "" || peer.PublicKey == "" {
		t.Errorf("AWG inbound %d got no keypair: %+v", awgIb.Id, peer)
	}
	vless, err := inboundSvc.GetInbound(vlessIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(vless): %v", err)
	}
	for _, leak := range []string{"privateKey", "publicKey", "preSharedKey"} {
		if strings.Contains(vless.Settings, leak) {
			t.Errorf("the minted tunnel key %q reached the VLESS inbound named in the same call:\n%s", leak, vless.Settings)
		}
	}
}

// bulkCreateOne runs the bulk path for a single client and fails on a skip, so
// callers assert on what the inbounds got rather than on the result counter.
func bulkCreateOne(t *testing.T, svc *ClientService, inboundSvc *InboundService, c model.Client, inboundIds ...int) {
	t.Helper()
	res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{{Client: c, InboundIds: inboundIds}})
	if err != nil {
		t.Fatalf("BulkCreate(%s -> inbounds %v): %v", c.Email, inboundIds, err)
	}
	if res.Created != 1 {
		t.Fatalf("BulkCreate(%s -> inbounds %v) created = %d, want 1 (skipped: %+v)", c.Email, inboundIds, res.Created, res.Skipped)
	}
}

// The bulk sibling of TestCreateClient_TwoTunnelInboundsShareOneKeypair, which
// ImportClients also delegates to: one payload, two tunnel inbounds, one pair.
func TestBulkCreateClient_TwoTunnelInboundsShareOneKeypair(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ibA := mkInbound(t, 21721, model.AWG, `{"address":"10.93.0.1/24","clients":[]}`)
	ibB := mkInbound(t, 21722, model.AWG, `{"address":"10.94.0.1/24","clients":[]}`)

	bulkCreateOne(t, svc, inboundSvc, model.Client{Email: "bulk-twin@x", SubID: "sub-bulk-twin", Enable: true}, ibA.Id, ibB.Id)

	peerA := awgPeer(t, inboundSvc, ibA.Id, "bulk-twin@x")
	peerB := awgPeer(t, inboundSvc, ibB.Id, "bulk-twin@x")
	record := lookupClientRecord(t, "bulk-twin@x")
	if peerA.PublicKey == "" || peerB.PublicKey == "" {
		t.Fatalf("one of the two AWG inbounds got no public key at all: %+v / %+v", peerA, peerB)
	}
	if peerA.PublicKey != peerB.PublicKey || record.PublicKey != peerA.PublicKey {
		t.Errorf("one BulkCreate payload minted a keypair per inbound: inbound %d holds public key %q, inbound %d holds %q, the client record holds %q — the subscription carries the record's key, so the other inbound's handshake silently never completes",
			ibA.Id, peerA.PublicKey, ibB.Id, peerB.PublicKey, record.PublicKey)
	}
	if peerA.PrivateKey != peerB.PrivateKey {
		t.Errorf("private key differs between inbound %d (%q) and inbound %d (%q)", ibA.Id, peerA.PrivateKey, ibB.Id, peerB.PrivateKey)
	}
}

// Same "one identity, one keypair" rule on the bulk path, which ImportClients
// also delegates to: a re-add must reuse the stored keys, not mint fresh ones.
func TestBulkCreateClient_ReusesStoredKeypairForKnownIdentity(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ibA := mkInbound(t, 21601, model.AWG, `{"address":"10.80.0.1/24","clients":[]}`)
	ibB := mkInbound(t, 21602, model.AWG, `{"address":"10.81.0.1/24","clients":[]}`)
	ibC := mkInbound(t, 21603, model.AWG, `{"address":"10.82.0.1/24","clients":[]}`)

	bulkCreateOne(t, svc, inboundSvc, model.Client{Email: "demo@x", SubID: "sub-demo", Enable: true}, ibA.Id)
	peerA := awgPeer(t, inboundSvc, ibA.Id, "demo@x")
	if peerA.PrivateKey == "" || peerA.PublicKey == "" || peerA.PreSharedKey == "" {
		t.Fatalf("first bulk add gave inbound %d an incomplete key set: %+v", ibA.Id, peerA)
	}

	bulkCreateOne(t, svc, inboundSvc, model.Client{Email: "demo@x", SubID: "sub-demo", Enable: true}, ibB.Id)
	peerB := awgPeer(t, inboundSvc, ibB.Id, "demo@x")
	for _, f := range []struct{ field, onA, onB string }{
		{"private key", peerA.PrivateKey, peerB.PrivateKey},
		{"public key", peerA.PublicKey, peerB.PublicKey},
		{"preshared key", peerA.PreSharedKey, peerB.PreSharedKey},
	} {
		if f.onA != f.onB {
			t.Errorf("%s rotated on bulk re-add of a known identity: inbound %d holds %q, inbound %d holds %q — one server waits for a key the other end never uses",
				f.field, ibA.Id, f.onA, ibB.Id, f.onB)
		}
	}
	record := lookupClientRecord(t, "demo@x")
	if record.PublicKey != peerA.PublicKey {
		t.Errorf("client record public key %q left the deployed peers behind (inbound %d holds %q)", record.PublicKey, ibA.Id, peerA.PublicKey)
	}

	bulkCreateOne(t, svc, inboundSvc, model.Client{Email: "stranger@x", SubID: "sub-stranger", Enable: true}, ibA.Id)
	stranger := awgPeer(t, inboundSvc, ibA.Id, "stranger@x")
	if stranger.PrivateKey == "" || stranger.PublicKey == "" || stranger.PreSharedKey == "" {
		t.Errorf("previously unknown identity got no key set of its own: %+v", stranger)
	}
	if stranger.PrivateKey == peerA.PrivateKey || stranger.PublicKey == peerA.PublicKey || stranger.PreSharedKey == peerA.PreSharedKey {
		t.Errorf("previously unknown identity was handed the known identity's keys: %+v", stranger)
	}

	// A non-empty incoming value wins over the stored one, like the four fields
	// next to it; inbounds A and B are left holding the superseded key.
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	psk, err := wgutil.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("psk: %v", err)
	}
	bulkCreateOne(t, svc, inboundSvc, model.Client{
		Email: "demo@x", SubID: "sub-demo", Enable: true,
		PrivateKey: priv, PublicKey: pub, PreSharedKey: psk,
	}, ibC.Id)
	peerC := awgPeer(t, inboundSvc, ibC.Id, "demo@x")
	if peerC.PrivateKey != priv || peerC.PublicKey != pub || peerC.PreSharedKey != psk {
		t.Errorf("the supplied key set was overwritten by the stored one: sent %q/%q/%q, inbound %d holds %q/%q/%q",
			priv, pub, psk, ibC.Id, peerC.PrivateKey, peerC.PublicKey, peerC.PreSharedKey)
	}
}

// The stored public key is load-bearing only for an identity registered with a
// public key and no private one: a peer the operator holds the private half of.
func TestBulkCreateClient_ReusesStoredPublicKeyWithoutPrivateKey(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ibA := mkInbound(t, 21611, model.AWG, `{"address":"10.83.0.1/24","clients":[]}`)
	ibB := mkInbound(t, 21612, model.AWG, `{"address":"10.84.0.1/24","clients":[]}`)

	_, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	bulkCreateOne(t, svc, inboundSvc, model.Client{Email: "pubonly@x", SubID: "sub-pubonly", Enable: true, PublicKey: pub}, ibA.Id)
	peerA := awgPeer(t, inboundSvc, ibA.Id, "pubonly@x")
	if peerA.PublicKey != pub || peerA.PrivateKey != "" {
		t.Fatalf("first bulk add did not register a public-key-only peer: %+v", peerA)
	}

	bulkCreateOne(t, svc, inboundSvc, model.Client{Email: "pubonly@x", SubID: "sub-pubonly", Enable: true}, ibB.Id)
	peerB := awgPeer(t, inboundSvc, ibB.Id, "pubonly@x")
	if peerB.PublicKey != pub {
		t.Errorf("public key rotated on bulk re-add: inbound %d holds %q, inbound %d holds %q — the operator's private half now matches only one of the two",
			ibA.Id, pub, ibB.Id, peerB.PublicKey)
	}
}

// The combination this fix first makes reachable: bulk-adding a KNOWN identity
// to a keyless inbound, whose copy must be stripped after the keys are filled in.
func TestBulkCreateClient_KnownIdentityKeylessInboundNeverSeesTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	homeIb := mkInbound(t, 21621, model.AWG, `{"address":"10.85.0.1/24","clients":[]}`)
	vlessIb := mkInbound(t, 21622, model.VLESS, `{"clients":[]}`)
	awgIb := mkInbound(t, 21623, model.AWG, `{"address":"10.86.0.1/24","clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "bulk-known@x", SubID: "sub-bulk-known", Enable: true},
		InboundIds: []int{homeIb.Id},
	}); err != nil {
		t.Fatalf("AWG Create: %v", err)
	}
	home := awgPeer(t, inboundSvc, homeIb.Id, "bulk-known@x")
	if home.PrivateKey == "" || home.PreSharedKey == "" {
		t.Fatalf("AWG create did not mint a keypair and PSK: %+v", home)
	}

	// Keyless target first, tunnel target second — see the Attach sibling below.
	bulkCreateOne(t, svc, inboundSvc, model.Client{Email: "bulk-known@x", SubID: "sub-bulk-known", Enable: true}, vlessIb.Id, awgIb.Id)

	vless, err := inboundSvc.GetInbound(vlessIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(vless): %v", err)
	}
	for _, leak := range []string{"privateKey", "publicKey", "preSharedKey"} {
		if strings.Contains(vless.Settings, leak) {
			t.Errorf("bulk re-add of a known identity leaked its tunnel key %q into VLESS settings:\n%s", leak, vless.Settings)
		}
	}

	peer := awgPeer(t, inboundSvc, awgIb.Id, "bulk-known@x")
	if peer.PrivateKey != home.PrivateKey || peer.PreSharedKey != home.PreSharedKey {
		t.Errorf("the second AWG inbound got a different key set than inbound %d: %q/%q vs %q/%q",
			homeIb.Id, home.PrivateKey, home.PreSharedKey, peer.PrivateKey, peer.PreSharedKey)
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

// Attach is the third of the four multi-inbound paths: an identity registered
// on a keyless inbound must not collect a keypair per tunnel inbound it joins.
func TestAttach_TwoTunnelInboundsShareOneKeypair(t *testing.T) {
	cases := []struct {
		name                 string
		proto                model.Protocol
		settingsA, settingsB string
	}{
		{"two AWG inbounds", model.AWG, `{"address":"10.96.0.1/24","clients":[]}`, `{"address":"10.97.0.1/24","clients":[]}`},
		{"two WireGuard inbounds", model.WireGuard, wgServerSettings(), wgServerSettings()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			vlessIb := mkInbound(t, 21801, model.VLESS, `{"clients":[]}`)
			ibA := mkInbound(t, 21802, tc.proto, tc.settingsA)
			ibB := mkInbound(t, 21803, tc.proto, tc.settingsB)

			if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
				Client:     model.Client{Email: "roamer@x", SubID: "sub-roamer", Enable: true},
				InboundIds: []int{vlessIb.Id},
			}); err != nil {
				t.Fatalf("keyless Create: %v", err)
			}
			identity := lookupClientRecord(t, "roamer@x")
			if identity.PrivateKey != "" || identity.PublicKey != "" {
				t.Fatalf("the keyless create already stored tunnel keys: %+v", identity)
			}

			if _, err := svc.Attach(inboundSvc, identity.Id, []int{ibA.Id, ibB.Id}); err != nil {
				t.Fatalf("Attach to two tunnel inbounds: %v", err)
			}

			peerA := awgPeer(t, inboundSvc, ibA.Id, "roamer@x")
			peerB := awgPeer(t, inboundSvc, ibB.Id, "roamer@x")
			record := lookupClientRecord(t, "roamer@x")
			if peerA.PublicKey == "" || peerB.PublicKey == "" {
				t.Fatalf("one of the two tunnel inbounds got no public key at all: %+v / %+v", peerA, peerB)
			}
			if peerA.PublicKey != peerB.PublicKey || record.PublicKey != peerA.PublicKey {
				t.Errorf("one Attach minted a keypair per inbound: inbound %d holds public key %q, inbound %d holds %q, the client record holds %q — the subscription carries the record's key, so the other inbound's handshake silently never completes",
					ibA.Id, peerA.PublicKey, ibB.Id, peerB.PublicKey, record.PublicKey)
			}
			if peerA.PrivateKey != peerB.PrivateKey {
				t.Errorf("private key differs between inbound %d (%q) and inbound %d (%q)", ibA.Id, peerA.PrivateKey, ibB.Id, peerB.PrivateKey)
			}
		})
	}
}

// BulkAttach is the fourth path, the one the multi-select attach modal calls:
// same rule, same silent failure when each inbound mints its own pair.
func TestBulkAttach_TwoTunnelInboundsShareOneKeypair(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	vlessIb := mkInbound(t, 21811, model.VLESS, `{"clients":[]}`)
	ibA := mkInbound(t, 21812, model.AWG, `{"address":"10.98.0.1/24","clients":[]}`)
	ibB := mkInbound(t, 21813, model.AWG, `{"address":"10.99.0.1/24","clients":[]}`)

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "bulk-roamer@x", SubID: "sub-bulk-roamer", Enable: true},
		InboundIds: []int{vlessIb.Id},
	}); err != nil {
		t.Fatalf("keyless Create: %v", err)
	}
	if identity := lookupClientRecord(t, "bulk-roamer@x"); identity.PrivateKey != "" || identity.PublicKey != "" {
		t.Fatalf("the keyless create already stored tunnel keys: %+v", identity)
	}

	res, _, err := svc.BulkAttach(inboundSvc, []string{"bulk-roamer@x"}, []int{ibA.Id, ibB.Id})
	if err != nil {
		t.Fatalf("BulkAttach: %v", err)
	}
	if len(res.Errors) != 0 || len(res.Attached) != 2 {
		t.Fatalf("BulkAttach attached = %v, errors = %v, want both targets and no errors", res.Attached, res.Errors)
	}

	peerA := awgPeer(t, inboundSvc, ibA.Id, "bulk-roamer@x")
	peerB := awgPeer(t, inboundSvc, ibB.Id, "bulk-roamer@x")
	record := lookupClientRecord(t, "bulk-roamer@x")
	if peerA.PublicKey == "" || peerB.PublicKey == "" {
		t.Fatalf("one of the two tunnel inbounds got no public key at all: %+v / %+v", peerA, peerB)
	}
	if peerA.PublicKey != peerB.PublicKey || record.PublicKey != peerA.PublicKey {
		t.Errorf("one BulkAttach minted a keypair per inbound: inbound %d holds public key %q, inbound %d holds %q, the client record holds %q — the subscription carries the record's key, so the other inbound's handshake silently never completes",
			ibA.Id, peerA.PublicKey, ibB.Id, peerB.PublicKey, record.PublicKey)
	}
	if peerA.PrivateKey != peerB.PrivateKey {
		t.Errorf("private key differs between inbound %d (%q) and inbound %d (%q)", ibA.Id, peerA.PrivateKey, ibB.Id, peerB.PrivateKey)
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
