// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build !linux

package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"
)

// TestStartRefusesEnforcementWhereUnsupported pins the fail-closed contract:
// a manifest that asks for isolation or limits must not start silently
// unsandboxed on a platform without enforcement.
func TestStartRefusesEnforcementWhereUnsupported(t *testing.T) {
	td := t.TempDir()
	orig := binFolder
	binFolder = func() string { return td }
	defer func() { binFolder = orig }()

	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	manifest, _ := zw.Create("manifest.json")
	_ = json.NewEncoder(manifest).Encode(Manifest{ID: "demo-refuse", Name: "Demo refuse", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "run.sh", Permissions: map[string][]string{"network": {networkNone}, "panelApi": {}, "bot": {}}, Resources: &Resources{MaxMemoryMB: 64}})
	entry, _ := zw.Create("run.sh")
	_, _ = entry.Write([]byte("#!/bin/sh\nwhile :; do sleep 1; done\n"))
	_ = zw.Close()

	if _, err := InstallPackage(bytes.NewReader(b.Bytes()), int64(b.Len())); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	err := Start("demo-refuse")
	if err == nil {
		t.Fatal("expected a refusal on a platform without enforcement")
	}
	s, getErr := Get("demo-refuse")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if s.Running {
		t.Fatal("refused plugin must not be running")
	}
	if s.LastError == "" {
		t.Fatal("refusal must surface in the plugin status")
	}
}
