package service

import (
	"path/filepath"
	"strings"
	"testing"

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

// TestInboundAwgHints_HeaderProtectionKey verifies the AWG3 header protection
// key flows into the pre-rendered obfuscation block that the frontend client
// .conf path consumes. Empty HPK must be omitted (the current master kernel
// module rejects the unknown field in setconf); a set HPK must appear as a
// `HeaderProtectionKey = <value>` line.
func TestInboundAwgHints_HeaderProtectionKey(t *testing.T) {
	t.Run("omitted when empty", func(t *testing.T) {
		settings := `{"address":"10.8.0.1/24","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000"}`
		_, obf := inboundAwgHints(settings)
		if strings.Contains(obf, "HeaderProtectionKey =") {
			t.Errorf("HeaderProtectionKey must be omitted when empty, got:\n%s", obf)
		}
	})
	t.Run("written when set", func(t *testing.T) {
		settings := `{"address":"10.8.0.1/24","jc":8,"jmin":50,"jmax":200,"s1":30,"s2":40,"s3":20,"s4":15,"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000","headerProtectionKey":"aBcD...base64hpk=="}`
		_, obf := inboundAwgHints(settings)
		if !strings.Contains(obf, "HeaderProtectionKey = aBcD...base64hpk==") {
			t.Errorf("HeaderProtectionKey must appear when set, got:\n%s", obf)
		}
	})
}
