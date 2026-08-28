package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// clearForeignTunnelKeys strips keypair and PSK together, so an incoming record
// with no tunnel keys is a keyless copy whose blank PSK means "not applicable".
func TestApplyClientRecordMerge_OnlyATunnelRecordClearsPreSharedKey(t *testing.T) {
	const storedPSK = "c3RvcmVkLXBzay0wMDAwMDAwMDAwMDAwMDAwMDAwMD0="

	cases := []struct {
		name     string
		incoming model.ClientRecord
		want     string
	}{
		{
			name:     "keypair present and psk blank clears the stored psk",
			incoming: model.ClientRecord{PrivateKey: "client-priv", PublicKey: "client-pub"},
			want:     "",
		},
		{
			name:     "public key only and psk blank clears the stored psk",
			incoming: model.ClientRecord{PublicKey: "client-pub"},
			want:     "",
		},
		{
			name:     "no tunnel keys and psk blank keeps the stored psk",
			incoming: model.ClientRecord{},
			want:     storedPSK,
		},
		{
			name:     "keypair present and psk supplied overwrites the stored psk",
			incoming: model.ClientRecord{PrivateKey: "client-priv", PublicKey: "client-pub", PreSharedKey: "rotated-psk"},
			want:     "rotated-psk",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := &model.ClientRecord{
				Email:        "psk-clear@x",
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
