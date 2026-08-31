//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// installAwgQuickRecorder puts an `awg` that reports every interface as up and
// an `awg-quick` that only records the verb it was asked for, so a test can
// assert the tunnel was left alone. Returns the path of the record file.
func installAwgQuickRecorder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	show := "#!/bin/sh\nprintf 'iface-privkey\\tiface-pubkey\\t51820\\toff\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "awg"), []byte(show), 0o755); err != nil {
		t.Fatalf("write awg stub: %v", err)
	}
	quick := "#!/bin/sh\necho \"$1\" >> \"" + calls + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "awg-quick"), []byte(quick), 0o755); err != nil {
		t.Fatalf("write awg-quick stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

func resetClientState(t *testing.T) {
	t.Helper()
	clientMu.Lock()
	saved := clients
	clients, clientSwept = map[string]clientState{}, sync.Once{}
	clientMu.Unlock()
	t.Cleanup(func() {
		clientMu.Lock()
		clients = saved
		clientMu.Unlock()
	})
}

func adoptTestInstance(t *testing.T) ClientInstance {
	t.Helper()
	o := &model.AwgOutbound{
		Id:       8,
		Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`,
	}
	ci, ok := ClientInstanceFromOutbound(o)
	if !ok {
		t.Fatal("fixture must produce a client instance")
	}
	return ci
}

// The in-memory map is empty in a fresh process, so EnsureClient fell through
// to awg-quick down + up for every outbound already running — a traffic gap on
// every panel restart, which platform_linux.go states must not happen.
func TestEnsureClient_AdoptsALiveInterfaceAfterRestart(t *testing.T) {
	dir := withTempConfigDir(t)
	calls := installAwgQuickRecorder(t)
	resetClientState(t)

	ci := adoptTestInstance(t)
	confPath := filepath.Join(dir, ci.Ifname+".conf")
	if err := os.WriteFile(confPath, []byte(renderClientConf(ci)), 0o600); err != nil {
		t.Fatalf("seed conf: %v", err)
	}

	if err := GetManager().EnsureClient(ci); err != nil {
		t.Fatalf("EnsureClient: %v", err)
	}

	if b, err := os.ReadFile(calls); err == nil && len(b) > 0 {
		t.Fatalf("a live interface whose config already matches was bounced: awg-quick %s", b)
	}
}

// The adoption must not swallow a real change: a .conf that no longer matches
// the desired render is exactly the case awg-quick has to be run for.
func TestEnsureClient_StaleConfStillRestarts(t *testing.T) {
	dir := withTempConfigDir(t)
	calls := installAwgQuickRecorder(t)
	resetClientState(t)

	ci := adoptTestInstance(t)
	confPath := filepath.Join(dir, ci.Ifname+".conf")
	if err := os.WriteFile(confPath, []byte("[Interface]\nPrivateKey = an-older-key\n"), 0o600); err != nil {
		t.Fatalf("seed stale conf: %v", err)
	}

	if err := GetManager().EnsureClient(ci); err != nil {
		t.Fatalf("EnsureClient: %v", err)
	}

	b, err := os.ReadFile(calls)
	if err != nil || len(b) == 0 {
		t.Fatal("a changed config must reach awg-quick, but it was never called")
	}
}

// installAwgCallRecorder puts an `awg` on PATH that records every invocation
// and reports the interface as up. Returns the path of the record file.
func installAwgCallRecorder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	calls := filepath.Join(dir, "awg-calls")
	script := "#!/bin/sh\necho \"$@\" >> \"" + calls + "\"\nprintf 'iface-privkey\tiface-pubkey\t51820\toff\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "awg"), []byte(script), 0o755); err != nil {
		t.Fatalf("write awg stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

// This runs inside every Xray config build. A live probe there made the
// generated config depend on a 5s-deadline shell call, so a single flaky read
// flipped configUnchanged and restarted Xray for every user on the panel.
func TestClientIfaceUp_DoesNotProbeWhenTheStateIsKnown(t *testing.T) {
	withTempConfigDir(t)
	calls := installAwgCallRecorder(t)
	const ifname = "awgo-79"

	clientMu.Lock()
	saved := clientUp
	clientUp = map[string]bool{ifname: false}
	clientMu.Unlock()
	t.Cleanup(func() {
		clientMu.Lock()
		clientUp = saved
		clientMu.Unlock()
	})

	// The stub answers "up"; the recorded state says down. The recorded state
	// has to win, and the stub must not be reached at all.
	if GetManager().ClientIfaceUp(ifname) {
		t.Error("a recorded down interface was reported up, so the probe answered instead of the state")
	}
	if b, err := os.ReadFile(calls); err == nil && len(b) > 0 {
		t.Fatalf("config generation shelled out to awg: %s", b)
	}
}
