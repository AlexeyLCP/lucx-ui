// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestCsqttClientURI(t *testing.T) {
	cfg := DefaultCsqttConfig()
	cfg.Password = "pass"
	cfg.SubHost = "1.2.3.4"
	cfg.VkHashes = "abcdefghijklmnop1,abcdefghijklmnop2"
	got := cfg.ClientURI()
	if !strings.HasPrefix(got, "csqtt://connect?") {
		t.Fatalf("URI = %q", got)
	}
	for _, want := range []string{"v=2", "host=1.2.3.4", "peer=46000", "password=pass", "hashes=abcdefghijklmnop1+abcdefghijklmnop2"} {
		if !strings.Contains(got, want) {
			t.Errorf("URI missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "%2B") {
		t.Fatalf("hash separator must be a raw +, got %q", got)
	}
	cfg.VkHashes = "abcdefghijklmnop1\nabcdefghijklmnop2"
	if !strings.Contains(cfg.ClientURI(), "hashes=abcdefghijklmnop1+abcdefghijklmnop2") {
		t.Fatalf("newline-separated hashes must split, got %q", cfg.ClientURI())
	}
	if strings.ContainsAny(got, "\r\n") || strings.Contains(got, "qwdtt://") {
		t.Fatalf("ClientURI must be a single csqtt:// line, got %q", got)
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
		"--device-id phone-1",
		"--web-user lucx",
		"--web-pass web",
		"--dns 77.88.8.8,77.88.8.1",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, args)
		}
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
	if !ok || !cfg.RouteThroughXray {
		t.Fatal("empty settings must route through Xray")
	}
	inst, ok := CsqttInstanceFromInbound(ib)
	if !ok || !inst.RouteThroughXray || inst.TunName != CsqttTunName(7) || inst.RouteTable != csqttRouteTable {
		t.Fatalf("instance tun bridge missing: %+v", inst)
	}
	if len(inst.RouteIfaces) != 1 || inst.RouteIfaces[0] != csqttIface {
		t.Fatalf("RouteIfaces = %v", inst.RouteIfaces)
	}
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
