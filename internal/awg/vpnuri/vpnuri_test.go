// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package vpnuri

import (
	"strings"
	"testing"
)

const sampleConf = `[Interface]
PrivateKey = CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=
Address = 10.200.0.2/32
DNS = 1.1.1.1
MTU = 1320
Jc = 4

[Peer]
PublicKey = DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 1.2.3.4:51820
`

func TestEncodeDecodeRoundTrip(t *testing.T) {
	uri, err := EncodeConf(sampleConf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uri, "vpn://") {
		t.Fatalf("prefix: %s", uri[:20])
	}
	// no padding in RawURLEncoding
	if strings.Contains(uri, "=") {
		t.Fatalf("unexpected padding in %s", uri)
	}
	got, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(sampleConf) {
		t.Fatalf("round-trip mismatch:\n got %q\nwant %q", got, sampleConf)
	}
}

func TestEncodeConf_RejectsEmpty(t *testing.T) {
	if _, err := EncodeConf(""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := EncodeConf("not a conf"); err == nil {
		t.Fatal("expected error for missing sections")
	}
}
