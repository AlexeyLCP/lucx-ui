package service

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGatewayBindIPs(t *testing.T) {
	byID := map[int]*model.Inbound{
		1: {Id: 1, Listen: "10.0.0.6"},
		2: {Id: 2, Listen: "10.0.0.6"},
		3: {Id: 3, Listen: "10.0.0.7"},
		4: {Id: 4, Listen: ""},
		5: {Id: 5, Listen: "127.0.0.1"},
	}
	cases := []struct {
		name             string
		selected, inside map[int]bool
		want             []string
	}{
		{"one specific IP", map[int]bool{1: true}, nil, []string{"10.0.0.6"}},
		{"shared across selected and inside", map[int]bool{1: true}, map[int]bool{2: true}, []string{"10.0.0.6"}},
		{"loopback ignored", map[int]bool{1: true, 5: true}, nil, []string{"10.0.0.6"}},
		{"two IPs bind both", map[int]bool{3: true, 1: true}, nil, []string{"10.0.0.6", "10.0.0.7"}},
		{"wildcard falls back", map[int]bool{1: true, 4: true}, nil, []string{"10.0.0.1"}},
		{"only loopback falls back", map[int]bool{5: true}, nil, []string{"10.0.0.1"}},
	}
	for _, c := range cases {
		if got := gatewayBindIPs(byID, c.selected, c.inside, "10.0.0.1"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := gatewayBindIPs(byID, map[int]bool{4: true}, nil, ""); got != nil {
		t.Errorf("no NIC IP: got %v, want nil (all interfaces)", got)
	}
}

func TestBindsOverlap(t *testing.T) {
	binds := []string{"10.0.0.6", "10.0.0.7"}
	for listen, want := range map[string]bool{
		"10.0.0.6": true, "10.0.0.7": true, "10.0.0.8": false, "": true, "0.0.0.0": true,
	} {
		if got := bindsOverlap(bindAddr{listen: listen}, binds); got != want {
			t.Errorf("listen %q: got %v, want %v", listen, got, want)
		}
	}
	if !bindsOverlap(bindAddr{listen: "10.0.0.8"}, nil) {
		t.Error("gateway on all interfaces must overlap any IP")
	}
}

func TestOrphanGatewayHostIDs(t *testing.T) {
	nodeIb := 7
	hosts := []model.Host{
		{Id: 1, CreatedAt: 1000},                    // old, referenced by a snapshot: keep
		{Id: 2, CreatedAt: 1000},                    // old, unreferenced: orphan
		{Id: 3, CreatedAt: 99000},                   // fresh, unreferenced (Apply in progress): keep
		{Id: 4, InboundId: nodeIb, CreatedAt: 1000}, // old, node-mirrored inbound: keep (#195)
	}
	got := orphanGatewayHostIDs(hosts, map[int]bool{1: true}, map[int]bool{nodeIb: true}, 50000)
	if !reflect.DeepEqual(got, []int{2}) {
		t.Errorf("orphans = %v, want [2]", got)
	}
}
