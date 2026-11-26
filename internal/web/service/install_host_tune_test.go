// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"os"
	"strings"
	"testing"
)

// Issue #215: the installer must ship and invoke the host-tuning script
// through the same LUCX-HOOK channel as lucx-acme.sh, and the script itself
// must stay opt-out-able and never fatal. Static pins (the real effects of
// the script need a root Linux host — CI shellcheck lints it too).
func TestInstallScriptWiresHostTune(t *testing.T) {
	inst, err := os.ReadFile("../../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	got := string(inst)
	for _, want := range []string{
		"LUCX_HOST_TUNE:-1",
		"bin/lucx-host-tune.sh",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("install.sh lost the host-tune hook (%s)", want)
		}
	}
	// Opt-out wins over the hook.
	if idxTune := strings.Index(got, "LUCX_HOST_TUNE:-1"); idxTune < 0 {
		t.Fatal("opt-out gate missing")
	}

	script, err := os.ReadFile("../../../bin/lucx-host-tune.sh")
	if err != nil {
		t.Fatal(err)
	}
	sg := string(script)
	for _, want := range []string{
		`LUCX_HOST_TUNE:-1`,
		"99-lucx-host-tune.conf",
		"LimitNOFILE=65535",
		"exit 0",
	} {
		if !strings.Contains(sg, want) {
			t.Fatalf("lucx-host-tune.sh missing %q", want)
		}
	}
	// Never fatal: the script must not use set -e (a failing sysctl on a
	// container host must not abort the installer).
	if strings.Contains(sg, "set -e") {
		t.Fatal("host-tune script must not use set -e")
	}
}
