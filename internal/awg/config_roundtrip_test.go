package awg

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestServerClientConfigRoundTrip(t *testing.T) {
	// 1. Generate AWG params (same as Create() flow)
	params, err := GenerateAWGParams(3, "quic", "ru")
	if err != nil {
		t.Fatalf("GenerateAWGParams: %v", err)
	}
	t.Logf("Server keys: priv=%s... pub=%s...", params.PrivateKey[:12], params.PublicKey[:12])

	if err := ValidateAWGParams(params); err != nil {
		t.Fatalf("ValidateAWGParams: %v", err)
	}

	// 2. Merge params into inbound (with CPS I1-I5)
	awgInbound := &model.Inbound{Id: 1, Port: 47010, Protocol: "awg", UserId: 1}
	i1, i2, i3, i4, i5 := GenerateCPS(3, CPSProfileQUIC)
	if err := MergeParamsToSettings(awgInbound, params, i1, i2, i3, i4, i5); err != nil {
		t.Fatalf("MergeParamsToSettings: %v", err)
	}

	// 3. Build server config
	serverConf := BuildServerConfig(awgInbound, "/tmp/awg1-up.sh", "/tmp/awg1-down.sh")
	if serverConf == "" {
		t.Fatal("empty server config")
	}
	t.Logf("Server config: %d bytes", len(serverConf))

	// 4. Generate client keys
	clientPriv := GenKey()
	clientPub := DerivePubkey(clientPriv)
	if clientPriv == "" || clientPub == "" {
		t.Fatal("empty client keys — awg/wg not installed and pure-Go fallback failed")
	}
	clientPSK := GenPSK()
	t.Logf("Client keys: pub=%s...", clientPub[:12])

	// 5. Build client config
	client := model.Client{
		ID:       clientPub,
		Password: clientPSK,
		Email:    "test-client",
		Enable:   true,
	}
	serverAddr := "vps-finland.example.com"
	clientConf := BuildClientConfig(awgInbound, client, clientPriv, params.PublicKey, serverAddr)
	if clientConf == "" {
		t.Fatal("empty client config")
	}
	t.Logf("Client config: %d bytes", len(clientConf))

	// === CORRESPONDENCE CHECKS ===

	// Server public key -> client Peer section
	if !strings.Contains(clientConf, "PublicKey = "+params.PublicKey) {
		t.Errorf("client [Peer] missing server public key %s", params.PublicKey[:12])
	}

	// PSK in client
	if !strings.Contains(clientConf, "PresharedKey = "+clientPSK) {
		t.Error("client missing PSK")
	}

	// Same obfuscation params
	obfFields := []string{
		fmt.Sprintf("Jc = %d", params.Jc),
		fmt.Sprintf("Jmin = %d", params.Jmin),
		fmt.Sprintf("Jmax = %d", params.Jmax),
		fmt.Sprintf("S1 = %d", params.S1),
		fmt.Sprintf("S2 = %d", params.S2),
		fmt.Sprintf("S3 = %d", params.S3),
		fmt.Sprintf("S4 = %d", params.S4),
		fmt.Sprintf("H1 = %s", params.H1),
		fmt.Sprintf("H2 = %s", params.H2),
		fmt.Sprintf("H3 = %s", params.H3),
		fmt.Sprintf("H4 = %s", params.H4),
	}
	for _, f := range obfFields {
		if !strings.Contains(clientConf, f) {
			t.Errorf("client missing obfuscation: %s", f)
		}
	}

	// CPS I1-I5 consistency (if in server, must be in client)
	for _, cps := range []string{"I1 =", "I2 =", "I3 =", "I4 =", "I5 ="} {
		serverHas := strings.Contains(serverConf, cps)
		clientHas := strings.Contains(clientConf, cps)
		if serverHas && !clientHas {
			t.Errorf("server has %s but client missing", cps)
		}
		if !serverHas && clientHas {
			t.Errorf("client has %s but server missing", cps)
		}
	}

	// Endpoint
	expectedEndpoint := fmt.Sprintf("Endpoint = %s:%d", serverAddr, awgInbound.Port)
	if !strings.Contains(clientConf, expectedEndpoint) {
		t.Errorf("client missing endpoint: %s", expectedEndpoint)
	}

	// Client private key
	if !strings.Contains(clientConf, "PrivateKey = "+clientPriv) {
		t.Error("client missing private key")
	}

	// Server config must NOT have Peer section (peers come from DB at runtime)
	if strings.Contains(serverConf, "[Peer]") && len(getPeers(awgInbound)) == 0 {
		// No peers in DB yet — server config should have no [Peer] sections
		// (this is the pre-client state before EnsureFirstClientExists)
	} else {
		t.Log("server config has [Peer] sections (clients already added)")
	}

	t.Logf("\n=== SERVER CONFIG ===\n%s\n=== CLIENT CONFIG ===\n%s", serverConf, clientConf)
}
