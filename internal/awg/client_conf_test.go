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
		t.Error("DNS must NOT be written by default (Xray resolves via UseIP)")
	}
}

func TestRenderClientConf_DNS_WhenSet(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","dns":"1.1.1.1, 1.0.0.1"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if !strings.Contains(conf, "DNS = 1.1.1.1, 1.0.0.1") {
		t.Errorf("DNS should appear when set, got:\n%s", conf)
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
