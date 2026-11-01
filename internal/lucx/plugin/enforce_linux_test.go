// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestPrepareCmdBuildsEnforcement(t *testing.T) {
	cmd := exec.Command("/bin/olcrtc-linux-amd64")
	cmd.Args = []string{"/bin/olcrtc-linux-amd64"}
	m := Manifest{Permissions: map[string][]string{"network": {networkNone}}, Resources: &Resources{MaxMemoryMB: 64}}
	if err := prepareCmd(cmd, m); err != nil {
		t.Fatalf("enforcement preparation failed: %v", err)
	}
	if got := cmd.Args[0]; got != "/bin/sh" {
		t.Fatalf("memory wrapper not prepended, args: %v", cmd.Args)
	}
	attr := cmd.SysProcAttr
	if attr.Cloneflags&syscall.CLONE_NEWNET == 0 {
		t.Fatal("network isolation flag missing")
	}
	plain := exec.Command("/bin/olcrtc-linux-amd64")
	if err := prepareCmd(plain, Manifest{}); err != nil || plain.SysProcAttr != nil || len(plain.Args) != 1 {
		t.Fatal("an unisolated manifest must not restrict the child")
	}
}

// TestStartStopsPlugin verifies the full start/status/stop flow for a real
// script plugin with a passing health probe.
func TestStartStopsPlugin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	td := t.TempDir()
	orig := binFolder
	binFolder = func() string { return td }
	defer func() { binFolder = orig }()

	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	manifest, _ := zw.Create("manifest.json")
	_ = json.NewEncoder(manifest).Encode(Manifest{ID: "demo-run", Name: "Demo run", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "run.sh", Permissions: map[string][]string{"network": {}, "panelApi": {}, "bot": {}}, Health: &HealthCheck{Type: "http", Target: srv.URL + "/healthz", IntervalSeconds: 5, TimeoutSeconds: 2}})
	entry, _ := zw.Create("run.sh")
	_, _ = entry.Write([]byte("#!/bin/sh\nwhile :; do sleep 1; done\n"))
	_ = zw.Close()

	if _, err := InstallPackage(bytes.NewReader(b.Bytes()), int64(b.Len())); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if err := Start("demo-run"); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer func() { _ = Stop("demo-run") }()
	s, err := Get("demo-run")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !s.Running || s.PID == 0 {
		t.Fatalf("plugin is not running: %+v", s)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s, err = Get("demo-run")
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if s.Health != nil && s.Health.Status == "pass" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("health did not pass: %+v", s.Health)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !s.Enforcement.Enforceable {
		t.Fatal("enforcement support not reported on linux")
	}
	if err := Stop("demo-run"); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if _, err := os.Stat(s.LogPath); err != nil {
		t.Fatalf("plugin log missing: %v", err)
	}
}

// TestStartAppliesNetNs verifies real network-namespace isolation on a host
// that permits unsharing (root or a privileged user namespace).
func TestStartAppliesNetNs(t *testing.T) {
	if syscall.Getuid() != 0 {
		t.Skip("network namespace isolation needs root; skipping")
	}
	td := t.TempDir()
	orig := binFolder
	binFolder = func() string { return td }
	defer func() { binFolder = orig }()

	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	manifest, _ := zw.Create("manifest.json")
	_ = json.NewEncoder(manifest).Encode(Manifest{ID: "demo-netns", Name: "Demo netns", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "run.sh", Permissions: map[string][]string{"network": {networkNone}, "panelApi": {}, "bot": {}}, Resources: &Resources{MaxMemoryMB: 64}})
	entry, _ := zw.Create("run.sh")
	_, _ = entry.Write([]byte("#!/bin/sh\nwhile :; do sleep 1; done\n"))
	_ = zw.Close()
	if _, err := InstallPackage(bytes.NewReader(b.Bytes()), int64(b.Len())); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if err := Start("demo-netns"); err != nil {
		t.Fatalf("isolated start failed: %v", err)
	}
	defer func() { _ = Stop("demo-netns") }()
	s, err := Get("demo-netns")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Running {
		t.Fatalf("isolated plugin not running: %+v", s)
	}
	if !s.Enforcement.NetworkIsolated || s.Enforcement.MemoryLimitMB != 64 {
		t.Fatalf("enforcement not reported: %+v", s.Enforcement)
	}
}
