// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRenderClientConf_TableOff(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320,"keepalive":25}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if !strings.Contains(conf, "Table = off") {
		t.Error("Table = off missing — critical, would override system default route")
	}
}

func TestRenderClientConf_NoListenPort(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if strings.Contains(conf, "ListenPort") {
		t.Error("ListenPort must NOT appear in client conf")
	}
}

func TestRenderClientConf_MandatoryFields(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up.example.com:51820","keepalive":25}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, want := range []string{
		"PrivateKey = k",
		"Address = 10.9.0.5/32",
		"MTU = 1320",
		"PublicKey = pub",
		"Endpoint = up.example.com:51820",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("missing %q in conf:\n%s", want, conf)
		}
	}
}

func TestRenderClientConf_DNS_OmittedByDefault(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if strings.Contains(conf, "DNS =") {
		t.Error("DNS must NOT be written (Xray resolves via UseIP, resolvconf crashes without systemd-resolved)")
	}
}

// TestRenderClientConf_DNS_NeverWritten is the regression guard for the
// resolvconf crash: even when Settings.DNS is populated (e.g. via Paste .conf
// from an upstream that included DNS), renderClientConf MUST NOT write it to
// the .conf. awg-quick up invokes `resolvconf -a awgo-N` on DNS= lines, which
// fails on hosts without a working resolvconf backend (systemd-resolved
// disabled, openresolv absent) with "Unit dbus-org.freedesktop.resolve1.service
// not found" — awg-quick rolls back the interface, reconcile fails every 10s.
// Caught live by tester VladufQa (awgo-3 down every 10s until systemd-resolved
// was enabled manually). With Table = off DNS would not carry the system
// default route anyway, so omitting it is correct, not just a workaround.
func TestRenderClientConf_DNS_NeverWritten(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","dns":"1.1.1.1, 1.0.0.1"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if strings.Contains(conf, "DNS =") {
		t.Errorf("DNS must NEVER be written to client .conf (resolvconf crash), got:\n%s", conf)
	}
}

func TestRenderClientConf_ObfuscationWhenSet(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"s1":20,"s2":30,"s3":40,"s4":50,"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, want := range []string{"Jc = 3", "Jmin = 50", "S1 = 20", "H1 = 100-500"} {
		if !strings.Contains(conf, want) {
			t.Errorf("missing %q in:\n%s", want, conf)
		}
	}
}

func TestRenderClientConf_ObfuscationOmittedWhenZero(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, bad := range []string{"Jc =", "Jmin =", "S1 =", "H1 ="} {
		if strings.Contains(conf, bad) {
			t.Errorf("obfuscation line %q should not appear when unset, in:\n%s", bad, conf)
		}
	}
}

func TestRenderClientConf_NoI1toI5(t *testing.T) {
	// I1-I5 crash `awg setconf` (kernel module rejects CPS tags in setconf
	// input, same as server-side — caught live by a tester on awgo-2: every
	// reconcile failed with exit status 1). Even when set in Settings, they
	// must NEVER be written to the .conf. Regression guard.
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","i1":"aa","i2":"bb","i3":"cc","i4":"dd","i5":"ee"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, bad := range []string{"I1 = aa", "I2 = bb", "I3 = cc", "I4 = dd", "I5 = ee"} {
		if strings.Contains(conf, bad) {
			t.Errorf("CPS tag %q must NOT appear in client .conf (crashes awg setconf), got:\n%s", bad, conf)
		}
	}
}

func TestRenderClientConf_IPv6(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"fd00::5/128","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if !strings.Contains(conf, "Address = fd00::5/128") {
		t.Errorf("IPv6 address not written, got:\n%s", conf)
	}
}

// HeaderProtectionKey must never reach the client .conf either — awgo-* is
// brought up by the same `awg setconf`, which rejects the unknown field and
// makes awg-quick roll the interface back. See renderServerConf's test for the
// full reasoning.
func TestRenderClientConf_NeverWritesHeaderProtectionKey(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings string
	}{
		{"empty", `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150}`},
		{"set", `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"headerProtectionKey":"aBcD...base64hpk=="}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &model.AwgOutbound{Id: 1, Settings: tc.settings}
			ci, _ := ClientInstanceFromOutbound(o)
			conf := renderClientConf(ci)
			if strings.Contains(conf, "HeaderProtectionKey") {
				t.Errorf("HeaderProtectionKey must never appear in client .conf, got:\n%s", conf)
			}
		})
	}
}
