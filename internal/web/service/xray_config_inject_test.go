package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/op/go-logging"
)

func TestMain(m *testing.M) {
	// A test binary re-executed with MTG_FAKE_CHILD=1 poses as an mtg child
	// process (see mtproto_fake_test.go) and never reaches the test runner.
	if os.Getenv("MTG_FAKE_CHILD") == "1" {
		fakeMtgChildMain()
	}
	// injectPanelEgress logs when it skips injection; the package logger must
	// exist before any test exercises a skipped path.
	xuilogger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

func TestEnsureAPIServices(t *testing.T) {
	// legacy template without RoutingService gets it injected
	out := ensureAPIServices(json_util.RawMessage(`{"services":["HandlerService","LoggerService","StatsService"],"tag":"api"}`))
	var parsed struct {
		Services []string `json:"services"`
		Tag      string   `json:"tag"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"HandlerService": true, "StatsService": true, "RoutingService": true, "LoggerService": true}
	if len(parsed.Services) != 4 {
		t.Fatalf("expected 4 services, got %v", parsed.Services)
	}
	for _, svc := range parsed.Services {
		if !want[svc] {
			t.Fatalf("unexpected service %q", svc)
		}
	}
	if parsed.Tag != "api" {
		t.Fatalf("tag must be preserved, got %q", parsed.Tag)
	}

	// complete api block is returned unchanged (no marshal churn)
	full := json_util.RawMessage(`{"services":["HandlerService","StatsService","RoutingService"],"tag":"api"}`)
	if got := ensureAPIServices(full); string(got) != string(full) {
		t.Fatalf("complete api block must pass through untouched, got %s", got)
	}

	// absent api block stays absent
	if got := ensureAPIServices(nil); got != nil {
		t.Fatalf("nil api block must stay nil, got %s", got)
	}
}

func TestEnsureStatsPolicy(t *testing.T) {
	// default-template shape: level "0" exists with traffic flags — the online
	// flag is added and the siblings survive untouched
	out := ensureStatsPolicy(json_util.RawMessage(`{"levels":{"0":{"handshake":4,"statsUserUplink":true,"statsUserDownlink":true}},"system":{"statsInboundDownlink":true}}`))
	var parsed struct {
		Levels map[string]map[string]any `json:"levels"`
		System map[string]any            `json:"system"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	level0 := parsed.Levels["0"]
	if level0["statsUserOnline"] != true {
		t.Fatalf("statsUserOnline must be injected into level 0, got %v", level0)
	}
	if level0["statsUserUplink"] != true || level0["statsUserDownlink"] != true || level0["handshake"] != float64(4) {
		t.Fatalf("sibling keys must be preserved, got %v", level0)
	}
	if parsed.System["statsInboundDownlink"] != true {
		t.Fatalf("system block must be preserved, got %v", parsed.System)
	}

	// missing levels block: level "0" is created with the flag
	out = ensureStatsPolicy(json_util.RawMessage(`{"system":{}}`))
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Levels["0"]["statsUserOnline"] != true {
		t.Fatalf("level 0 must be created with statsUserOnline, got %s", out)
	}

	// every level gets the flag, an explicit false included — the flag is
	// panel infrastructure, like the api services
	out = ensureStatsPolicy(json_util.RawMessage(`{"levels":{"0":{"statsUserOnline":false},"1":{"connIdle":300}}}`))
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"0", "1"} {
		if parsed.Levels[key]["statsUserOnline"] != true {
			t.Fatalf("level %s must have statsUserOnline forced on, got %s", key, out)
		}
	}
	if parsed.Levels["1"]["connIdle"] != float64(300) {
		t.Fatalf("level 1 siblings must be preserved, got %s", out)
	}

	// already-enabled input passes through byte-identical (no marshal churn,
	// no spurious restart)
	full := json_util.RawMessage(`{"levels":{"0":{"statsUserOnline":true}}}`)
	if got := ensureStatsPolicy(full); string(got) != string(full) {
		t.Fatalf("already-enabled policy must pass through untouched, got %s", got)
	}

	// absent policy block stays absent
	if got := ensureStatsPolicy(nil); got != nil {
		t.Fatalf("nil policy must stay nil, got %s", got)
	}

	// unparsable policy is left untouched
	bad := json_util.RawMessage(`{not json`)
	if got := ensureStatsPolicy(bad); string(got) != string(bad) {
		t.Fatalf("unparsable policy must be left untouched, got %s", got)
	}
}

func egressTestConfig() *xray.Config {
	return &xray.Config{
		RouterConfig: json_util.RawMessage(`{"domainStrategy":"AsIs","rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"}]}`),
		InboundConfigs: []xray.InboundConfig{
			{Port: 62789, Protocol: "tunnel", Tag: "api", Listen: json_util.RawMessage(`"127.0.0.1"`)},
		},
	}
}

type egressRouting struct {
	DomainStrategy string `json:"domainStrategy"`
	Rules          []struct {
		InboundTag  []string `json:"inboundTag"`
		OutboundTag string   `json:"outboundTag"`
		Type        string   `json:"type"`
	} `json:"rules"`
}

func TestInjectPanelEgress(t *testing.T) {
	cfg := egressTestConfig()
	injectPanelEgress(cfg, "warp")

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("expected the egress inbound to be appended, got %d inbounds", len(cfg.InboundConfigs))
	}
	ib := cfg.InboundConfigs[1]
	if ib.Tag != PanelEgressInboundTag || ib.Protocol != "socks" || ib.Port != panelEgressBasePort {
		t.Fatalf("unexpected egress inbound: %+v", ib)
	}
	if string(ib.Listen) != `"127.0.0.1"` {
		t.Fatalf("egress inbound must listen on loopback, got %s", ib.Listen)
	}

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if routing.DomainStrategy != "AsIs" {
		t.Fatalf("routing keys outside rules must be preserved, got %+v", routing)
	}
	if len(routing.Rules) != 2 {
		t.Fatalf("expected egress rule + existing rule, got %+v", routing.Rules)
	}
	first := routing.Rules[0]
	if first.Type != "field" || first.OutboundTag != "warp" ||
		len(first.InboundTag) != 1 || first.InboundTag[0] != PanelEgressInboundTag {
		t.Fatalf("egress rule must be prepended, got %+v", first)
	}
}

func TestInjectPanelEgress_BalancerTag(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{"domainStrategy":"AsIs","rules":[],"balancers":[{"tag":"lb","selector":["warp"]}]}`)

	// A tag that names a balancer must be targeted via balancerTag so the
	// router resolves it; an outbound tag coexisting with balancers still uses
	// outboundTag.
	injectPanelEgress(cfg, "lb")

	var routing struct {
		Rules []struct {
			InboundTag  []string `json:"inboundTag"`
			OutboundTag string   `json:"outboundTag"`
			BalancerTag string   `json:"balancerTag"`
			Type        string   `json:"type"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 1 {
		t.Fatalf("expected the egress rule, got %+v", routing.Rules)
	}
	first := routing.Rules[0]
	if first.BalancerTag != "lb" || first.OutboundTag != "" {
		t.Fatalf("a balancer tag must target balancerTag, not outboundTag, got %+v", first)
	}
	if len(first.InboundTag) != 1 || first.InboundTag[0] != PanelEgressInboundTag {
		t.Fatalf("egress rule must bind the egress inbound, got %+v", first)
	}

	// A non-balancer tag alongside balancers keeps the plain outbound path.
	cfg2 := egressTestConfig()
	cfg2.RouterConfig = json_util.RawMessage(`{"rules":[],"balancers":[{"tag":"lb","selector":["warp"]}]}`)
	injectPanelEgress(cfg2, "warp")
	var routing2 struct {
		Rules []struct {
			OutboundTag string `json:"outboundTag"`
			BalancerTag string `json:"balancerTag"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(cfg2.RouterConfig, &routing2); err != nil {
		t.Fatal(err)
	}
	if routing2.Rules[0].OutboundTag != "warp" || routing2.Rules[0].BalancerTag != "" {
		t.Fatalf("a concrete outbound must target outboundTag, got %+v", routing2.Rules[0])
	}
}

func TestInjectPanelEgress_PortCollision(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(
		cfg.InboundConfigs,
		xray.InboundConfig{Port: panelEgressBasePort, Protocol: "vless", Tag: "in-1"},
		xray.InboundConfig{Port: panelEgressBasePort + 1, Protocol: "vless", Tag: "in-2"},
	)
	injectPanelEgress(cfg, "direct")
	got := cfg.InboundConfigs[len(cfg.InboundConfigs)-1]
	if got.Tag != PanelEgressInboundTag || got.Port != panelEgressBasePort+2 {
		t.Fatalf("egress inbound must skip taken ports, got %+v", got)
	}
}

func TestInjectPanelEgress_TagCollisionSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(
		cfg.InboundConfigs,
		xray.InboundConfig{Port: 1234, Protocol: "socks", Tag: PanelEgressInboundTag},
	)
	before := string(cfg.RouterConfig)
	injectPanelEgress(cfg, "direct")
	if len(cfg.InboundConfigs) != 2 || string(cfg.RouterConfig) != before {
		t.Fatal("a user inbound owning the egress tag must make injection a no-op")
	}
}

func TestInjectPanelEgress_NoRoutingSection(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = nil
	injectPanelEgress(cfg, "direct")

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 1 || routing.Rules[0].OutboundTag != "direct" {
		t.Fatalf("a routing section must be created with the egress rule, got %+v", routing)
	}
	if len(cfg.InboundConfigs) != 2 {
		t.Fatal("egress inbound must still be appended")
	}
}

func TestInjectPanelEgress_BadRoutingSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{not json`)
	injectPanelEgress(cfg, "direct")
	if len(cfg.InboundConfigs) != 1 {
		t.Fatal("unparsable routing must skip the whole injection, inbound included")
	}
	if string(cfg.RouterConfig) != `{not json` {
		t.Fatal("unparsable routing must be left untouched")
	}
}

func mtprotoInbound(tag string, settings string) *model.Inbound {
	return &model.Inbound{Tag: tag, Protocol: model.MTProto, Enable: true, Settings: settings}
}

func TestInjectMtprotoEgress_WithOutbound(t *testing.T) {
	cfg := egressTestConfig()
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50000,"outboundTag":"warp"}`))

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("expected the bridge inbound to be appended, got %d", len(cfg.InboundConfigs))
	}
	ib := cfg.InboundConfigs[1]
	if ib.Tag != "inbound-443" || ib.Protocol != "socks" || ib.Port != 50000 {
		t.Fatalf("unexpected bridge inbound: %+v", ib)
	}
	if string(ib.Listen) != `"127.0.0.1"` {
		t.Fatalf("bridge must listen on loopback, got %s", ib.Listen)
	}

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 2 {
		t.Fatalf("expected the egress rule prepended to the existing rule, got %+v", routing.Rules)
	}
	first := routing.Rules[0]
	if first.Type != "field" || first.OutboundTag != "warp" ||
		len(first.InboundTag) != 1 || first.InboundTag[0] != "inbound-443" {
		t.Fatalf("egress rule must bind the inbound tag to the outbound, got %+v", first)
	}
}

func TestInjectMtprotoEgress_NoOutboundLeavesRouting(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.RouterConfig)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50001}`))

	if len(cfg.InboundConfigs) != 2 || cfg.InboundConfigs[1].Port != 50001 {
		t.Fatalf("bridge must still be appended without an outbound, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != before {
		t.Fatalf("no outbound means no rule change, got %s", cfg.RouterConfig)
	}
}

func TestInjectMtprotoEgress_BalancerTag(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{"rules":[],"balancers":[{"tag":"lb","selector":["warp"]}]}`)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50002,"outboundTag":"lb"}`))

	var routing struct {
		Rules []struct {
			OutboundTag string `json:"outboundTag"`
			BalancerTag string `json:"balancerTag"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 1 || routing.Rules[0].BalancerTag != "lb" || routing.Rules[0].OutboundTag != "" {
		t.Fatalf("a balancer tag must target balancerTag, got %+v", routing.Rules)
	}
}

func TestInjectMtprotoEgress_Disabled(t *testing.T) {
	// Not routed, and routed-but-portless, are both no-ops.
	for _, settings := range []string{
		`{"routeThroughXray":false,"routeXrayPort":50000}`,
		`{"routeThroughXray":true}`,
		`{"routeThroughXray":true,"routeXrayPort":0}`,
	} {
		cfg := egressTestConfig()
		before := string(cfg.RouterConfig)
		injectMtprotoEgress(cfg, mtprotoInbound("inbound-443", settings))
		if len(cfg.InboundConfigs) != 1 || string(cfg.RouterConfig) != before {
			t.Fatalf("settings %s must be a no-op, got %d inbounds", settings, len(cfg.InboundConfigs))
		}
	}
}

func TestInjectMtprotoEgress_TagCollisionSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: 443, Protocol: "vless", Tag: "inbound-443"})
	before := string(cfg.RouterConfig)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50003,"outboundTag":"warp"}`))
	if len(cfg.InboundConfigs) != 2 || string(cfg.RouterConfig) != before {
		t.Fatal("a real inbound already owning the tag must make the bridge a no-op")
	}
}

// LUCX-HOOK: AWG injectAwgEgress tests, mirroring the mtproto suite above.

func awgInbound(tag string, settings string) *model.Inbound {
	return &model.Inbound{Tag: tag, Protocol: model.AWG, Enable: true, Settings: settings, Id: 1}
}

func TestInjectAwgEgress_WithOutbound(t *testing.T) {
	cfg := egressTestConfig()
	injectAwgEgress(cfg, awgInbound("inbound-awg-1",
		`{"routeThroughXray":true,"outboundTag":"warp","mtu":1320}`))

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("expected the TUN egress inbound to be appended, got %d inbounds", len(cfg.InboundConfigs))
	}
	ib := cfg.InboundConfigs[1]
	if ib.Tag != "inbound-awg-1" || ib.Protocol != "tun" {
		t.Fatalf("unexpected egress inbound: %+v", ib)
	}
	var settings struct {
		Name    string   `json:"name"`
		MTU     int      `json:"mtu"`
		Gateway []string `json:"gateway"`
	}
	if err := json.Unmarshal(ib.Settings, &settings); err != nil {
		t.Fatalf("TUN settings unmarshal: %v", err)
	}
	if settings.Name != "tun1" {
		t.Errorf("expected tun name tun1, got %s", settings.Name)
	}
	if settings.MTU != 1320 {
		t.Errorf("expected mtu 1320, got %d", settings.MTU)
	}
	if len(settings.Gateway) != 1 || settings.Gateway[0] != "10.254.1.1/30" {
		t.Errorf("expected gateway [10.254.1.1/30] (per-inbound /30 outside the AWG subnet), got %v", settings.Gateway)
	}
	// Routing rule prepended with outboundTag.
	var r egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Rules) != 2 {
		t.Fatalf("expected 2 rules (awg + original api), got %d", len(r.Rules))
	}
	if r.Rules[0].OutboundTag != "warp" || len(r.Rules[0].InboundTag) != 1 || r.Rules[0].InboundTag[0] != "inbound-awg-1" {
		t.Fatalf("prepended rule wrong: %+v", r.Rules[0])
	}
}

func TestInjectAwgEgress_NoOutboundLeavesRouting(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.RouterConfig)
	injectAwgEgress(cfg, awgInbound("inbound-awg-1",
		`{"routeThroughXray":true,"mtu":1320}`))

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("TUN inbound must still be appended without an outbound, got %d inbounds", len(cfg.InboundConfigs))
	}
	if string(cfg.RouterConfig) != before {
		t.Fatal("routing must be untouched when no outboundTag is set (Xray kettle mode)")
	}
}

func TestInjectAwgEgress_Disabled(t *testing.T) {
	for _, settings := range []string{
		`{"routeThroughXray":false,"outboundTag":"warp"}`,
		`{}`,
		`not json`,
	} {
		cfg := egressTestConfig()
		before := string(cfg.RouterConfig)
		injectAwgEgress(cfg, awgInbound("inbound-awg-1", settings))
		if len(cfg.InboundConfigs) != 1 || string(cfg.RouterConfig) != before {
			t.Fatalf("settings %s must be a no-op, got %d inbounds", settings, len(cfg.InboundConfigs))
		}
	}
}

func TestInjectAwgEgress_TagCollisionSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: 443, Protocol: "vless", Tag: "inbound-awg-1"})
	before := string(cfg.RouterConfig)
	injectAwgEgress(cfg, awgInbound("inbound-awg-1",
		`{"routeThroughXray":true,"outboundTag":"warp"}`))
	if len(cfg.InboundConfigs) != 2 || string(cfg.RouterConfig) != before {
		t.Fatal("a real inbound already owning the tag must make the TUN bridge a no-op")
	}
}

func TestInjectAwgEgress_DefaultMTUAndGateway(t *testing.T) {
	cfg := egressTestConfig()
	injectAwgEgress(cfg, awgInbound("inbound-awg-1",
		`{"routeThroughXray":true}`))
	ib := cfg.InboundConfigs[1]
	var settings struct {
		MTU     int      `json:"mtu"`
		Gateway []string `json:"gateway"`
	}
	if err := json.Unmarshal(ib.Settings, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.MTU != 1320 {
		t.Errorf("expected default mtu 1320, got %d", settings.MTU)
	}
	if len(settings.Gateway) != 1 || settings.Gateway[0] != "10.254.1.1/30" {
		t.Errorf("expected default gateway [10.254.1.1/30] (per-inbound /30), got %v", settings.Gateway)
	}
}

// The router can only match domain rules against AWG traffic when the TUN
// inbound sniffs the SNI/Host out of the flows. routeOnly keeps the sniffed
// domain a routing-time hint: the dial target stays the IP the client
// resolved, so enabling sniffing never changes where traffic actually goes.
func TestInjectAwgEgress_SniffingRouteOnly(t *testing.T) {
	cfg := egressTestConfig()
	injectAwgEgress(cfg, awgInbound("inbound-awg-1", `{"routeThroughXray":true,"mtu":1320}`))
	ib := cfg.InboundConfigs[1]
	var sniffing struct {
		Enabled      bool     `json:"enabled"`
		DestOverride []string `json:"destOverride"`
		RouteOnly    bool     `json:"routeOnly"`
	}
	if err := json.Unmarshal(ib.Sniffing, &sniffing); err != nil {
		t.Fatalf("injected TUN inbound must carry a sniffing block, got %q: %v", ib.Sniffing, err)
	}
	if !sniffing.Enabled || !sniffing.RouteOnly {
		t.Fatalf("sniffing must be enabled with routeOnly, got %+v", sniffing)
	}
	want := []string{"http", "tls", "quic"}
	if len(sniffing.DestOverride) != len(want) {
		t.Fatalf("destOverride = %v, want %v", sniffing.DestOverride, want)
	}
	for i, w := range want {
		if sniffing.DestOverride[i] != w {
			t.Fatalf("destOverride = %v, want %v", sniffing.DestOverride, want)
		}
	}
}

// Two routed AWG inbounds must not share a TUN address or a routing table, so
// the gateway is derived from the inbound id, in step with tunN and table
// 1000+N on the awg side.
func TestInjectAwgEgress_PerInboundGateway(t *testing.T) {
	cfg := egressTestConfig()
	ib := awgInbound("inbound-awg-7", `{"routeThroughXray":true,"mtu":1400}`)
	ib.Id = 7
	injectAwgEgress(cfg, ib)
	got := cfg.InboundConfigs[len(cfg.InboundConfigs)-1]
	var settings struct {
		Name    string   `json:"name"`
		Gateway []string `json:"gateway"`
	}
	if err := json.Unmarshal(got.Settings, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Name != "tun7" {
		t.Errorf("expected tun name tun7, got %s", settings.Name)
	}
	if len(settings.Gateway) != 1 || settings.Gateway[0] != "10.254.7.1/30" {
		t.Errorf("expected gateway [10.254.7.1/30], got %v", settings.Gateway)
	}
}

// END LUCX-HOOK
