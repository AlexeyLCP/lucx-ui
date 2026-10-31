package tunnel

import (
	"strings"
	"testing"
)

func TestMtprotoZigBackendIsOptInAndRendersLegacyCompatibleFields(t *testing.T) {
	cfg := TproxyConfig{Enabled: true, Backend: "legacy", Hostname: "relay.example", Secret: strings.Repeat("a", 32), Port: 443}
	inst, ok := MtprotoZigConfigFromTproxy(3, cfg)
	if !ok || inst.Enabled {
		t.Fatalf("legacy backend = %+v, ok=%v", inst, ok)
	}
	cfg.Backend = "mtproto-zig"
	inst, ok = MtprotoZigConfigFromTproxy(3, cfg)
	if !ok || !inst.Enabled || !strings.Contains(inst.ConfigText, `domain = "relay.example"`) || !strings.Contains(inst.ConfigText, `user = "`+strings.Repeat("a", 32)+`"`) {
		t.Fatalf("zig backend = %+v, ok=%v", inst, ok)
	}
}
