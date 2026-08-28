package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// A tunnel inbound's settings JSON omits preSharedKey when the client was saved
// without one, so replaying that snapshot must not erase a PSK acquired later.
func TestSyncInbound_StaleSettingsSnapshotKeepsIdentityPreSharedKey(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	wgIb := mkInbound(t, 51951, model.WireGuard, wgServerSettings())
	awgServerPriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	awgIb := mkInbound(t, 51952, model.AWG,
		`{"privateKey":"`+awgServerPriv+`","address":"10.71.0.1/24","clients":[]}`)

	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: "stale-psk@wg", SubID: "sub-stale-psk", Enable: true,
			PrivateKey: priv, PublicKey: pub,
		},
		InboundIds: []int{wgIb.Id},
	}); err != nil {
		t.Fatalf("Create on wireguard: %v", err)
	}

	rec := lookupClientRecord(t, "stale-psk@wg")
	if _, err := svc.Attach(inboundSvc, rec.Id, []int{awgIb.Id}); err != nil {
		t.Fatalf("Attach to awg: %v", err)
	}
	minted := lookupClientRecord(t, "stale-psk@wg").PreSharedKey
	if minted == "" {
		t.Fatalf("the AWG attach minted no PSK, so this scenario no longer applies")
	}

	wgNow, err := inboundSvc.GetInbound(wgIb.Id)
	if err != nil {
		t.Fatalf("GetInbound(wireguard): %v", err)
	}
	wgClients, err := inboundSvc.GetClients(wgNow)
	if err != nil {
		t.Fatalf("GetClients(wireguard): %v", err)
	}
	if wgClients[0].PreSharedKey != "" || wgClients[0].PublicKey == "" {
		t.Fatalf("expected the wireguard snapshot to carry keys but no PSK, got psk=%q pub=%q",
			wgClients[0].PreSharedKey, wgClients[0].PublicKey)
	}

	// What UpdateInbound and three traffic cron jobs do on their own.
	if err := svc.SyncInbound(nil, wgIb.Id, wgClients); err != nil {
		t.Fatalf("SyncInbound(wireguard): %v", err)
	}

	if got := lookupClientRecord(t, "stale-psk@wg").PreSharedKey; got != minted {
		t.Fatalf("identity PSK after replaying the stale snapshot = %q, want %q — the resync erased it", got, minted)
	}
}

func TestApplyClientRecordMerge_BlankPreSharedKeyNeverClears(t *testing.T) {
	const storedPSK = "c3RvcmVkLXBzay0wMDAwMDAwMDAwMDAwMDAwMDAwMD0="

	cases := []struct {
		name     string
		incoming model.ClientRecord
		want     string
	}{
		{
			name:     "stale snapshot carrying the keypair but no psk",
			incoming: model.ClientRecord{PrivateKey: "client-priv", PublicKey: "client-pub"},
			want:     storedPSK,
		},
		{
			name:     "keyless inbound copy of the identity",
			incoming: model.ClientRecord{},
			want:     storedPSK,
		},
		{
			name:     "record supplying a psk rotates it",
			incoming: model.ClientRecord{PrivateKey: "client-priv", PublicKey: "client-pub", PreSharedKey: "rotated-psk"},
			want:     "rotated-psk",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := &model.ClientRecord{
				Email:        "psk-merge@x",
				PrivateKey:   "stored-priv",
				PublicKey:    "stored-pub",
				PreSharedKey: storedPSK,
			}

			applyClientRecordMerge(row, &tc.incoming)

			if row.PreSharedKey != tc.want {
				t.Fatalf("PreSharedKey = %q, want %q", row.PreSharedKey, tc.want)
			}
		})
	}
}
