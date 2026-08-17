// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeProber replays recorded command outputs keyed by the joined command line.
type fakeProber struct {
	outputs map[string]string
	failing map[string]bool
}

func (f fakeProber) Run(name string, args ...string) (string, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	if f.failing[key] {
		return f.outputs[key], errors.New("exit 1")
	}
	return f.outputs[key], nil
}

func fixedNow() time.Time { return time.Unix(1_800_000_000, 0) }

func natInstance() Instance {
	return Instance{
		Id: 1, Ifname: "awg1", Port: 47000, PrivateKey: "k", MTU: 1320,
		Address: "10.8.0.1/24", RouteThroughXray: false,
	}
}

func healthyKernelProber() fakeProber {
	return fakeProber{
		outputs: map[string]string{
			"ip link show awg1":               "3: awg1: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1320 state UP mode DEFAULT",
			"sysctl -n net.ipv4.ip_forward":   "1\n",
			"awg show awg1 peers":             "pubkeyA\npubkeyB\n",
			"awg show awg1 latest-handshakes": "pubkeyA\t1799999950\npubkeyB\t0\n",
			"ip -o -4 route show default":     "default via 192.168.1.1 dev eth0 proto static\n",
			"awg version":                     "amneziawg-tools v3.0.20260730 - https://amnezia.org\n",
			"iptables -t mangle -C PREROUTING -i awg1 -j MARK --set-mark 655361":         "",
			"iptables -t nat -C POSTROUTING -m mark --mark 655361 -o eth0 -j MASQUERADE": "",
			"iptables -C FORWARD -i awg1 -j ACCEPT":                                      "",
			"iptables -C FORWARD -o awg1 -j ACCEPT":                                      "",
		},
		failing: map[string]bool{},
	}
}

func TestDiagnose_KernelNATHealthy(t *testing.T) {
	d := diagnose(natInstance(), healthyKernelProber(), fixedNow)
	if d.Mode != "kernel-nat" {
		t.Errorf("Mode = %q, want kernel-nat", d.Mode)
	}
	if !d.Healthy() {
		for _, c := range d.Checks {
			if !c.OK {
				t.Errorf("check %q failed: %s", c.Name, c.Detail)
			}
		}
	}
	if len(d.Checks) != 7 {
		t.Errorf("kernel-nat must run 7 checks, got %d: %+v", len(d.Checks), d.Checks)
	}
	var peers DiagCheck
	for _, c := range d.Checks {
		if c.Name == "wireguard peers" {
			peers = c
		}
	}
	if !strings.Contains(peers.Detail, "2 peer(s)") || !strings.Contains(peers.Detail, "1 with handshake") {
		t.Errorf("peers detail must count configured and live, got %q", peers.Detail)
	}
	if !strings.Contains(peers.Detail, "50s ago") {
		t.Errorf("peers detail must carry latest handshake age, got %q", peers.Detail)
	}
}

func TestDiagnose_MissingInterfaceShortCircuits(t *testing.T) {
	p := healthyKernelProber()
	p.failing["ip link show awg1"] = true
	p.outputs["ip link show awg1"] = "Device \"awg1\" does not exist.\n"
	d := diagnose(natInstance(), p, fixedNow)
	if len(d.Checks) != 1 || d.Checks[0].OK {
		t.Errorf("missing interface must stop the probe at 1 failed check, got %+v", d.Checks)
	}
	if d.Healthy() {
		t.Error("missing interface cannot be healthy")
	}
}

func TestDiagnose_KernelNATFlushedRules(t *testing.T) {
	p := healthyKernelProber()
	p.failing["iptables -t mangle -C PREROUTING -i awg1 -j MARK --set-mark 655361"] = true
	p.failing["iptables -t nat -C POSTROUTING -m mark --mark 655361 -o eth0 -j MASQUERADE"] = true
	p.failing["iptables -C FORWARD -o awg1 -j ACCEPT"] = true
	d := diagnose(natInstance(), p, fixedNow)
	var masq, fwd DiagCheck
	for _, c := range d.Checks {
		switch c.Name {
		case "masquerade":
			masq = c
		case "forward chain":
			fwd = c
		}
	}
	if masq.OK || !strings.Contains(masq.Detail, "-o eth0") || !strings.Contains(masq.Detail, "MARK") {
		t.Errorf("masquerade check must fail and name mark+iface rule, got %+v", masq)
	}
	if fwd.OK || !strings.Contains(fwd.Detail, "-o awg1") {
		t.Errorf("forward check must fail and name only the missing leg, got %+v", fwd)
	}
	if d.Healthy() {
		t.Error("flushed iptables cannot be healthy")
	}
}

func TestDiagnose_XrayTunHealthy(t *testing.T) {
	inst := natInstance()
	inst.RouteThroughXray = true
	p := fakeProber{
		outputs: map[string]string{
			"ip link show awg1":               "3: awg1: <UP,LOWER_UP> mtu 1320 state UP",
			"sysctl -n net.ipv4.ip_forward":   "1\n",
			"awg show awg1 peers":             "pubkeyA\n",
			"awg show awg1 latest-handshakes": "pubkeyA\t1799999999\n",
			"ip link show tun1":               "7: tun1: <POINTOPOINT,UP,LOWER_UP> mtu 9000 state UNKNOWN",
			"ip rule show iif awg1":           "32000: from all iif awg1 lookup 1001\n",
			"ip route show table 1001":        "default dev tun1 scope link\n",
		},
		failing: map[string]bool{},
	}
	d := diagnose(inst, p, fixedNow)
	if d.Mode != "xray-tun" {
		t.Errorf("Mode = %q, want xray-tun", d.Mode)
	}
	if !d.Healthy() {
		for _, c := range d.Checks {
			if !c.OK {
				t.Errorf("check %q failed: %s", c.Name, c.Detail)
			}
		}
	}
	var names []string
	for _, c := range d.Checks {
		names = append(names, c.Name)
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"xray tun tun1", "policy rule", "route table 1001"} {
		if !strings.Contains(joined, want) {
			t.Errorf("xray-tun must probe %q, got %q", want, joined)
		}
	}
}

func TestDiagnose_XrayTunRouteLost(t *testing.T) {
	inst := natInstance()
	inst.RouteThroughXray = true
	p := fakeProber{
		outputs: map[string]string{
			"ip link show awg1":               "3: awg1: <UP,LOWER_UP> mtu 1320 state UP",
			"sysctl -n net.ipv4.ip_forward":   "1\n",
			"awg show awg1 peers":             "pubkeyA\n",
			"awg show awg1 latest-handshakes": "pubkeyA\t1799999999\n",
			"ip link show tun1":               "7: tun1: <UP,LOWER_UP> mtu 9000 state UNKNOWN",
			"ip rule show iif awg1":           "32000: from all iif awg1 lookup 1001\n",
			"ip route show table 1001":        "",
		},
		failing: map[string]bool{},
	}
	d := diagnose(inst, p, fixedNow)
	var route DiagCheck
	for _, c := range d.Checks {
		if strings.HasPrefix(c.Name, "route table") {
			route = c
		}
	}
	if route.OK || !strings.Contains(route.Detail, "reconcile re-adds") {
		t.Errorf("lost default route must fail and point at reconcile recovery, got %+v", route)
	}
}

func TestParseDefaultRouteInterface(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"default via 192.168.1.1 dev eth0 proto static\n", "eth0"},
		{"default via 10.0.0.1 dev ens3\n", "ens3"},
		{"", ""},
		{"default dev wg0 scope link\n", "wg0"},
	}
	for _, tt := range tests {
		if got := parseDefaultRouteInterface(tt.in); got != tt.want {
			t.Errorf("parseDefaultRouteInterface(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseLatestHandshakes(t *testing.T) {
	live, age := parseLatestHandshakes("pubA\t1799999900\npubB\t0\npubC\t1799999800\n", fixedNow)
	if live != 2 || age != 100 {
		t.Errorf("live=%d age=%d, want 2 and 100 (most recent of the two)", live, age)
	}
	live, age = parseLatestHandshakes("pubA\t0\n", fixedNow)
	if live != 0 || age != -1 {
		t.Errorf("no handshakes: live=%d age=%d, want 0 and -1", live, age)
	}
	live, age = parseLatestHandshakes("", fixedNow)
	if live != 0 || age != -1 {
		t.Errorf("empty: live=%d age=%d, want 0 and -1", live, age)
	}
}
