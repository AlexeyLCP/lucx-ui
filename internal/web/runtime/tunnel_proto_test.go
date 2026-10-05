package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestIsTunnelInboundProto_Csqtt(t *testing.T) {
	if !isTunnelInboundProto(model.Csqtt) || !isTunnelInboundProto(model.Qwdtt) || !isTunnelInboundProto(model.Openflux) {
		t.Fatal("csqtt/qwdtt must not be pushed to the xray API")
	}
	if isTunnelInboundProto(model.VLESS) || isTunnelInboundProto(model.AWG) {
		t.Fatal("vless/awg are not tunnel inbound protos")
	}
}

func TestTunnelUpdateDropsRunning(t *testing.T) {
	if tunnelUpdateDropsRunning(model.Gateway, model.Gateway, true) {
		t.Fatal("same-protocol enable must keep the process")
	}
	if tunnelUpdateDropsRunning(model.Cover, model.Cover, true) {
		t.Fatal("cover no-op update must keep the process")
	}
	if !tunnelUpdateDropsRunning(model.Gateway, model.Gateway, false) {
		t.Fatal("disable must stop")
	}
	if !tunnelUpdateDropsRunning(model.Gateway, model.VLESS, true) {
		t.Fatal("leaving a tunnel proto must stop the sidecar")
	}
	if !tunnelUpdateDropsRunning(model.VLESS, model.Gateway, true) {
		t.Fatal("incoming tunnel must remove the old handler")
	}
}
