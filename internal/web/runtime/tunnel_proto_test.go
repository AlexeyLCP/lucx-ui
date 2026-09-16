package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestIsTunnelInboundProto_Csqtt(t *testing.T) {
	if !isTunnelInboundProto(model.Csqtt) || !isTunnelInboundProto(model.Qwdtt) {
		t.Fatal("csqtt/qwdtt must not be pushed to the xray API")
	}
	if isTunnelInboundProto(model.VLESS) || isTunnelInboundProto(model.AWG) {
		t.Fatal("vless/awg are not tunnel inbound protos")
	}
}
