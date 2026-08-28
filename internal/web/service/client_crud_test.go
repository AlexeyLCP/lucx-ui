package service

import (
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
