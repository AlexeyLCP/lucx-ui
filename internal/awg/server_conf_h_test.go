// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"
)

// An import (import_build.go) can leave H blank on an unobfuscated instance;
// "H1 = " makes awg setconf reject the whole file, so blank H must be omitted.
func TestRenderServerConf_UnobfuscatedOmitsBlankH(t *testing.T) {
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
		Address: "10.8.0.1/24",
	}
	conf := renderServerConf(inst)
	for _, want := range []string{"H1 =", "H2 =", "H3 =", "H4 ="} {
		if strings.Contains(conf, want) {
			t.Errorf("unobfuscated instance must omit blank H fields, got %q in:\n%s", want, conf)
		}
	}
}

// The blank-H fix must not touch this path, or a working config's bytes
// (and therefore its reconcile fingerprint) would change.
func TestRenderServerConf_ObfuscatedKeepsH(t *testing.T) {
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
		Address:    "10.8.0.1/24",
		AwgVersion: "3.1", HeaderProtectionKey: "hpk-key",
		Jc: 4, Jmin: 10, Jmax: 50, S1: 20, S2: 30, S3: 15, S4: 12,
		H1: "1-100", H2: "101-200", H3: "201-300", H4: "301-400",
	}
	conf := renderServerConf(inst)
	t.Logf("rendered conf:\n%s", conf)
	for _, want := range []string{"H1 = 1-100", "H2 = 101-200", "H3 = 201-300", "H4 = 301-400"} {
		if !strings.Contains(conf, want) {
			t.Errorf("obfuscated instance must keep %q, got:\n%s", want, conf)
		}
	}
}
