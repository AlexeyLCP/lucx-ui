package service

import (
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestInboundAwgHints_HIndexesStayAligned(t *testing.T) {
	settings := `{"address":"10.200.0.1/24","jc":4,"jmin":10,"jmax":50,"s1":40,"s2":60,"h1":"1-10","h3":"30-40","h4":"50-60","awgVersion":"2"}`
	_, obf, _ := inboundAwgHints(settings, true)
	if !strings.Contains(obf, "H1 = 1-10") || !strings.Contains(obf, "H3 = 30-40") || !strings.Contains(obf, "H4 = 50-60") {
		t.Fatalf("H labels misaligned:\n%s", obf)
	}
	if strings.Contains(obf, "H2 =") {
		t.Fatalf("empty H2 must stay omitted:\n%s", obf)
	}
}

func TestInboundHasSidecar(t *testing.T) {
	if !inboundHasSidecar(model.Naive) || !inboundHasSidecar(model.AWG) || inboundHasSidecar(model.VLESS) {
		t.Fatal("sidecar protocols must teardown on delete even when disabled")
	}
}

func TestAwgRoutesThroughXray(t *testing.T) {
	cases := map[string]struct {
		ib   *model.Inbound
		want bool
	}{
		"routed":   {&model.Inbound{Protocol: model.AWG, Settings: `{"routeThroughXray":true}`}, true},
		"off":      {&model.Inbound{Protocol: model.AWG, Settings: `{"routeThroughXray":false}`}, false},
		"absent":   {&model.Inbound{Protocol: model.AWG, Settings: `{}`}, false},
		"non-awg":  {&model.Inbound{Protocol: model.VLESS, Settings: `{"routeThroughXray":true}`}, false},
		"bad json": {&model.Inbound{Protocol: model.AWG, Settings: `{nope`}, false},
		"nil":      {nil, false},
	}
	for name, c := range cases {
		if got := awgRoutesThroughXray(c.ib); got != c.want {
			t.Fatalf("%s: got %v want %v", name, got, c.want)
		}
	}
}

// initAwgServiceTest gives each test a throwaway DB and pins the runtime
// manager to nil so nodePushPlan takes the push=false path — no sidecar
// process is ever started from a unit test.
func initAwgServiceTest(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	prev := runtime.GetManager()
	runtime.SetManager(nil)
	t.Cleanup(func() { runtime.SetManager(prev) })
}

func routedAwgTestInbound(port int) *model.Inbound {
	return &model.Inbound{
		UserId:         1,
		Port:           port,
		Protocol:       model.AWG,
		Remark:         "awg-routed",
		Enable:         true,
		Settings:       `{"privateKey":"test-priv","address":"10.8.0.1/24","routeThroughXray":true,"outboundTag":"warp","clients":[]}`,
		StreamSettings: `{}`,
		Sniffing:       `{}`,
	}
}

// The TUN egress inbound lives only in the generated Xray config, so creating
// a routed AWG inbound must force a config regen — exactly like a routed
// mtproto inbound does. Without needRestart the TUN device never appears and
// client traffic leaves un-NATed through the default route.
func TestAddInbound_RoutedAwgForcesXrayRegen(t *testing.T) {
	initAwgServiceTest(t)
	svc := &InboundService{}
	_, needRestart, err := svc.AddInbound(routedAwgTestInbound(40199))
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if !needRestart {
		t.Fatal("adding a routed AWG inbound must set needRestart so injectAwgEgress runs")
	}
}

func TestAddInbound_PlainAwgDoesNotForceRegen(t *testing.T) {
	initAwgServiceTest(t)
	svc := &InboundService{}
	ib := routedAwgTestInbound(40198)
	ib.Settings = `{"privateKey":"test-priv","address":"10.8.0.1/24","routeThroughXray":false,"clients":[]}`
	_, needRestart, err := svc.AddInbound(ib)
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	if needRestart {
		t.Fatal("a kernel-routed AWG inbound must not force an Xray restart")
	}
}

func TestDelInbound_RoutedAwgForcesXrayRegen(t *testing.T) {
	initAwgServiceTest(t)
	ib := routedAwgTestInbound(40197)
	ib.Enable = false
	ib.Tag = "awg-del-test"
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	svc := &InboundService{}
	needRestart, err := svc.DelInbound(ib.Id)
	if err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	if !needRestart {
		t.Fatal("deleting a routed AWG inbound must set needRestart so the TUN inbound is dropped from the config")
	}
}

// Disabling a routed AWG inbound must drop its TUN inbound from the generated
// config, so needRestart has to come back true. This runs with a real local
// runtime: the disable path only calls awg.Manager.Remove, a no-op for an
// interface that was never started, so no awg-quick process is ever spawned.
// The enable direction would start a kernel interface and is deliberately left
// to the on-server verification, as is UpdateInbound (whose nodePushPlan
// fallback forces needRestart=true whenever the runtime cannot be reached,
// masking the routing check from a unit test).
func TestSetInboundEnable_DisableRoutedAwgForcesXrayRegen(t *testing.T) {
	initAwgServiceTest(t)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	ib := routedAwgTestInbound(40196)
	ib.Tag = "awg-enable-test"
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	svc := &InboundService{}
	needRestart, err := svc.SetInboundEnable(ib.Id, false)
	if err != nil {
		t.Fatalf("SetInboundEnable: %v", err)
	}
	if !needRestart {
		t.Fatal("disabling a routed AWG inbound must set needRestart so the TUN inbound is dropped from the config")
	}
}

// TestInboundAwgHints_HeaderProtectionKeyVersionGated pins the AWG3 header
// protection key into the obfuscation block ONLY when awgVersion == "3" and the
// key is non-empty. The block is the inbound's "ceiling"; the clients page
// filters it down per export version (filterAwgObfuscation). Upstream kernel
// v3.0.20260731 + tools v3.0.20260730 parse the field; older builds reject it,
// so v1/v2 inbounds must never carry it.
func TestInboundAwgHints_HeaderProtectionKeyVersionGated(t *testing.T) {
	awg3 := true
	awg.SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { awg.SetModuleSupportsAwg3(nil) })
	const base = `{"address":"10.8.0.1/24","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000"`
	for _, tc := range []struct {
		name     string
		settings string
		wantHPK  bool
		wantVer  string
	}{
		{"absent", base + `}`, false, "2"},
		{"empty v2", base + `,"awgVersion":"2","headerProtectionKey":""}`, false, "2"},
		{"set v2", base + `,"awgVersion":"2","headerProtectionKey":"aBcD...base64hpk=="}`, false, "2"},
		{"set v3", base + `,"awgVersion":"3","headerProtectionKey":"aBcD...base64hpk=="}`, true, "3"},
		{"set v3.1", base + `,"awgVersion":"3.1","headerProtectionKey":"aBcD...base64hpk=="}`, true, "3.1"},
		{"empty v3", base + `,"awgVersion":"3","headerProtectionKey":""}`, false, "3"},
		{"no version", base + `,"headerProtectionKey":"aBcD...base64hpk=="}`, false, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, obf, ver := inboundAwgHints(tc.settings, true)
			if strings.Contains(obf, "HeaderProtectionKey") != tc.wantHPK {
				t.Errorf("HeaderProtectionKey in block = %v, want %v, got:\n%s", !tc.wantHPK, tc.wantHPK, obf)
			}
			if ver != tc.wantVer {
				t.Errorf("version = %q, want %q", ver, tc.wantVer)
			}
			if !strings.Contains(obf, "Jc = 8") {
				t.Errorf("the rest of the obfuscation block must survive, got:\n%s", obf)
			}
		})
	}
}

// TestInboundAwgHints_DeviceFieldsEmission pins the six AWG3 device-level
// fields into the obfuscation block ONLY when awgVersion == "3" and the field
// is > 0. The block is the inbound's ceiling — the clients page filters it
// down per export version.
func TestInboundAwgHints_DeviceFieldsEmission(t *testing.T) {
	awg3 := true
	awg.SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { awg.SetModuleSupportsAwg3(nil) })
	deviceLines := []string{
		"ContentPaddingAddition = 32", "RekeyAfterTime = 120", "RekeyTimeout = 5",
		"RejectAfterTime = 180", "KeepaliveTimeout = 10", "MaxHandshakeAttempts = 18",
	}
	const fields = `"contentPaddingAddition":32,"rekeyAfterTime":120,"rekeyTimeout":5,"rejectAfterTime":180,"keepaliveTimeout":10,"maxHandshakeAttempts":18`
	const base = `{"address":"10.8.0.1/24","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000"`
	for _, tc := range []struct {
		name     string
		settings string
		want     []bool
		wantVer  string
	}{
		{"v3 all set", base + `,"awgVersion":"3",` + fields + `}`, []bool{true, true, true, true, true, true}, "3"},
		{"v2 all set", base + `,"awgVersion":"2",` + fields + `}`, []bool{false, false, false, false, false, false}, "2"},
		{"v3 none", base + `,"awgVersion":"3"}`, []bool{false, false, false, false, false, false}, "3"},
		{"v3 one set", base + `,"awgVersion":"3","rekeyAfterTime":120}`, []bool{false, true, false, false, false, false}, "3"},
		{"no version all set", base + `,` + fields + `}`, []bool{false, false, false, false, false, false}, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, obf, ver := inboundAwgHints(tc.settings, true)
			for i, line := range deviceLines {
				got := strings.Contains(obf, line)
				if got != tc.want[i] {
					t.Errorf("%s: want %q in block=%v, got=%v\nBlock:\n%s", tc.name, line, tc.want[i], got, obf)
				}
			}
			if ver != tc.wantVer {
				t.Errorf("version = %q, want %q", ver, tc.wantVer)
			}
			if !strings.Contains(obf, "Jc = 8") {
				t.Errorf("the rest of the obfuscation block must survive, got:\n%s", obf)
			}
		})
	}
}

// TestAwgOutboundSubnetConflict covers the lucx.64 guard that blocks an AWG
// inbound tunnel subnet already occupied by an AWG outbound (awgo-N) interface.
// A /24 (or wider) outbound prefix overlapping the inbound's /24 is a clash; a
// bare /32 host address is exempt (it installs no /24 connected route and
// defaultAwgClients already keeps client IPs off it).
func TestAwgOutboundSubnetConflict(t *testing.T) {
	inbound24 := netip.MustParsePrefix("10.8.0.1/24").Masked()
	cases := []struct {
		name    string
		newNet  netip.Prefix
		outAddr string
		want    bool
	}{
		{"same /24 clashes", inbound24, "10.8.0.2/24", true},
		{"outbound /24 host octet differs still clashes", inbound24, "10.8.0.99/24", true},
		{"wider /16 outbound clashes", inbound24, "10.8.5.5/16", true},
		{"bare /32 exempt", inbound24, "10.8.0.2/32", false},
		{"disjoint /24 no clash", inbound24, "10.8.5.1/24", false},
		{"different second octet no clash", inbound24, "10.200.0.1/24", false},
		{"empty outbound address no clash", inbound24, "", false},
		{"unparseable outbound address no clash", inbound24, "not-an-ip", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got := awgOutboundSubnetConflict(tc.newNet, tc.outAddr)
			if got != tc.want {
				t.Errorf("awgOutboundSubnetConflict(%v, %q) = %v, want %v", tc.newNet, tc.outAddr, got, tc.want)
			}
		})
	}
}

// TestInboundAwgHints_DropsOversizedIFields pins the same all-or-nothing gate
// renderServerConf already has: an oversized set must vanish whole.
func TestInboundAwgHints_DropsOversizedIFields(t *testing.T) {
	huge := strings.Repeat("x", 712) // 5 fields x 712 chars = 3600 IBytes, over the worst-case budget
	settings := `{"address":"10.8.0.1/24","awgVersion":"2",` +
		`"i1":"` + huge + `","i2":"` + huge + `","i3":"` + huge + `","i4":"` + huge + `","i5":"` + huge + `"}`
	_, obf, _ := inboundAwgHints(settings, true)
	for _, key := range []string{"I1 = ", "I2 = ", "I3 = ", "I4 = ", "I5 = "} {
		if strings.Contains(obf, key) {
			t.Errorf("oversized I-set must be dropped whole, found %q in block", key)
		}
	}
}

// TestInboundAwgHints_NodeInboundKeepsAwg3Fields pins localInbound as the only
// switch gating 3.x fields on this host's own tools-support probe.
func TestInboundAwgHints_NodeInboundKeepsAwg3Fields(t *testing.T) {
	unsupported := false
	awg.SetModuleSupportsAwg3(&unsupported)
	awg.SetModuleSupportsAwg31(&unsupported)
	t.Cleanup(func() {
		awg.SetModuleSupportsAwg3(nil)
		awg.SetModuleSupportsAwg31(nil)
	})
	const settings = `{"address":"10.8.0.1/24","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,` +
		`"awgVersion":"3.1","headerProtectionKey":"aBcD...base64hpk==","randomTrailers":true}`

	for _, tc := range []struct {
		name         string
		localInbound bool
		wantPresent  bool
	}{
		{"node inbound keeps fields the host cannot itself confirm", false, true},
		{"local inbound is still gated on this host's own probe", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, obf, _ := inboundAwgHints(settings, tc.localInbound)
			if strings.Contains(obf, "HeaderProtectionKey") != tc.wantPresent {
				t.Errorf("HeaderProtectionKey present = %v, want %v, got:\n%s", !tc.wantPresent, tc.wantPresent, obf)
			}
			if strings.Contains(obf, "RandomTrailers") != tc.wantPresent {
				t.Errorf("RandomTrailers present = %v, want %v, got:\n%s", !tc.wantPresent, tc.wantPresent, obf)
			}
		})
	}
}

// A field of blanks is not a value: "H1 =  " or a blank key makes awg setconf
// reject the whole file, exactly as the empty ones the .conf renderers skip.
func TestInboundAwgHints_BlankFieldsAreNotValues(t *testing.T) {
	awg3 := true
	awg.SetModuleSupportsAwg3(&awg3)
	t.Cleanup(func() { awg.SetModuleSupportsAwg3(nil) })
	const base = `{"address":"10.8.0.1/24","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40`
	for _, tc := range []struct {
		name, settings, absent, present string
	}{
		{"blank header", base + `,"awgVersion":"2","h1":"   ","h2":"600-900"}`, "H1 =", "H2 = 600-900"},
		{"blank key", base + `,"awgVersion":"3","headerProtectionKey":"   "}`, "HeaderProtectionKey", "Jc = 8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, obf, _ := inboundAwgHints(tc.settings, true)
			if strings.Contains(obf, tc.absent) {
				t.Errorf("%q must not reach the export, got:\n%q", tc.absent, obf)
			}
			if !strings.Contains(obf, tc.present) {
				t.Errorf("the rest of the block must survive, %q missing from:\n%q", tc.present, obf)
			}
		})
	}
}

// A blank I-field is not a value: this block is pasted verbatim into the client
// card and the vpn:// line, where "I1 =" makes awg setconf refuse the file.
func TestInboundAwgHints_BlankIFieldIsNotAValue(t *testing.T) {
	const base = `{"address":"10.8.0.1/24","awgVersion":"2","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40`
	for _, tc := range []struct{ name, i1, want string }{
		{"blanks only", "   ", ""},
		{"whitespace edges stay raw", " <b 0x00> ", "I1 =  <b 0x00> \n"},
		{"plain descriptor", "<b 0x00>", "I1 = <b 0x00>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, obf, _ := inboundAwgHints(base+`,"i1":"`+tc.i1+`","i2":"<b 0xff>"}`, true)
			switch {
			case tc.want == "" && strings.Contains(obf, "I1"):
				t.Errorf("blank I1 %q must not reach the export, got:\n%s", tc.i1, obf)
			case tc.want != "" && !strings.Contains(obf, tc.want):
				t.Errorf("missing %q, got:\n%s", tc.want, obf)
			}
			if !strings.Contains(obf, "I2 = <b 0xff>\n") {
				t.Errorf("the rest of the I-set must survive, got:\n%s", obf)
			}
		})
	}
}
