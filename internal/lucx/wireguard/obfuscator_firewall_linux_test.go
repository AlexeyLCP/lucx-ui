// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package wireguard

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var errProbe = errors.New("rule absent")

func TestWgoDropArgs(t *testing.T) {
	st := WgoFirewallState{ID: 3, WGPort: 51820, Block: true}
	insert := wgoDropArgs(st, "-I")
	want := []string{
		"iptables", "-w", "5", "-I", "INPUT", "1", "-p", "udp",
		"--dport", "51820", "!", "-i", "lo", "-m", "comment", "--comment", "lucx-wgo-3", "-j", "DROP",
	}
	if !reflect.DeepEqual(insert, want) {
		t.Fatalf("insert = %v", insert)
	}
	check := wgoDropArgs(st, "-C")
	if check[3] != "-C" || check[4] != "INPUT" {
		t.Fatalf("check = %v", check)
	}
	del := wgoDropArgs(st, "-D")
	if del[3] != "-D" || del[4] != "INPUT" || del[5] != "-p" {
		t.Fatalf("delete = %v", del)
	}
}

func TestParseWgoRules(t *testing.T) {
	dump := `-P INPUT ACCEPT
-A INPUT -p udp --dport 51820 ! -i lo -m comment --comment lucx-wgo-3 -j DROP
-A INPUT -p udp --dport 51821 ! -i lo -m comment --comment "lucx-wgo-7" -j DROP
-A INPUT -s 10.0.0.1/32 -j ACCEPT
`
	rules := parseWgoRules(dump)
	if len(rules) != 2 {
		t.Fatalf("rules = %v", rules)
	}
	if rules[3] != `-p udp --dport 51820 ! -i lo -m comment --comment lucx-wgo-3 -j DROP` {
		t.Fatalf("spec(3) = %q", rules[3])
	}
	if rules[7] != `-p udp --dport 51821 ! -i lo -m comment --comment "lucx-wgo-7" -j DROP` {
		t.Fatalf("spec(7) = %q", rules[7])
	}
	del := wgoDeleteArgs(rules[7])
	if del[len(del)-4] != "--comment" || del[len(del)-3] != `lucx-wgo-7` {
		t.Fatalf("delete argv comment = %v", del)
	}
	if !strings.Contains(strings.Join(del, " "), "-D INPUT -p udp") {
		t.Fatalf("delete argv = %v", del)
	}
}

func TestSyncWgoFirewallSweepsStale(t *testing.T) {
	var calls [][]string
	restore := wgoIptables
	wgoIptables = func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, args)
		if strings.HasPrefix(joined, "iptables -w 5 -S INPUT") {
			return []byte("-A INPUT -p udp --dport 51820 ! -i lo -m comment --comment lucx-wgo-3 -j DROP\n" +
				"-A INPUT -p udp --dport 51821 ! -i lo -m comment --comment lucx-wgo-9 -j DROP\n"), nil
		}
		// The rule of active inbound 3 already exists: its -C succeeds, every
		// other -C fails (rule absent).
		if strings.HasPrefix(joined, "iptables -w 5 -C") {
			if strings.Contains(joined, "lucx-wgo-3") {
				return nil, nil
			}
			return nil, errProbe
		}
		return nil, nil
	}
	defer func() { wgoIptables = restore }()

	SyncWgoFirewall([]WgoFirewallState{{ID: 3, WGPort: 51820, Block: true}})

	sawDelete := false
	for _, c := range calls {
		joined := strings.Join(c, " ")
		if c[3] == "-D" && strings.Contains(joined, "lucx-wgo-9") {
			sawDelete = true
		}
		if c[3] == "-I" && strings.Contains(joined, "lucx-wgo-3") {
			t.Fatal("existing rule must not be re-inserted")
		}
	}
	if !sawDelete {
		t.Fatalf("stale rule of inbound 9 not swept: %v", calls)
	}
}
