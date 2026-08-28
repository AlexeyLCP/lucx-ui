// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// awg3Modules reports the host as carrying an AWG 3.1 module for one test.
func awg3Modules(t *testing.T) {
	t.Helper()
	yes := true
	SetModuleSupportsAwg3(&yes)
	SetModuleSupportsAwg31(&yes)
	t.Cleanup(func() {
		SetModuleSupportsAwg3(nil)
		SetModuleSupportsAwg31(nil)
	})
}

// Jc only ever proxied for "obfuscation is on"; the AWG3 field families carry
// their own awg3ok/awg31ok gates, exactly as renderServerConf writes them.
func TestRenderClientConf_Awg3FieldsSurviveJcZero(t *testing.T) {
	awg3Modules(t)
	const base = `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":0,`
	for _, tc := range []struct {
		name     string
		settings string
		want     []string
	}{
		{
			"header protection key",
			base + `"awgVersion":"3","headerProtectionKey":"hpk-base64"}`,
			[]string{"HeaderProtectionKey = hpk-base64"},
		},
		{
			"device timers and padding",
			base + `"awgVersion":"3","contentPaddingAddition":"32","rekeyAfterTime":"120",` +
				`"rekeyTimeout":"5","rejectAfterTime":"180","keepaliveTimeout":"10","maxHandshakeAttempts":"18"}`,
			[]string{
				"ContentPaddingAddition = 32", "RekeyAfterTime = 120", "RekeyTimeout = 5",
				"RejectAfterTime = 180", "KeepaliveTimeout = 10", "MaxHandshakeAttempts = 18",
			},
		},
		{
			"3.1 device flags",
			base + `"awgVersion":"3.1","randomTrailers":true,"disableCookies":true}`,
			[]string{"RandomTrailers = on", "DisableCookies = on"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ci, ok := ClientInstanceFromOutbound(&model.AwgOutbound{Id: 1, Settings: tc.settings})
			if !ok {
				t.Fatalf("ClientInstanceFromOutbound rejected %s", tc.settings)
			}
			conf := renderClientConf(ci)
			for _, w := range tc.want {
				if !strings.Contains(conf, w) {
					t.Errorf("missing %q at Jc = 0, got:\n%s", w, conf)
				}
			}
		})
	}
}

// The rendered text IS EnsureClient's restart fingerprint (client_manager.go:63),
// so a drift here bounces every live tunnel on upgrade — not just a red test.
func TestRenderClientConf_ObfuscatedRenderIsByteStable(t *testing.T) {
	awg3Modules(t)
	const settings = `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","psk":"psk-value",` +
		`"endpoint":"up:51820","keepalive":"25","allowedIPs":"10.0.0.0/8","mtu":1320,` +
		`"jc":4,"jmin":10,"jmax":50,"s1":20,"s2":30,"s3":15,"s4":12,` +
		`"h1":"1-100","h2":"101-200","h3":"201-300","h4":"301-400",` +
		`"i1":"aa","i2":"bb","i3":"cc","i4":"dd","i5":"ee",` +
		`"awgVersion":"3.1","headerProtectionKey":"hpk-base64",` +
		`"contentPaddingAddition":"32","rekeyAfterTime":"120","rekeyTimeout":"5",` +
		`"rejectAfterTime":"180","keepaliveTimeout":"10","maxHandshakeAttempts":"18",` +
		`"randomTrailers":true,"disableCookies":true}`
	const want = `[Interface]
PrivateKey = k
Address = 10.9.0.5/32
MTU = 1320
Table = off
Jc = 4
Jmin = 10
Jmax = 50
S1 = 20
S2 = 30
S3 = 15
S4 = 12
H1 = 1-100
H2 = 101-200
H3 = 201-300
H4 = 301-400
HeaderProtectionKey = hpk-base64
ContentPaddingAddition = 32
RekeyAfterTime = 120
RekeyTimeout = 5
RejectAfterTime = 180
KeepaliveTimeout = 10
MaxHandshakeAttempts = 18
RandomTrailers = on
DisableCookies = on
I1 = aa
I2 = bb
I3 = cc
I4 = dd
I5 = ee

[Peer]
PublicKey = pub
PresharedKey = psk-value
Endpoint = up:51820
AllowedIPs = 10.0.0.0/8
PersistentKeepalive = 25
`
	ci, ok := ClientInstanceFromOutbound(&model.AwgOutbound{Id: 1, Settings: settings})
	if !ok {
		t.Fatalf("ClientInstanceFromOutbound rejected %s", settings)
	}
	if got := renderClientConf(ci); got != want {
		t.Errorf("obfuscated render drifted.\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}
