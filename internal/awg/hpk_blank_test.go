// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"errors"
	"strings"
	"testing"
)

// A key of blanks reads as "no key" when saving and as "a key" in both .conf
// renderers, so an I-set in the 3457-3492 band saves and then vanishes.
func TestBlankHeaderProtectionKey_SameVerdictAsTheSavePath(t *testing.T) {
	awg3Modules(t)
	iField := strings.Repeat("x", 3484) // IBytes 3492: fits without a key, not with one
	for _, tc := range []struct {
		name       string
		hpk        string
		wantBudget int
		wantLine   bool
	}{
		{"blank key", "   ", 3492, false},
		{"empty key", "", 3492, false},
		{"real key", "aBcD...base64hpk==", 3456, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := renderServerConf(Instance{
				Ifname: "awg9", PrivateKey: "k", Port: 51820, MTU: 1320,
				AwgVersion: "3", HeaderProtectionKey: tc.hpk, I1: iField,
			})
			client := renderClientConf(ClientInstance{
				Id: 1, Ifname: "awgo-1",
				Settings: ClientSettings{
					PrivateKey: "k", Address: "10.9.0.5/32", MTU: 1320,
					PublicKey: "pub", Endpoint: "up:51820",
					AwgVersion: "3", HeaderProtectionKey: tc.hpk, I1: iField,
				},
			})
			saved := !errors.Is(ValidateIFields(BaselineIfname, tc.hpk, iField, "", "", "", ""), ErrIFieldsTooLarge)
			wantFit := IBytes(iField, "", "", "", "") <= tc.wantBudget
			gotServer := strings.Contains(server, "I1 = "+iField)
			gotClient := strings.Contains(client, "I1 = "+iField)
			if saved != wantFit || gotServer != wantFit || gotClient != wantFit {
				t.Errorf("hpk %q, I-set %d bytes: save=%v server=%v client=%v — all three want %v (budget %d)",
					tc.hpk, IBytes(iField, "", "", "", ""), saved, gotServer, gotClient, wantFit, tc.wantBudget)
			}
			for name, conf := range map[string]string{"server": server, "client": client} {
				if got := strings.Contains(conf, "HeaderProtectionKey"); got != tc.wantLine {
					t.Errorf("%s .conf: HeaderProtectionKey line present = %v, want %v (key %q)",
						name, got, tc.wantLine, tc.hpk)
				}
			}
		})
	}
}

// The predicate trims, the emitted value must not. Trimming it too would shift
// the fingerprint of every config whose value was pasted with a stray space.
func TestRenderers_WhitespaceEdgesAreEmittedVerbatim(t *testing.T) {
	awg3Modules(t)
	const hpk = "  abc  "
	const h1 = "100-500 "
	confs := map[string]string{
		"server": renderServerConf(Instance{
			Ifname: "awg9", PrivateKey: "k", Port: 51820, MTU: 1320,
			AwgVersion: "3", HeaderProtectionKey: hpk, H1: h1,
		}),
		"client": renderClientConf(ClientInstance{
			Id: 1, Ifname: "awgo-1",
			Settings: ClientSettings{
				PrivateKey: "k", Address: "10.9.0.5/32", MTU: 1320,
				PublicKey: "pub", Endpoint: "up:51820",
				AwgVersion: "3", HeaderProtectionKey: hpk, H1: h1,
			},
		}),
	}
	for side, conf := range confs {
		for _, want := range []string{"HeaderProtectionKey = " + hpk + "\n", "H1 = " + h1 + "\n"} {
			if !strings.Contains(conf, want) {
				t.Errorf("%s .conf: missing %q — the emitted value must stay raw, got:\n%s", side, want, conf)
			}
		}
	}
}

// The device half of this text is the reconcile fingerprint (deviceFingerprint),
// so a byte of drift bounces every live inbound on upgrade.
func TestRenderServerConf_ObfuscatedRenderIsByteStable(t *testing.T) {
	awg3Modules(t)
	inst := Instance{
		Id: 4, Ifname: "awg4", Port: 21860, PrivateKey: "server-priv", MTU: 1320,
		AwgVersion: "3.1", HeaderProtectionKey: "hpk-base64",
		Jc: 4, Jmin: 10, Jmax: 50, S1: 20, S2: 30, S3: 15, S4: 12,
		H1: "1-100", H2: "101-200", H3: "201-300", H4: "301-400",
		I1: "aa", I2: "bb", I3: "cc", I4: "dd", I5: "ee",
		ContentPaddingAddition: "32", RekeyAfterTime: "120", RekeyTimeout: "5",
		RejectAfterTime: "180", KeepaliveTimeout: "10", MaxHandshakeAttempts: "18",
		RandomTrailers: true, DisableCookies: true,
	}
	const want = `# Managed by x-ui - do not edit
[Interface]
PrivateKey = server-priv
ListenPort = 21860
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
`
	if got := renderServerConf(inst); got != want {
		t.Errorf("obfuscated render drifted.\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}
