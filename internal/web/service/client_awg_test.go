// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestAwgAllocationFallback(t *testing.T) {
	tests := []struct {
		serverAddr string
		want       string
	}{
		{"10.9.0.1/24", "10.9.0.0/24"},
		{"192.168.100.1/16", "192.168.0.0/16"},
		{"", defaultAwgBase},
		{"   ", defaultAwgBase},
		{"not-an-ip", defaultAwgBase},
		{"10.8.0.1", defaultAwgBase},
	}
	for _, tt := range tests {
		t.Run(tt.serverAddr, func(t *testing.T) {
			if got := awgAllocationFallback(tt.serverAddr); got != tt.want {
				t.Errorf("awgAllocationFallback(%q) = %q, want %q", tt.serverAddr, got, tt.want)
			}
		})
	}
}

func TestAwgSettingsAddress(t *testing.T) {
	if got := awgSettingsAddress(`{"address":"10.9.0.1/24","mtu":1320}`); got != "10.9.0.1/24" {
		t.Errorf("awgSettingsAddress = %q, want 10.9.0.1/24", got)
	}
	if got := awgSettingsAddress(`{"mtu":1320}`); got != "" {
		t.Errorf("missing address must yield empty, got %q", got)
	}
	if got := awgSettingsAddress(`{broken`); got != "" {
		t.Errorf("malformed JSON must yield empty, got %q", got)
	}
	if got := awgSettingsAddress(""); got != "" {
		t.Errorf("empty settings must yield empty, got %q", got)
	}
}

// TestParseAwgOutboundAddress covers the pure-function path of
// ActiveOutboundAddresses — parsing the tunnel Address from one AWG
// outbound settings JSON blob. This is the collision guard that keeps
// defaultAwgClients from handing a new client the same IP an awgo-N
// interface already owns.
func TestParseAwgOutboundAddress(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"with address", `{"address":"10.8.0.2/32","endpoint":"up:51820"}`, "10.8.0.2/32"},
		{"no address", `{"endpoint":"up:51820"}`, ""},
		{"empty address", `{"address":""}`, ""},
		{"whitespace address", `{"address":"  "}`, ""},
		{"malformed json", `{broken`, ""},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAwgOutboundAddress(tt.in); got != tt.want {
				t.Errorf("parseAwgOutboundAddress(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestMigrateAwgClientSubnets: client tunnel IPs stay put across Address
// edits (no re-export). Only a client that collides with the server's new
// host IP is re-allocated.
func TestMigrateAwgClientSubnets(t *testing.T) {
	cases := []struct {
		name     string
		oldAddr  string
		newAddr  string
		settings string
		check    func(t *testing.T, out string)
	}{
		{
			name:     "subnet change preserves client IPs",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.5.1/24",
			settings: `{"address":"10.8.5.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]},{"email":"b","allowedIPs":["10.8.0.3/32"]}]}`,
			check: func(t *testing.T, out string) {
				want := `{"address":"10.8.5.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]},{"email":"b","allowedIPs":["10.8.0.3/32"]}]}`
				if out != want {
					t.Errorf("client IPs must be preserved, got %q", out)
				}
			},
		},
		{
			name:     "collision with new server host reallocates that client only",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.0.2/24",
			settings: `{"address":"10.8.0.2/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]},{"email":"b","allowedIPs":["10.8.0.3/32"]}]}`,
			check: func(t *testing.T, out string) {
				var s struct {
					Clients []struct {
						Email      string   `json:"email"`
						AllowedIPs []string `json:"allowedIPs"`
					} `json:"clients"`
				}
				if err := json.Unmarshal([]byte(out), &s); err != nil {
					t.Fatalf("json: %v", err)
				}
				if len(s.Clients) != 2 {
					t.Fatalf("clients=%d", len(s.Clients))
				}
				if s.Clients[0].AllowedIPs[0] == "10.8.0.2/32" {
					t.Errorf("colliding client a must be reallocated, still %v", s.Clients[0].AllowedIPs)
				}
				if s.Clients[1].AllowedIPs[0] != "10.8.0.3/32" {
					t.Errorf("non-colliding client b must stay, got %v", s.Clients[1].AllowedIPs)
				}
			},
		},
		{
			name:     "same subnet different host is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.0.5/24",
			settings: `{"address":"10.8.0.5/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"10.8.0.5/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}` {
					t.Errorf("same-subnet change must be a no-op, got %q", out)
				}
			},
		},
		{
			name:     "address unchanged is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.0.1/24",
			settings: `{"address":"10.8.0.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"10.8.0.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}` {
					t.Errorf("unchanged address must be a no-op, got %q", out)
				}
			},
		},
		{
			name:     "custom allowedIPs left untouched",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.5.1/24",
			settings: `{"address":"10.8.5.1/24","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]},{"email":"b","allowedIPs":["0.0.0.0/0"]}]}`,
			check: func(t *testing.T, out string) {
				var s struct {
					Clients []struct {
						Email      string   `json:"email"`
						AllowedIPs []string `json:"allowedIPs"`
					} `json:"clients"`
				}
				_ = json.Unmarshal([]byte(out), &s)
				if s.Clients[1].AllowedIPs[0] != "0.0.0.0/0" {
					t.Errorf("custom 0.0.0.0/0 must be preserved, got %v", s.Clients[1].AllowedIPs)
				}
				if s.Clients[0].AllowedIPs[0] != "10.8.0.2/32" {
					t.Errorf("client a IP must be preserved, got %v", s.Clients[0].AllowedIPs)
				}
			},
		},
		{
			name:     "no clients is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "10.8.5.1/24",
			settings: `{"address":"10.8.5.1/24","clients":[]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"10.8.5.1/24","clients":[]}` {
					t.Errorf("no-clients must be a no-op, got %q", out)
				}
			},
		},
		{
			name:     "invalid new address is a no-op",
			oldAddr:  "10.8.0.1/24",
			newAddr:  "not-an-ip",
			settings: `{"address":"not-an-ip","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}`,
			check: func(t *testing.T, out string) {
				if out != `{"address":"not-an-ip","clients":[{"email":"a","allowedIPs":["10.8.0.2/32"]}]}` {
					t.Errorf("invalid address must be a no-op, got %q", out)
				}
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			c.check(t, migrateAwgClientSubnets(c.oldAddr, c.newAddr, c.settings))
		})
	}
}

// TestFillAwgClients covers the UpdateInbound path: the form resubmits every
// existing client with its AllowedIPs, so the collision check must exclude
// each client's own stored address (lucx.127 — Malderin "allowedIPs entry
// already used by another client" on a plain metadata save).
func TestFillAwgClients(t *testing.T) {
	base := awgAllocationFallback("10.200.0.1/24")
	t.Run("unchanged edit is not a self-collision", func(t *testing.T) {
		existing := []model.Client{
			{Email: "alice", PublicKey: "PUBA", PreSharedKey: "PSKA", AllowedIPs: []string{"10.200.0.2/32"}},
			{Email: "bob", PublicKey: "PUBB", PreSharedKey: "PSKB", AllowedIPs: []string{"10.200.0.3/32"}},
		}
		clients := []model.Client{
			{Email: "alice", PublicKey: "PUBA", PreSharedKey: "PSKA", AllowedIPs: []string{"10.200.0.2/32"}},
			{Email: "bob", PublicKey: "PUBB", PreSharedKey: "PSKB", AllowedIPs: []string{"10.200.0.3/32"}},
		}
		if err := fillAwgClients(existing, clients, nil, base, nil, "25"); err != nil {
			t.Fatalf("unchanged clients must save, got %v", err)
		}
	})
	t.Run("rename matched by pubkey keeps own IP", func(t *testing.T) {
		existing := []model.Client{
			{Email: "alice", PublicKey: "PUBA", AllowedIPs: []string{"10.200.0.2/32"}},
		}
		clients := []model.Client{
			{Email: "alice2", PublicKey: "PUBA", AllowedIPs: []string{"10.200.0.2/32"}},
		}
		if err := fillAwgClients(existing, clients, nil, base, nil, "25"); err != nil {
			t.Fatalf("renamed client must keep its IP, got %v", err)
		}
	})
	t.Run("duplicate across distinct clients still rejected", func(t *testing.T) {
		existing := []model.Client{
			{Email: "alice", PublicKey: "PUBA", AllowedIPs: []string{"10.200.0.2/32"}},
		}
		clients := []model.Client{
			{Email: "alice", PublicKey: "PUBA", AllowedIPs: []string{"10.200.0.2/32"}},
			{Email: "carol", PublicKey: "PUBC", AllowedIPs: []string{"10.200.0.2/32"}},
		}
		err := fillAwgClients(existing, clients, nil, base, nil, "25")
		if err == nil {
			t.Fatal("carol taking alice's IP must be rejected")
		}
		if !strings.Contains(err.Error(), "already used") {
			t.Fatalf("want already-used error, got %v", err)
		}
	})
	t.Run("blank client allocated off the inbound subnet", func(t *testing.T) {
		existing := []model.Client{
			{Email: "alice", PublicKey: "PUBA", AllowedIPs: []string{"10.200.0.2/32"}},
		}
		clients := []model.Client{
			{Email: "alice", PublicKey: "PUBA", AllowedIPs: []string{"10.200.0.2/32"}},
			{Email: "dave"},
		}
		if err := fillAwgClients(existing, clients, nil, base, nil, "25"); err != nil {
			t.Fatalf("blank client must allocate, got %v", err)
		}
		if len(clients[1].AllowedIPs) == 0 || clients[1].AllowedIPs[0] == "10.200.0.2/32" {
			t.Fatalf("dave must get a fresh IP, got %v", clients[1].AllowedIPs)
		}
		if clients[1].PublicKey == "" || clients[1].PrivateKey == "" {
			t.Fatalf("dave must get a keypair, got pub=%q priv=%q", clients[1].PublicKey, clients[1].PrivateKey)
		}
	})
	t.Run("new form client with keys gets PSK", func(t *testing.T) {
		clients := []model.Client{
			{Email: "new", PublicKey: "PUBNEW", PrivateKey: "PRIVNEW", AllowedIPs: []string{"10.200.0.9/32"}},
		}
		if err := fillAwgClients(nil, clients, nil, base, nil, "25"); err != nil {
			t.Fatal(err)
		}
		if clients[0].PreSharedKey == "" {
			t.Fatal("new AWG client must get a PSK")
		}
		if clients[0].PublicKey != "PUBNEW" {
			t.Fatalf("public key rotated: %q", clients[0].PublicKey)
		}
	})
	t.Run("existing client keeps empty PSK", func(t *testing.T) {
		existing := []model.Client{
			{Email: "imp", PublicKey: "PUBIMP", AllowedIPs: []string{"10.200.0.9/32"}},
		}
		clients := []model.Client{
			{Email: "imp", PublicKey: "PUBIMP", AllowedIPs: []string{"10.200.0.9/32"}},
		}
		if err := fillAwgClients(existing, clients, nil, base, nil, "25"); err != nil {
			t.Fatal(err)
		}
		if clients[0].PreSharedKey != "" {
			t.Fatalf("existing peer must not get a new PSK, got %q", clients[0].PreSharedKey)
		}
	})
	t.Run("empty keepalive defaults by version", func(t *testing.T) {
		v2 := []model.Client{{Email: "a", PublicKey: "P", PreSharedKey: "S", AllowedIPs: []string{"10.200.0.2/32"}}}
		if err := fillAwgClients(nil, v2, nil, base, nil, defaultAwgKeepAlive("2")); err != nil {
			t.Fatal(err)
		}
		if v2[0].KeepAlive.String() != "25" {
			t.Fatalf("v2 keepAlive = %q, want 25", v2[0].KeepAlive)
		}
		v3 := []model.Client{{Email: "b", PublicKey: "Q", PreSharedKey: "S", AllowedIPs: []string{"10.200.0.3/32"}}}
		if err := fillAwgClients(nil, v3, nil, base, nil, defaultAwgKeepAlive("3")); err != nil {
			t.Fatal(err)
		}
		if v3[0].KeepAlive.String() != "15-25" {
			t.Fatalf("v3 keepAlive = %q, want 15-25", v3[0].KeepAlive)
		}
		v31 := []model.Client{{Email: "c", PublicKey: "R", PreSharedKey: "S", AllowedIPs: []string{"10.200.0.4/32"}}}
		if err := fillAwgClients(nil, v31, nil, base, nil, defaultAwgKeepAlive("3.1")); err != nil {
			t.Fatal(err)
		}
		if v31[0].KeepAlive.String() != "15-25" {
			t.Fatalf("v3.1 keepAlive = %q, want 15-25", v31[0].KeepAlive)
		}
	})
}

// TestAwgAllowedIPsStale covers the lucx.92 detector that flags a client
// whose stored single-host address no longer belongs to the inbound's
// current subnet (detached-and-re-attached after a subnet change).
func TestAwgAllowedIPsStale(t *testing.T) {
	tests := []struct {
		name       string
		allowedIPs []string
		serverAddr string
		want       bool
	}{
		{"inside subnet", []string{"10.9.0.5/32"}, "10.9.0.1/24", false},
		{"outside subnet", []string{"10.8.0.5/32"}, "10.9.0.1/24", true},
		{"empty allowedIPs", nil, "10.9.0.1/24", false},
		{"custom 0.0.0.0/0 not stale", []string{"0.0.0.0/0"}, "10.9.0.1/24", false},
		{"mixed host + route not stale", []string{"10.8.0.5/32", "0.0.0.0/0"}, "10.9.0.1/24", false},
		{"unparseable not stale", []string{"junk"}, "10.9.0.1/24", false},
		{"bad server addr not stale", []string{"10.8.0.5/32"}, "not-an-ip", false},
		{"ipv6 outside", []string{"fd00::5/128"}, "fd01::1/64", true},
		{"ipv6 inside", []string{"fd01::5/128"}, "fd01::1/64", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := awgAllowedIPsStale(tt.allowedIPs, tt.serverAddr); got != tt.want {
				t.Errorf("awgAllowedIPsStale(%v, %q) = %v, want %v", tt.allowedIPs, tt.serverAddr, got, tt.want)
			}
		})
	}
}

func TestClearBroadcastTunnelIP(t *testing.T) {
	c := model.Client{AllowedIPs: []string{"10.200.0.7/32"}}
	clearBroadcastTunnelIP(&c, model.AWG, 1)
	if len(c.AllowedIPs) != 1 || c.AllowedIPs[0] != "10.200.0.7/32" {
		t.Fatalf("single AWG inbound must keep typed IP, got %v", c.AllowedIPs)
	}
	clearBroadcastTunnelIP(&c, model.AWG, 2)
	if c.AllowedIPs != nil {
		t.Fatalf("multi AWG attach must clear IP, got %v", c.AllowedIPs)
	}
	c.AllowedIPs = []string{"10.200.0.7/32"}
	clearBroadcastTunnelIP(&c, model.VLESS, 2)
	if len(c.AllowedIPs) != 1 {
		t.Fatalf("non-tunnel proto must keep IP, got %v", c.AllowedIPs)
	}
}

// A control character embedded in a value rendered into a .conf must be
// rejected before save, not reach a file awg-quick runs as root via PostUp.
func TestValidateAwgSettingsJSON_RejectsControlCharacters(t *testing.T) {
	base := map[string]any{
		"awgVersion": "1", "h1": "1", "h2": "2", "h3": "3", "h4": "4",
		"jc": 1, "jmin": 1, "jmax": 3, "s1": 20, "s2": 20, "s3": 20, "s4": 20,
		"address": "10.200.0.1/24", "dns": "1.1.1.1, 1.0.0.1",
	}
	cases := []struct {
		name  string
		field string
		value string
	}{
		{"i1 with newline", "i1", "abc\ndef"},
		{"address with CR", "address", "10.200.0.1\r/24"},
		{"dns with NUL", "dns", "1.1.1.1\x001.0.0.1"},
		{"headerProtectionKey with DEL", "headerProtectionKey", "MCPfR\x7fGcDG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := make(map[string]any, len(base)+1)
			for k, v := range base {
				settings[k] = v
			}
			settings[tc.field] = tc.value
			raw, err := json.Marshal(settings)
			if err != nil {
				t.Fatalf("marshal settings: %v", err)
			}
			err = validateAwgSettingsJSON(string(raw))
			if !errors.Is(err, errAwgControlChar) {
				t.Fatalf("validateAwgSettingsJSON(%s=%q) = %v, want errAwgControlChar", tc.field, tc.value, err)
			}
		})
	}
	// Sanity: the same shape with no control characters must still pass, so
	// the new check does not overreject legitimate values.
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("marshal base settings: %v", err)
	}
	if err := validateAwgSettingsJSON(string(raw)); err != nil {
		t.Fatalf("validateAwgSettingsJSON(clean settings) = %v, want nil", err)
	}
}

// Broken settings JSON must fail closed, not silently disable every other
// check (H grammar, I budget, AWGParams invariants) that follows it.
func TestValidateAwgSettingsJSON_BrokenJSON(t *testing.T) {
	t.Run("malformed JSON is rejected", func(t *testing.T) {
		err := validateAwgSettingsJSON("{")
		if !errors.Is(err, errAwgSettingsMalformed) {
			t.Fatalf(`validateAwgSettingsJSON("{") = %v, want errAwgSettingsMalformed`, err)
		}
	})
	t.Run("empty settings is a legal absence of obfuscation", func(t *testing.T) {
		if err := validateAwgSettingsJSON(""); err != nil {
			t.Fatalf(`validateAwgSettingsJSON("") = %v, want nil`, err)
		}
	})
	t.Run("empty object is a legal absence of obfuscation", func(t *testing.T) {
		if err := validateAwgSettingsJSON("{}"); err != nil {
			t.Fatalf(`validateAwgSettingsJSON("{}") = %v, want nil`, err)
		}
	})
	t.Run("a genuine validation error is not mistaken for broken JSON", func(t *testing.T) {
		settings := `{"jc":5,"s1":20,"h1":"","h2":"2","h3":"3","h4":"4"}`
		err := validateAwgSettingsJSON(settings)
		if errors.Is(err, errAwgSettingsMalformed) {
			t.Fatalf("validateAwgSettingsJSON(%s) = %v, wrongly classified as malformed JSON", settings, err)
		}
		if !errors.Is(err, awg.ErrEmptyObfuscationHeader) {
			t.Fatalf("validateAwgSettingsJSON(%s) = %v, want awg.ErrEmptyObfuscationHeader", settings, err)
		}
	})
}

// amneziawg-tools bound-check against UINT32_MAX and silently truncate
// (RekeyTimeout=70000 becomes 4464), so this must be caught before save.
func TestValidateAwgSettingsJSON_RejectsOversizedTimer(t *testing.T) {
	settings := `{"awgVersion":"3","jc":4,"jmin":1,"jmax":3,"s1":20,"s2":20,"s3":20,"s4":20,` +
		`"h1":"1","h2":"2","h3":"3","h4":"4","rekeyTimeout":"70000"}`
	err := validateAwgSettingsJSON(settings)
	if !errors.Is(err, awg.ErrTimerOutOfRange) {
		t.Fatalf("validateAwgSettingsJSON(%s) = %v, want awg.ErrTimerOutOfRange", settings, err)
	}
}

// A legal single-value timer and a legal lo-hi range must both still pass, so
// the new check does not degenerate into "no device timers allowed".
func TestValidateAwgSettingsJSON_AcceptsLegalTimers(t *testing.T) {
	base := `{"awgVersion":"3","jc":4,"jmin":1,"jmax":3,"s1":20,"s2":20,"s3":20,"s4":20,` +
		`"h1":"1","h2":"2","h3":"3","h4":"4"`
	tests := []struct {
		name string
		json string
	}{
		{"single value", base + `,"rekeyTimeout":"150"}`},
		{"lo-hi range", base + `,"rekeyTimeout":"100-500"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAwgSettingsJSON(tt.json); err != nil {
				t.Fatalf("validateAwgSettingsJSON(%s) = %v, want nil", tt.json, err)
			}
		})
	}
}

// mtuLineFromConf extracts the MTU value written into a client .conf's
// [Interface] section, e.g. "MTU = 1420".
func mtuLineFromConf(t *testing.T, conf string) int {
	t.Helper()
	for _, line := range strings.Split(conf, "\n") {
		if v, ok := strings.CutPrefix(line, "MTU = "); ok {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				t.Fatalf("MTU line %q: %v", line, err)
			}
			return n
		}
	}
	t.Fatalf("conf has no MTU line:\n%s", conf)
	return 0
}

// TestBuildAwgClientConf_DefaultMTUMatchesInstance guards D5: an inbound
// missing mtu must give the client .conf and server Instance the same value.
func TestBuildAwgClientConf_DefaultMTUMatchesInstance(t *testing.T) {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	cpriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("client keypair: %v", err)
	}
	client := &model.Client{PrivateKey: cpriv, AllowedIPs: []string{"10.200.0.2/32"}, Enable: true}

	t.Run("mtu omitted", func(t *testing.T) {
		settings, _ := json.Marshal(map[string]any{
			"privateKey": priv,
			"publicKey":  pub,
			"dns":        "1.1.1.1",
		})
		inbound := &model.Inbound{Id: 1, Protocol: model.AWG, Port: 51820, Settings: string(settings)}

		inst, ok := awg.InstanceFromInbound(inbound)
		if !ok {
			t.Fatalf("InstanceFromInbound: not ok")
		}
		conf, err := BuildAwgClientConf(inbound, client, "203.0.113.9")
		if err != nil {
			t.Fatalf("BuildAwgClientConf: %v", err)
		}
		clientMTU := mtuLineFromConf(t, conf)

		if clientMTU != inst.MTU {
			t.Fatalf("client conf MTU = %d, server Instance MTU = %d; must agree when mtu is omitted", clientMTU, inst.MTU)
		}
		if inst.MTU != awg.DefaultMTU {
			t.Fatalf("server Instance MTU = %d, want awg.DefaultMTU = %d", inst.MTU, awg.DefaultMTU)
		}
	})

	t.Run("explicit mtu is not overridden", func(t *testing.T) {
		settings, _ := json.Marshal(map[string]any{
			"privateKey": priv,
			"publicKey":  pub,
			"dns":        "1.1.1.1",
			"mtu":        1280,
		})
		inbound := &model.Inbound{Id: 2, Protocol: model.AWG, Port: 51820, Settings: string(settings)}

		inst, ok := awg.InstanceFromInbound(inbound)
		if !ok {
			t.Fatalf("InstanceFromInbound: not ok")
		}
		if inst.MTU != 1280 {
			t.Fatalf("server Instance MTU = %d, want explicit 1280", inst.MTU)
		}
		conf, err := BuildAwgClientConf(inbound, client, "203.0.113.9")
		if err != nil {
			t.Fatalf("BuildAwgClientConf: %v", err)
		}
		if got := mtuLineFromConf(t, conf); got != 1280 {
			t.Fatalf("client conf MTU = %d, want explicit 1280", got)
		}
	})
}

func TestCountAwgOrWireguard(t *testing.T) {
	got := countAwgOrWireguard([]*model.Inbound{
		{Protocol: model.AWG},
		{Protocol: model.VLESS},
		{Protocol: model.WireGuard},
		nil,
	})
	if got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
}

// The check ran on a trimmed copy while the renderers wrote the raw value, so
// a leading newline smuggled a second directive past it — PreUp runs as root.
func TestValidateAwgSettingsJSON_RejectsLeadingNewline(t *testing.T) {
	base := map[string]any{
		"awgVersion": "2", "h1": "1", "h2": "2", "h3": "3", "h4": "4",
		"jc": 1, "jmin": 1, "jmax": 3, "s1": 20, "s2": 20, "s3": 20, "s4": 20,
		"address": "10.200.0.1/24", "dns": "1.1.1.1, 1.0.0.1",
	}
	cases := []struct {
		name  string
		field string
		value string
	}{
		{"address opens a PreUp line", "address", "\nPreUp = touch /tmp/pwned"},
		{"i1 opens a PostUp line", "i1", "\nPostUp = touch /tmp/pwned"},
		{"headerProtectionKey opens a PostDown line", "headerProtectionKey", "\nPostDown = touch /tmp/pwned"},
		{"trailing newline is written too", "address", "10.200.0.1/24\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := make(map[string]any, len(base)+1)
			for k, v := range base {
				settings[k] = v
			}
			settings[tc.field] = tc.value
			raw, err := json.Marshal(settings)
			if err != nil {
				t.Fatalf("marshal settings: %v", err)
			}
			err = validateAwgSettingsJSON(string(raw))
			if !errors.Is(err, errAwgControlChar) {
				t.Fatalf("validateAwgSettingsJSON(%s=%q) = %v, want errAwgControlChar", tc.field, tc.value, err)
			}
		})
	}
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("marshal base settings: %v", err)
	}
	if err := validateAwgSettingsJSON(string(raw)); err != nil {
		t.Fatalf("validateAwgSettingsJSON(clean settings) = %v, want nil", err)
	}
}

// Nothing checked the key was a key, so a bogus one reached the .conf and the
// awg tools rejected the whole file — reason unnamed, tunnel already down.
func TestValidateAwgSettingsJSON_HeaderProtectionKeyFormat(t *testing.T) {
	// 44 base64 chars = 32 bytes, the only shape the AWG3 cipher accepts.
	const validKey = "MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18="
	base := map[string]any{
		"awgVersion": "3", "h1": "1", "h2": "2", "h3": "3", "h4": "4",
		"jc": 4, "jmin": 10, "jmax": 50, "s1": 20, "s2": 30, "s3": 15, "s4": 13,
		"address": "10.200.0.1/24", "dns": "1.1.1.1, 1.0.0.1",
	}
	for _, tc := range []struct {
		name     string
		key      string
		override map[string]any
		wantErr  error
	}{
		{"blank key is header protection switched off", "", nil, nil},
		{"base64 of 32 bytes saves", validKey, nil, nil},
		{"a word is not a key", "привет", nil, errAwgHeaderProtectionKey},
		{"base64 of 30 bytes is the wrong size", "MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivX", nil, errAwgHeaderProtectionKey},
		// Pins the check order: the decoder ignores \r\n, so a mail-wrapped key
		// decodes to 32 bytes and only the control-character check ever sees it.
		{"a line-wrapped key is caught before it is decoded", validKey[:16] + "\r\n" + validKey[16:], nil, errAwgControlChar},
		{"a key of blanks is not an absent key", "   ", nil, errAwgHeaderProtectionKey},
		// Jc 0 / S1 0 is a working mode here and still renders the key, so the
		// check has to sit ahead of the "no obfuscation, nothing to check" return.
		{"an unobfuscated inbound still renders the key", "привет", map[string]any{"jc": 0, "s1": 0}, errAwgHeaderProtectionKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := make(map[string]any, len(base)+1)
			for k, v := range base {
				settings[k] = v
			}
			for k, v := range tc.override {
				settings[k] = v
			}
			settings["headerProtectionKey"] = tc.key
			raw, err := json.Marshal(settings)
			if err != nil {
				t.Fatalf("marshal settings: %v", err)
			}
			got := validateAwgSettingsJSON(string(raw))
			if tc.wantErr == nil {
				if got != nil {
					t.Fatalf("validateAwgSettingsJSON(headerProtectionKey=%q) = %v, want nil", tc.key, got)
				}
				return
			}
			if !errors.Is(got, tc.wantErr) {
				t.Fatalf("validateAwgSettingsJSON(headerProtectionKey=%q) = %v, want %v", tc.key, got, tc.wantErr)
			}
		})
	}
}
