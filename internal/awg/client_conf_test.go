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
		"MTU = 1420",
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

func TestRenderClientConf_S3S4OmittedFor15(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":4,"jmin":10,"jmax":50,"s1":20,"s2":30,"s3":40,"s4":50,"h1":"1","h2":"2","h3":"3","h4":"4","awgVersion":"1.5"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if strings.Contains(conf, "S3 =") || strings.Contains(conf, "S4 =") {
		t.Errorf("S3/S4 must be omitted for awgVersion 1.5, got:\n%s", conf)
	}
	if !strings.Contains(conf, "S1 = 20") {
		t.Errorf("S1 must remain, got:\n%s", conf)
	}
}

// TestRenderClientConf_IFieldsOmittedFor15 mirrors
// TestRenderClientConf_S3S4OmittedFor15 for I1-I5: v1.5 tools reject the
// tags, so the awgo-N outbound .conf must not carry them either.
func TestRenderClientConf_IFieldsOmittedFor15(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","i1":"aa","i2":"bb","i3":"cc","i4":"dd","i5":"ee","awgVersion":"1.5"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, bad := range []string{"I1 =", "I2 =", "I3 =", "I4 =", "I5 ="} {
		if strings.Contains(conf, bad) {
			t.Errorf("I-fields must be omitted for awgVersion 1.5, got %q in:\n%s", bad, conf)
		}
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

// I1-I5 must ride in the .conf: applying them with `awg set` after awg-quick
// up landed 20.4 ms after the first handshake initiation had left the wire, so
// the very first handshake carried no CPS mimicry. setconf accepts the tags
// (tools v3.1.20260812); the historical "Invalid argument" came from malformed
// descriptors, not from I-fields as a class.
func TestRenderClientConf_I1toI5Written(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","i1":"aa","i2":"bb","i3":"cc","i4":"dd","i5":"ee"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, want := range []string{"I1 = aa", "I2 = bb", "I3 = cc", "I4 = dd", "I5 = ee"} {
		if !strings.Contains(conf, want) {
			t.Errorf("missing %q in client .conf — first handshake loses CPS mimicry:\n%s", want, conf)
		}
	}
}

func TestRenderClientConf_IFieldsOmittedWhenUnset(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","i1":"   "}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, bad := range []string{"I1 =", "I2 =", "I3 =", "I4 =", "I5 ="} {
		if strings.Contains(conf, bad) {
			t.Errorf("blank I-field %q must not be written, got:\n%s", bad, conf)
		}
	}
}

// Past the netlink budget the interface still comes up and passes traffic but
// `awg show` fails with EMSGSIZE, so the whole I-set is dropped instead.
func TestRenderClientConf_IFieldsBudget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		chars   int
		written bool
	}{
		{"exactly at budget", 3495, true},    // IBytes 3500
		{"one align step over", 3496, false}, // IBytes 3504
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := strings.Repeat("x", tc.chars)
			o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","i1":"` + v + `"}`}
			ci, _ := ClientInstanceFromOutbound(o)
			if got := strings.Contains(renderClientConf(ci), "I1 = "+v); got != tc.written {
				t.Fatalf("%d chars (IBytes %d, budget %d): written = %v, want %v",
					tc.chars, IBytes(v, "", "", "", ""), IBytesBudget(ci.Ifname, false), got, tc.written)
			}
		})
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

// HeaderProtectionKey is version-gated in the client .conf (awgo-N outbounds):
// written only when awgVersion == "3" AND the key is non-empty. Older builds of
// the kernel module reject the field, so v1/v2 outbounds must never carry it.
func TestRenderClientConf_HeaderProtectionKeyVersionGated(t *testing.T) {
	awg3 := true
	SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	for _, tc := range []struct {
		name     string
		settings string
		want     bool
	}{
		{"empty key v3", `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"awgVersion":"3"}`, false},
		{"set key v3", `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"awgVersion":"3","headerProtectionKey":"aBcD...base64hpk=="}`, true},
		{"set key v3.1", `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"awgVersion":"3.1","headerProtectionKey":"aBcD...base64hpk=="}`, true},
		{"set key v2", `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"awgVersion":"2","headerProtectionKey":"aBcD...base64hpk=="}`, false},
		{"set key no version", `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"headerProtectionKey":"aBcD...base64hpk=="}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &model.AwgOutbound{Id: 1, Settings: tc.settings}
			ci, _ := ClientInstanceFromOutbound(o)
			conf := renderClientConf(ci)
			contains := strings.Contains(conf, "HeaderProtectionKey = ")
			if contains != tc.want {
				t.Errorf("want HeaderProtectionKey in conf=%v, got=%v\nConf:\n%s", tc.want, contains, conf)
			}
		})
	}
}

// The six device-level AWG3 fields are version-gated in the client .conf too:
// emitted only when Jc > 0 (the whole obfuscation block is), AwgVersion == "3",
// and the field > 0. On a non-v3 outbound the lines must NOT appear even when
// the field carries a value. Mirrors TestRenderClientConf_HeaderProtectionKeyVersionGated.
func TestRenderClientConf_DeviceFieldsGated(t *testing.T) {
	awg3 := true
	SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { SetModuleSupportsAwg3(nil) })
	const fields = `"contentPaddingAddition":32,"rekeyAfterTime":120,"rekeyTimeout":5,"rejectAfterTime":180,"keepaliveTimeout":10,"maxHandshakeAttempts":18`
	const v3 = `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"awgVersion":"3",` + fields + `}`
	const v2 = `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"awgVersion":"2",` + fields + `}`
	wantLines := []string{
		"ContentPaddingAddition = 32", "RekeyAfterTime = 120", "RekeyTimeout = 5",
		"RejectAfterTime = 180", "KeepaliveTimeout = 10", "MaxHandshakeAttempts = 18",
	}
	t.Run("v3 emits", func(t *testing.T) {
		o := &model.AwgOutbound{Id: 1, Settings: v3}
		ci, _ := ClientInstanceFromOutbound(o)
		conf := renderClientConf(ci)
		for _, w := range wantLines {
			if !strings.Contains(conf, w) {
				t.Errorf("missing %q in:\n%s", w, conf)
			}
		}
	})
	t.Run("v2 omits", func(t *testing.T) {
		o := &model.AwgOutbound{Id: 1, Settings: v2}
		ci, _ := ClientInstanceFromOutbound(o)
		conf := renderClientConf(ci)
		for _, w := range wantLines {
			if strings.Contains(conf, w) {
				t.Errorf("%q must NOT appear for version 2 in:\n%s", w, conf)
			}
		}
	})
}

func TestRenderClientConf_Awg31Flags(t *testing.T) {
	awg3, awg31 := true, true
	SetModuleSupportsAwg3(&awg3)
	SetModuleSupportsAwg31(&awg31)
	t.Cleanup(func() {
		SetModuleSupportsAwg3(nil)
		SetModuleSupportsAwg31(nil)
	})
	const base = `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150`
	t.Run("v3.1 emits", func(t *testing.T) {
		o := &model.AwgOutbound{Id: 1, Settings: base + `,"awgVersion":"3.1","randomTrailers":true,"disableCookies":true}`}
		ci, _ := ClientInstanceFromOutbound(o)
		conf := renderClientConf(ci)
		if !strings.Contains(conf, "RandomTrailers = on") || !strings.Contains(conf, "DisableCookies = on") {
			t.Fatalf("want 3.1 flags, got:\n%s", conf)
		}
	})
	t.Run("v3 omits", func(t *testing.T) {
		o := &model.AwgOutbound{Id: 1, Settings: base + `,"awgVersion":"3","randomTrailers":true,"disableCookies":true}`}
		ci, _ := ClientInstanceFromOutbound(o)
		conf := renderClientConf(ci)
		if strings.Contains(conf, "RandomTrailers") || strings.Contains(conf, "DisableCookies") {
			t.Fatalf("v3 must omit 3.1 flags, got:\n%s", conf)
		}
	})
}
