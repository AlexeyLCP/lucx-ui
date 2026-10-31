package tunnel

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestTelemtRender(t *testing.T) {
	cfg := TelemtConfig{Port: 443, TlsDomain: "example.com", Users: []TelemtUser{{Name: "alice@example", Secret: "ee0011", Enable: true}}}
	got := cfg.Render()
	for _, want := range []string{"[general.modes]", "tls = true", "tls_domain = \"example.com\"", "\"alice@example\" = \"ee0011\""} {
		if !strings.Contains(got, want) {
			t.Errorf("render missing %q:\n%s", want, got)
		}
	}
}

func TestTelemtInstancesFromInboundKeepsDisabledWithoutStarting(t *testing.T) {
	ib := &model.Inbound{Id: 7, Protocol: model.Telemt, Port: 443, Enable: false, Settings: `{"tlsDomain":"example.com"}`}
	insts, ok := TelemtInstancesFromInbound(ib)
	if !ok || len(insts) != 1 || insts[0].Enabled {
		t.Fatalf("instances = %+v ok=%v", insts, ok)
	}
}
