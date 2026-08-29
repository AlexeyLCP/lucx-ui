// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
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

// The gate is per field, exactly like the client export (inbound.go:539): only
// a blank H is skipped, so header-magic-only and partial sets both survive.
func TestRenderServerConf_WritesEachNonEmptyH(t *testing.T) {
	for _, tc := range []struct {
		name   string
		jc, s1 int
		h      [4]string
		want   [4]string // "" means the H<i> line must be absent
	}{
		{"header magic only", 0, 0, [4]string{"5", "6", "7", "8"}, [4]string{"H1 = 5", "H2 = 6", "H3 = 7", "H4 = 8"}},
		{"partial set is not dropped", 0, 0, [4]string{"5", "", "", ""}, [4]string{"H1 = 5", "", "", ""}},
		{"blank member of an obfuscated set", 4, 20, [4]string{"1", " ", "3", "4"}, [4]string{"H1 = 1", "", "H3 = 3", "H4 = 4"}},
		{"all blank stays out", 4, 20, [4]string{}, [4]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := renderServerConf(Instance{
				Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
				Address: "10.8.0.1/24", Jc: tc.jc, Jmin: 10, Jmax: 50, S1: tc.s1, S2: 30,
				H1: tc.h[0], H2: tc.h[1], H3: tc.h[2], H4: tc.h[3],
			})
			for i, want := range tc.want {
				if want == "" {
					if prefix := fmt.Sprintf("H%d =", i+1); strings.Contains(conf, prefix) {
						t.Errorf("%s must be absent, got:\n%s", prefix, conf)
					}
					continue
				}
				if !strings.Contains(conf, want) {
					t.Errorf("missing %q, got:\n%s", want, conf)
				}
			}
		})
	}
}
