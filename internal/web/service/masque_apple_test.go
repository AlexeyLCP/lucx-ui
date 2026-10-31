package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBuildAppleRelayDDM(t *testing.T) {
	doc, err := BuildAppleRelayDDM(&model.Inbound{Id: 7, Remark: "relay", Enable: true, Port: 8443, Protocol: model.MASQUE, StreamSettings: `{"path":"/relay"}`}, "relay.example.com", AppleRelayExportRequest{MatchDomains: []string{"example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "apple-network-relay-ddm" || doc.Declaration.Type != "com.apple.configuration.networkrelay" {
		t.Fatalf("unexpected DDM: %+v", doc)
	}
	if got := doc.Declaration.Payload.Relays[0].HTTP3RelayURL; got != "https://relay.example.com:8443/relay" {
		t.Fatalf("relay URL = %q", got)
	}
	if doc.Declaration.Payload.RelayUUID == "" {
		t.Fatal("relay UUID missing")
	}
}

func TestBuildAppleRelayDDMRejectsNonMasque(t *testing.T) {
	_, err := BuildAppleRelayDDM(&model.Inbound{Protocol: model.VLESS, Enable: true}, "relay.example.com", AppleRelayExportRequest{})
	if err == nil {
		t.Fatal("expected protocol validation error")
	}
}
