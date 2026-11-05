// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strings"
	"testing"
)

func TestProcAddr(t *testing.T) {
	cases := map[string]string{
		"0100007F:01BB":                         "127.0.0.1:443",
		"C77100CB:01BB":                         "203.0.113.199:443",
		"0000000000000000FFFF0000C77100CB:20FB": "203.0.113.199:8443",
		"00000000000000000000000001000000:0050": "[::1]:80",
	}
	for in, want := range cases {
		got, err := procAddr(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestParseProcNetTCPEstablishedOnly(t *testing.T) {
	table := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: C77100CB:01BB 0A0B0C0D:D431 01 00000000:00000000 00:00000000 00000000     0        0 1 1 0000000000000000 20 4 30 10 -1
   1: C77100CB:01BB 0A0B0C0D:D432 01 00000000:00000000 00:00000000 00000000     0        0 2 1 0000000000000000 20 4 30 10 -1
   2: C37100CB:01BB 0A0B0C0D:D433 01 00000000:00000000 00:00000000 00000000     0        0 3 1 0000000000000000 20 4 30 10 -1
   3: C77100CB:01BB 0A0B0C0E:D434 0A 00000000:00000000 00:00000000 00000000     0        0 4 1 0000000000000000 20 4 30 10 -1
`
	out := map[string]map[string]int{}
	parseProcNetTCP(strings.NewReader(table), out)
	client := out["13.12.11.10"]
	if client["203.0.113.199:443"] != 2 || client["203.0.113.195:443"] != 1 {
		t.Errorf("unexpected sockets for client: %v", client)
	}
	if _, ok := out["14.12.11.10"]; ok {
		t.Error("LISTEN socket (state 0A) must be ignored")
	}
}

func TestEndpointMapLookup(t *testing.T) {
	m := endpointMap{"203.0.113.199:443": "tcp-raw", "*:8443": "wild"}
	if m.lookup("203.0.113.199:443") != "tcp-raw" {
		t.Error("specific listen not resolved")
	}
	if m.lookup("203.0.113.195:8443") != "wild" {
		t.Error("wildcard listen not resolved")
	}
	if m.lookup("203.0.113.196:443") != "" {
		t.Error("unknown endpoint must not resolve")
	}
}

func TestRecordLiveInboundKeepsLatestWeight(t *testing.T) {
	RecordLiveInbound("a@x", "t1", 5, 1000)
	RecordLiveInbound("a@x", "t1", 9, 2000)
	RecordLiveInbound(" ", "t1", 1, 2000)
	liveInbounds.Lock()
	e := liveInbounds.byEmail["a@x"]["t1"]
	_, blank := liveInbounds.byEmail[""]
	delete(liveInbounds.byEmail, "a@x")
	liveInbounds.Unlock()
	if e.weight != 9 || e.lastSeen != 2000 {
		t.Errorf("entry = %+v", e)
	}
	if blank {
		t.Error("blank email must be ignored")
	}
}

func TestAttributeSidecarIPsOnlyWithSingleUser(t *testing.T) {
	sockets := map[string]map[string]int{
		"5.5.5.5": {"203.0.113.195:8443": 2},
		"6.6.6.6": {"203.0.113.194:443": 1},
	}
	eps := endpointMap{"203.0.113.195:8443": "mieru", "203.0.113.194:443": "trust"}
	got := attributeSidecarIPs(sockets, eps, map[string][]string{
		"mieru": {"alice"},
		"trust": {"bob", "carol"},
	})
	if got["5.5.5.5"] != (sidecarIP{email: "alice", tag: "mieru"}) {
		t.Errorf("single live user on mieru: got %+v", got["5.5.5.5"])
	}
	if _, ok := got["6.6.6.6"]; ok {
		t.Error("two live users on trust: the IP must stay unattributed")
	}
}

func TestRecordLiveIPSkipsInternalAndExpires(t *testing.T) {
	RecordLiveIP("ip@x", "127.0.0.1", "t", 1000)
	RecordLiveIP("ip@x", "::ffff:5.5.5.5", "t", 1000)
	RecordLiveIP("ip@x", "6.6.6.6", "ws", 100000)
	RecordLiveIP("ip@x", "6.6.6.6", "", 100000)
	snap := liveIPsSnapshot(100000)
	ips := map[string]bool{}
	for _, ip := range snap["ip@x"] {
		ips[ip.IP] = true
		if ip.IP == "6.6.6.6" && (len(ip.tags) != 1 || ip.tags[0] != "ws") {
			t.Errorf("inbound per IP lost: %v", ip.tags)
		}
	}
	if ips["127.0.0.1"] {
		t.Error("loopback must be ignored")
	}
	if ips["5.5.5.5"] {
		t.Error("an IP older than the grace window must expire")
	}
	if !ips["6.6.6.6"] {
		t.Error("a fresh IP must be listed")
	}
	liveIPs.Lock()
	delete(liveIPs.byEmail, "ip@x")
	liveIPs.Unlock()
}
