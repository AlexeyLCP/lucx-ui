// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// clientInstanceWithH builds an obfuscated outbound (Jc > 0, so the H block is
// reached) carrying the given H1-H4 verbatim.
func clientInstanceWithH(t *testing.T, h [4]string) ClientInstance {
	t.Helper()
	settings := fmt.Sprintf(
		`{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820",`+
			`"jc":4,"jmin":10,"jmax":50,"s1":20,"s2":30,"h1":%q,"h2":%q,"h3":%q,"h4":%q}`,
		h[0], h[1], h[2], h[3])
	ci, ok := ClientInstanceFromOutbound(&model.AwgOutbound{Id: 1, Settings: settings})
	if !ok {
		t.Fatalf("ClientInstanceFromOutbound rejected %s", settings)
	}
	return ci
}

// Saving an outbound never validates headers, so a blank H reaches the render;
// "H1 = " alone makes awg setconf reject the whole file (as on the server side).
func TestRenderClientConf_WritesEachNonEmptyH(t *testing.T) {
	for _, tc := range []struct {
		name string
		h    [4]string
		want [4]string // "" means the H<i> line must be absent
	}{
		{
			"blank member of an obfuscated set",
			[4]string{"", "101-200", "201-300", "301-400"},
			[4]string{"", "H2 = 101-200", "H3 = 201-300", "H4 = 301-400"},
		},
		{
			"whitespace counts as blank",
			[4]string{" ", "101-200", "201-300", "301-400"},
			[4]string{"", "H2 = 101-200", "H3 = 201-300", "H4 = 301-400"},
		},
		{
			"all four survive in order",
			[4]string{"1-100", "101-200", "201-300", "301-400"},
			[4]string{"H1 = 1-100", "H2 = 101-200", "H3 = 201-300", "H4 = 301-400"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := renderClientConf(clientInstanceWithH(t, tc.h))
			prev := -1
			for i, want := range tc.want {
				if want == "" {
					if prefix := fmt.Sprintf("H%d =", i+1); strings.Contains(conf, prefix) {
						t.Errorf("%s must be absent, got:\n%s", prefix, conf)
					}
					continue
				}
				at := strings.Index(conf, want)
				if at < 0 {
					t.Errorf("missing %q, got:\n%s", want, conf)
					continue
				}
				if at < prev {
					t.Errorf("%q is out of order, got:\n%s", want, conf)
				}
				prev = at
			}
		})
	}
}
