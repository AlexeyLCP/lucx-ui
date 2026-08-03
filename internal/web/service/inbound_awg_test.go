package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

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
		{"empty v3", base + `,"awgVersion":"3","headerProtectionKey":""}`, false, "3"},
		{"no version", base + `,"headerProtectionKey":"aBcD...base64hpk=="}`, false, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, obf, ver := inboundAwgHints(tc.settings)
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
			_, obf, ver := inboundAwgHints(tc.settings)
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
