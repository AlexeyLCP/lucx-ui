// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// csqttDecryptPayloadBytes opens the sealed payload of a v2.5.0-style link.
func csqttDecryptPayloadBytes(t *testing.T, raw []byte) []byte {
	t.Helper()
	keyHash := sha256.Sum256([]byte(csqttCipherSeed))
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	out, err := gcm.Open(nil, raw[:12], raw[12:], nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return out
}

// csqttDecryptTest helper: reverses csqttEncryptLink (upstream v2.5.0 format).
func csqttDecryptLinkForTest(t *testing.T, link string) string {
	t.Helper()
	payload := strings.TrimPrefix(link, "csqtt://")
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) < 28 {
		t.Fatalf("too short: %d bytes", len(raw))
	}
	got := csqttDecryptPayloadBytes(t, raw)
	return string(got)
}

// The golden vector was produced by an independent node.js implementation of
// the upstream encrypt_csqtt_link spec (SHA256 seed key, HMAC-derived IV, AES-256-GCM).
func TestCsqttClientURI(t *testing.T) {
	cfg := DefaultCsqttConfig()
	cfg.Password = "pass"
	cfg.SubHost = "1.2.3.4"
	cfg.VkHashes = "abcdefghijklmnop1,abcdefghijklmnop2"
	got := cfg.ClientURI()
	wantLink := "csqtt://Er5brAAfeA7zKVa8qs39cEkpjq_eYWTbT75L5ZjeMrubgGMvtxToWsrWeTILHkXC8QoPDSIaUgTJpWsRqUk_kY6Hgiy0tmtD0MS_YhsQ1RyQ8X1gdjex6B-pPcVhQHbmCZH09IQm8TOfAKYczBrPh980FQFhjL_F"
	if got != wantLink {
		t.Fatalf("URI mismatch with upstream v2.5.0 vector:\n got %q\nwant %q", got, wantLink)
	}
	decrypted := csqttDecryptLinkForTest(t, got)
	wantPayload := "connect?v=2&host=1.2.3.4&peer=46000&password=pass&hashes=abcdefghijklmnop1+abcdefghijklmnop2"
	if decrypted != wantPayload {
		t.Fatalf("payload = %q, want %q", decrypted, wantPayload)
	}
	if again := cfg.ClientURI(); again != got {
		t.Fatalf("ClientURI must be deterministic (HMAC IV), got %q then %q", got, again)
	}
	if strings.Contains(got, "connect?connect") || strings.ContainsAny(got, "\r\n") ||
		strings.Contains(got, "qwdtt://") || strings.Contains(got, "%2B") {
		t.Fatalf("ClientURI must be a single encrypted csqtt:// line, got %q", got)
	}
	cfg.VkHashes = "abcdefghijklmnop1\nabcdefghijklmnop2"
	cfg.SubHost = "5.6.7.8"
	cfg.Password = "p2"
	if d := csqttDecryptLinkForTest(t, cfg.ClientURI()); !strings.Contains(d, "hashes=abcdefghijklmnop1+abcdefghijklmnop2") {
		t.Fatalf("newline-separated hashes must split, decrypted %q", d)
	}
	cfg.SubHost = ""
	if cfg.ClientURI() != "" {
		t.Fatal("empty host must yield empty URI")
	}
}

func TestCsqttBuildArgs(t *testing.T) {
	cfg := DefaultCsqttConfig()
	cfg.Password = "s3cret"
	cfg.DeviceID = "phone-1"
	cfg.WebPass = "web"
	args := cfg.BuildArgs()
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--listen 0.0.0.0:46000",
		"--web-port 46002",
		"--password s3cret",
		"--web-user lucx",
		"--web-pass web",
		"--dns 77.88.8.8,77.88.8.1",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, args)
		}
	}
	if strings.Contains(joined, "--device-id") {
		// upstream v2.5.0 CLI rejects unknown flags: the core would not start.
		t.Errorf("must not pass removed --device-id flag: %v", args)
	}
	if strings.Contains(joined, " start") || strings.Contains(joined, "--start") {
		t.Errorf("must not pass systemctl subcommand: %v", args)
	}
}

func TestCsqttValidate(t *testing.T) {
	if err := DefaultCsqttConfig().Validate(); err != nil {
		t.Fatalf("valid rejected: %v", err)
	}
	bad := DefaultCsqttConfig()
	bad.ListenAddr = "no-port"
	if err := bad.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCsqttRouteThroughXrayDefault(t *testing.T) {
	ib := &model.Inbound{Id: 7, Protocol: model.Csqtt, Enable: true, Port: 46000, Settings: `{"listenAddr":"0.0.0.0:46000"}`}
	cfg, ok := CsqttConfigFromInbound(ib)
	if !ok || cfg.RouteThroughXray {
		t.Fatal("missing key must stay routeThroughXray=false")
	}
	inst, ok := CsqttInstanceFromInbound(ib)
	if !ok || inst.RouteThroughXray || inst.TunName != "" {
		t.Fatalf("default must not install the Xray bridge: %+v", inst)
	}

	ib.Settings = `{"listenAddr":"0.0.0.0:46000","routeThroughXray":true}`
	inst, ok = CsqttInstanceFromInbound(ib)
	if !ok || !inst.RouteThroughXray || inst.TunName != CsqttTunName(7) || inst.RouteTable != csqttRouteTable {
		t.Fatalf("instance tun bridge missing: %+v", inst)
	}
	if len(inst.RouteIfaces) != 1 || inst.RouteIfaces[0] != csqttIface {
		t.Fatalf("RouteIfaces = %v", inst.RouteIfaces)
	}
}

func TestCsqttFirewallCoversBothLegs(t *testing.T) {
	xray := csqttXrayFirewall("tun7")
	if !csqttFirewallHas(xray, "FORWARD", "-i", csqttIface) || !csqttFirewallHas(xray, "FORWARD", "-i", "tun7") {
		t.Fatalf("xray bridge missing FORWARD accepts: %+v", xray)
	}
	if !csqttFirewallHas(xray, "FORWARD", "--clamp-mss-to-pmtu") {
		t.Fatal("xray bridge missing MSS clamp")
	}
	direct := csqttDirectFirewall()
	if !csqttFirewallHas(direct, "POSTROUTING", "-j", "MASQUERADE") {
		t.Fatal("direct path missing MASQUERADE")
	}
	if csqttFirewallHas(xray, "POSTROUTING", "MASQUERADE") {
		t.Fatal("xray bridge must not install MASQUERADE")
	}
}

func csqttFirewallHas(rules []csqttFirewallSpec, parts ...string) bool {
	for _, r := range rules {
		line := r.chain + " " + strings.Join(r.spec, " ")
		ok := true
		for _, p := range parts {
			if !strings.Contains(line, p) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestCsqttPasswordStampWipesDb(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "csqtt.db")
	if err := os.WriteFile(db, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	syncCsqttPasswordStamp(dir, "first")
	if _, err := os.Stat(db); err != nil {
		t.Fatalf("first stamp must keep existing db: %v", err)
	}
	syncCsqttPasswordStamp(dir, "first")
	if _, err := os.Stat(db); err != nil {
		t.Fatalf("same password must keep db: %v", err)
	}
	syncCsqttPasswordStamp(dir, "second")
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("password change must drop csqtt.db, stat=%v", err)
	}
}

func TestCsqttRemoveWipesDataDir(t *testing.T) {
	prev := tunnelDir
	dir := t.TempDir()
	tunnelDir = func() string { return dir }
	t.Cleanup(func() { tunnelDir = prev })

	data := dataDirFor(CsqttKey, Csqtt)
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "csqtt.db"), []byte("bound"), 0o600); err != nil {
		t.Fatal(err)
	}
	removeManagedFiles(CsqttKey)
	if _, err := os.Stat(data); !os.IsNotExist(err) {
		t.Fatalf("delete inbound must wipe csqtt-data, stat=%v", err)
	}
}

func TestCsqttNameRegistry(t *testing.T) {
	if !Csqtt.Valid() || Csqtt.DisplayName() != "CSQTT" {
		t.Fatalf("Csqtt registry broken")
	}
	if got := Csqtt.BinaryName(); !strings.HasPrefix(got, "csqtt-") {
		t.Fatalf("BinaryName = %q", got)
	}
}
