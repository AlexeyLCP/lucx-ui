// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestValidateTproxySettings_nodeInboundDoesNotReadMasterPaths(t *testing.T) {
	id := 1
	ib := &model.Inbound{
		Protocol: model.Tproxy,
		NodeID:   &id,
		Settings: `{"hostname":"node.example.com","port":8443,"secret":"00112233445566778899aabbccddeeff","siteSource":"dir","siteDir":"/etc/passwd","carrierMode":"https","certFile":"/etc/shadow","keyFile":"/etc/shadow"}`,
	}
	if err := (&InboundService{}).validateTproxySettings(ib); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTrustTunnelCert_fromNodeSyncSkipsFiles(t *testing.T) {
	ib := &model.Inbound{
		Protocol: model.TrustTunnel,
		Settings: `{"hostname":"n.example","listen":"0.0.0.0:8443","upstreamProtocol":"http2","certFile":"/no/such.pem","keyFile":"/no/such.key"}`,
	}
	if err := (&InboundService{FromNodeSync: true}).validateTrustTunnelCert(ib); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTrustTunnelPortConflict_skipsNodeInbound(t *testing.T) {
	id := 1
	ib := &model.Inbound{
		Protocol: model.TrustTunnel,
		Port:     2053,
		NodeID:   &id,
		Settings: `{"listen":"0.0.0.0:2053","hostname":"n.example","upstreamProtocol":"http2"}`,
	}
	if err := (&InboundService{}).checkTrustTunnelPortConflict(ib, 0); err != nil {
		t.Fatal(err)
	}
}
