package xray

import (
	"net"
	"testing"

	routerService "github.com/xtls/xray-core/app/router/command"
	xnet "github.com/xtls/xray-core/common/net"
)

// xray-core 26.10.10 resolves routing.script from the loader's working directory, and the
// running core keeps its script anyway: a script beside the xray binary failed every hot
// rule change panel-side, turning it into a full restart and breaking the node bridge.
func TestBuildReloadableRouting_IgnoresLuaScript(t *testing.T) {
	t.Chdir(t.TempDir())
	routing := []byte(`{"script":"route.lua","rules":[{"type":"field","ip":["10.0.0.0/8"],"outboundTag":"direct"}]}`)

	config, err := buildReloadableRouting(routing)
	if err != nil {
		t.Fatalf("buildReloadableRouting: %v", err)
	}
	if config.Script != "" {
		t.Fatalf("Script = %q, want it left to the running core", config.Script)
	}
	if len(config.Rule) != 1 || config.Rule[0].GetTag() != "direct" {
		t.Fatalf("rules = %v, want the one rule to direct", config.Rule)
	}
}

// LUCX-HOOK: a rule that picks one AWG client by its tunnel IP must match
// that client's traffic and no other. The AWG bridge (TUN inbound) hands the
// decrypted packet to Xray with the client's tunnel address intact, so
// sourceIP is the only way to route one AWG client apart from the others —
// and the route tester must be able to ask the core about it.
func TestRoutingRule_SourceIPMatchesOneAwgClient(t *testing.T) {
	routing := []byte(`{"rules":[{"type":"field","sourceIP":["10.200.0.5/32"],"outboundTag":"warp"}]}`)
	config, err := buildReloadableRouting(routing)
	if err != nil {
		t.Fatalf("buildReloadableRouting: %v", err)
	}
	if len(config.Rule) != 1 {
		t.Fatalf("rules = %v, want one", config.Rule)
	}
	cond, err := config.Rule[0].BuildCondition()
	if err != nil {
		t.Fatalf("BuildCondition: %v", err)
	}

	routeContext := func(t *testing.T, sourceIP string) *routerService.RoutingContext {
		t.Helper()
		sources, err := routeTestIPBytes([]string{sourceIP})
		if err != nil {
			t.Fatalf("routeTestIPBytes(%s): %v", sourceIP, err)
		}
		return &routerService.RoutingContext{
			Network:    xnet.Network_TCP,
			SourceIPs:  sources,
			TargetIPs:  [][]byte{net.ParseIP("1.1.1.1").To4()},
			TargetPort: 443,
		}
	}

	if !cond.Apply(routerService.AsRoutingContext(routeContext(t, "10.200.0.5"))) {
		t.Fatal("sourceIP rule did not match the client's own tunnel IP")
	}
	if cond.Apply(routerService.AsRoutingContext(routeContext(t, "10.200.0.6"))) {
		t.Fatal("sourceIP rule matched another client's tunnel IP")
	}
}

func TestRouteTestIPBytes_RejectsGarbage(t *testing.T) {
	if _, err := routeTestIPBytes([]string{"not-an-ip"}); err == nil {
		t.Fatal("routeTestIPBytes accepted a non-IP")
	}
}

// END LUCX-HOOK
