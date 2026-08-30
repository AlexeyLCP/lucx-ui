// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestParseTCPEstablished(t *testing.T) {
	dump := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:20FB 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1 1 0000000000000000 100 0 0 10 0
   1: 0100007F:20FB 0100007F:C001 01 00000000:00000000 00:00000000 00000000     0        0 2 1 0000000000000000 100 0 0 10 0
   2: 0101A8C0:20FB 0201A8C0:C002 01 00000000:00000000 00:00000000 00000000     0        0 3 1 0000000000000000 100 0 0 10 0`
	if n := parseTCPEstablished(dump, 8443, true); n != 1 {
		t.Fatalf("established = %d, want 1 (skip listen + loopback)", n)
	}
	if n := parseTCPEstablished(dump, 8443, false); n != 2 {
		t.Fatalf("with loopback = %d, want 2", n)
	}
}

func TestParseIptablesSave(t *testing.T) {
	dump := `*filter
[10:100] -A INPUT -p tcp -m tcp --dport 8443 -m comment --comment lucx-anytls-anytls-1 -j MARK --set-xmark 0x0/0x0
[4:40] -A OUTPUT -p tcp -m tcp --sport 8443 -m comment --comment "lucx-anytls-anytls-1" -j MARK --set-xmark 0x0/0x0
`
	up, down, ok := parseIptablesSave(dump, "lucx-anytls-anytls-1")
	if !ok || up != 40 || down != 100 {
		t.Fatalf("up=%d down=%d ok=%v", up, down, ok)
	}
	legacy := `*filter
[10:100] -A INPUT -p tcp --dport 8443 -m comment --comment lucx-anytls-anytls-1 -j RETURN
[4:40] -A OUTPUT -p tcp --sport 8443 -m comment --comment lucx-anytls-anytls-1 -j RETURN
`
	up, down, ok = parseIptablesSave(legacy, "lucx-anytls-anytls-1")
	if !ok || up != 40 || down != 100 {
		t.Fatalf("legacy RETURN up=%d down=%d ok=%v", up, down, ok)
	}
}

func TestAnytlsAcctComment(t *testing.T) {
	if got := anytlsAcctComment("anytls-17"); got != "lucx-anytls-anytls-17" {
		t.Fatalf("got %q", got)
	}
}

func TestLegacyAnytlsReturnArgs(t *testing.T) {
	dump := `*filter
[6520:399000] -A INPUT -p tcp -m tcp --dport 8555 -m comment --comment lucx-anytls-anytls-17 -j RETURN
[195:11660] -A INPUT -p tcp -m tcp --dport 8443 -m comment --comment lucx-anytls-anytls-17 -j RETURN
[10:100] -A INPUT -p tcp -m tcp --dport 8443 -m comment --comment lucx-anytls-anytls-1 -j LUCX_ANYTLS_ACCT
-A OUTPUT -p tcp -m tcp --sport 8555 -m comment --comment lucx-anytls-anytls-17 -j RETURN
`
	got := legacyAnytlsReturnArgs(dump)
	if len(got) != 4 {
		t.Fatalf("n=%d want 4", len(got))
	}
	if got[0][0] != "-D" || got[0][1] != "INPUT" || got[3][1] != "OUTPUT" {
		t.Fatalf("got %#v", got)
	}
}

func TestAnytlsValidate(t *testing.T) {
	if err := (AnytlsConfig{Port: 8443, Password: "s3cret", SNI: "vpn.example.com"}).Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := (AnytlsConfig{Port: 0, Password: "x", SNI: "vpn.example.com"}).Validate(); err == nil {
		t.Fatal("port 0 must be rejected")
	}
	if err := (AnytlsConfig{Port: 70000, Password: "x", SNI: "vpn.example.com"}).Validate(); err == nil {
		t.Fatal("port 70000 must be rejected")
	}
	if err := (AnytlsConfig{Port: 8443, SNI: "vpn.example.com"}).Validate(); err == nil {
		t.Fatal("empty password must be rejected")
	}
	if err := (AnytlsConfig{Port: 8443, Password: "x"}).Validate(); err == nil {
		t.Fatal("empty sni must be rejected")
	}
}

func TestAnytlsBuildArgsAndListen(t *testing.T) {
	cfg := AnytlsConfig{Port: 9443, Password: "pass word", SNI: "vpn.example.com"}
	args := cfg.BuildArgs("/etc/ssl/cert.pem", "/etc/ssl/key.pem", "/tmp/anytls-password")
	want := []string{"-l", "0.0.0.0:9443", "-password-file", "/tmp/anytls-password", "-cert", "/etc/ssl/cert.pem", "-key", "/etc/ssl/key.pem"}
	if len(args) != len(want) {
		t.Fatalf("args = %v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestAnytlsClientLink(t *testing.T) {
	cfg := AnytlsConfig{Port: 8443, Password: "hunter2", SNI: "vpn.example.com"}
	got := cfg.ClientLink("node.example", "home")
	if !strings.HasPrefix(got, "anytls://hunter2@node.example:8443/") {
		t.Fatalf("ClientLink = %q", got)
	}
	if !strings.Contains(got, "sni=vpn.example.com") {
		t.Fatalf("trusted cert needs sni=: %q", got)
	}
	if strings.Contains(got, "insecure=") {
		t.Fatalf("must not skip TLS verify: %q", got)
	}
	if !strings.Contains(got, "#home") {
		t.Fatalf("remark fragment missing: %q", got)
	}
	got = (AnytlsConfig{Port: 443, Password: "p@ss", SNI: "vpn.example.com"}).ClientLink("203.0.113.9", "")
	if !strings.Contains(got, "p%40ss") {
		t.Fatalf("password must be percent-encoded: %q", got)
	}
	if cfg.ClientLink("", "x") != "" {
		t.Fatal("empty host must yield empty link")
	}
	if (AnytlsConfig{Port: 8443, Password: "x"}).ClientLink("h", "") != "" {
		t.Fatal("empty sni must yield empty link")
	}
}

func TestAnytlsEnsurePassword(t *testing.T) {
	cfg := AnytlsConfig{Port: 8443, SNI: "vpn.example.com"}
	out, err := cfg.EnsurePassword()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Password) != 24 {
		t.Fatalf("password len = %d, want 24", len(out.Password))
	}
	again, err := out.EnsurePassword()
	if err != nil {
		t.Fatal(err)
	}
	if again.Password != out.Password {
		t.Fatal("EnsurePassword must not rotate an existing password")
	}
}

func TestAnytlsInstanceFromInbound(t *testing.T) {
	dir := t.TempDir()
	cert, key := writeTestCert(t, dir, time.Now().Add(24*time.Hour), "vpn.example.com")

	ib := &model.Inbound{
		Id:       12,
		Protocol: model.Anytls,
		Enable:   true,
		Remark:   "anytls-home",
		Settings: `{"port": 9443, "password": "shared", "sni": "vpn.example.com"}`,
	}
	inst, ok := AnytlsInstanceFromInbound(ib, cert, key)
	if !ok {
		t.Fatal("expected ok")
	}
	if inst.Key != "anytls-12" {
		t.Fatalf("Key = %q", inst.Key)
	}
	if inst.Core != Anytls || !inst.Enabled {
		t.Fatalf("inst = %+v", inst)
	}
	if inst.ProbePort != 9443 {
		t.Fatalf("ProbePort = %d", inst.ProbePort)
	}
	joined := strings.Join(inst.Args, " ")
	if !strings.Contains(joined, "-l 0.0.0.0:9443") {
		t.Fatalf("Args = %v", inst.Args)
	}
	hasFile := strings.Contains(joined, "-password-file ")
	hasP := strings.Contains(joined, "-p shared")
	if hasFile == hasP {
		t.Fatalf("want password-file (new overlay) or -p (old binary), got %v", inst.Args)
	}
	if !strings.Contains(joined, "-cert "+cert) || !strings.Contains(joined, "-key "+key) {
		t.Fatalf("Args missing cert/key: %v", inst.Args)
	}
	if inst.FingerprintExtra == "" {
		t.Fatal("FingerprintExtra must hash the cert so ACME renewal restarts")
	}

	disabled := *ib
	disabled.Enable = false
	inst, ok = AnytlsInstanceFromInbound(&disabled, cert, key)
	if !ok || inst.Enabled {
		t.Fatalf("disabled inbound must yield Enabled:false, got %+v", inst)
	}

	other := &model.Inbound{Id: 1, Protocol: model.VLESS}
	if _, ok := AnytlsInstanceFromInbound(other, cert, key); ok {
		t.Fatal("non-anytls inbound must not map")
	}

	passwordless := &model.Inbound{
		Id:       7,
		Protocol: model.Anytls,
		Enable:   true,
		Settings: `{"port": 8443, "sni": "vpn.example.com"}`,
	}
	inst, ok = AnytlsInstanceFromInbound(passwordless, cert, key)
	if !ok || inst.Enabled {
		t.Fatalf("passwordless inbound must stay down until save mints a password: %+v", inst)
	}

	nocert := &model.Inbound{
		Id:       8,
		Protocol: model.Anytls,
		Enable:   true,
		Settings: `{"port": 8443, "password": "x", "sni": "vpn.example.com"}`,
	}
	inst, ok = AnytlsInstanceFromInbound(nocert, "", "")
	if !ok || inst.Enabled {
		t.Fatalf("no cert must stay down: %+v", inst)
	}

	ported := &model.Inbound{
		Id:       3,
		Protocol: model.Anytls,
		Enable:   true,
		Port:     9444,
		Settings: `{"port": 8443, "password": "x", "sni": "vpn.example.com"}`,
	}
	inst, ok = AnytlsInstanceFromInbound(ported, cert, key)
	if !ok || inst.ProbePort != 9444 {
		t.Fatalf("inbound.Port must win over settings.port: %+v", inst)
	}
}
