// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telemt

import (
	"strings"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	data := ConfigData{
		ID: 0, Port: 443, PublicHost: "5.9.1.2",
		SocksPort: 31427, SocksPassword: "abc123def456", APIPort: 9090,
		TLSDomain: "gosuslugi.ru", MaxConnections: 10000,
		Clients: []TelemtClient{{Name: "myphone", Secret: "ee00000000000000000000000000000000"}},
	}
	toml, err := GenerateConfig(data)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}
	checks := []string{
		`[general]`, `[general.modes]`, `tls = true`,
		`public_host = "5.9.1.2"`, `port = 443`,
		`[[upstreams]]`, `type = "socks5"`, `address = "127.0.0.1:31427"`,
		`username = "telemt"`, `password = "abc123def456"`,
		`tls_domain = "gosuslugi.ru"`, `tls_emulation = true`,
		`myphone = "ee00000000000000000000000000000000"`,
	}
	for _, check := range checks {
		if !strings.Contains(toml, check) {
			t.Errorf("TOML missing: %s\nGot:\n%s", check, toml)
		}
	}
}

func TestGenerateConfig_MultipleClients(t *testing.T) {
	data := ConfigData{
		ID: 1, Port: 443, PublicHost: "1.2.3.4",
		SocksPort: 12345, SocksPassword: "pw", APIPort: 9091,
		TLSDomain: "cloudflare.com", MaxConnections: 5000,
		Clients: []TelemtClient{
			{Name: "phone", Secret: "eeaaaa0000000000000000000000000000"},
			{Name: "tablet", Secret: "eebbbb0000000000000000000000000000"},
		},
	}
	toml, _ := GenerateConfig(data)
	if !strings.Contains(toml, "phone =") || !strings.Contains(toml, "tablet =") {
		t.Error("both clients should be in the config")
	}
}

func TestGenerateConfig_NoClients(t *testing.T) {
	data := ConfigData{
		ID: 0, Port: 443, PublicHost: "x.x.x.x",
		SocksPort: 1, SocksPassword: "p", APIPort: 9090,
		TLSDomain: "test.ru", MaxConnections: 1000,
	}
	toml, _ := GenerateConfig(data)
	if strings.Contains(toml, "[access.users]") {
		t.Error("access.users should be omitted when no clients")
	}
}

func TestTelemtConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		data    ConfigData
		checks  []string
		wantErr bool
	}{
		{
			name: "faketls_update_microsoft",
			data: ConfigData{
				ID: 0, Port: 443, PublicHost: "5.9.1.2",
				SocksPort: 31427, SocksPassword: "abc123",
				APIPort: 9090, TLSDomain: "update.microsoft.com",
				MaxConnections: 10000,
				Clients: []TelemtClient{
					{Name: "phone", Secret: "ee00000000000000000000000000000000"},
				},
			},
			checks: []string{
				"tls_domain = \"update.microsoft.com\"",
				"phone = \"ee00000000000000000000000000000000\"",
				"tls = true", "[[upstreams]]",
			},
		},
		{
			name: "stealth_ubuntu_releases",
			data: ConfigData{
				ID: 1, Port: 8443, PublicHost: "10.0.0.1",
				SocksPort: 40000, SocksPassword: "stealth", APIPort: 9091,
				TLSDomain: "releases.ubuntu.com", MaxConnections: 5000,
				Clients: []TelemtClient{
					{Name: "tablet", Secret: "eeffffffffffffffffffffffffffffffffffff"},
				},
			},
			checks: []string{
				"port = 8443",
				"tls_domain = \"releases.ubuntu.com\"",
				"max_connections = 5000",
			},
		},
		{
			name: "no_clients_valid",
			data: ConfigData{
				ID: 2, Port: 443, PublicHost: "x.x.x.x",
				SocksPort: 1, SocksPassword: "x", APIPort: 9092,
				TLSDomain: "update.microsoft.com", MaxConnections: 1000,
			},
			checks: []string{
				"[general]", "[server]", "[censorship]", "[[upstreams]]",
			},
		},
		{
			name: "multi_client",
			data: ConfigData{
				ID: 3, Port: 443, PublicHost: "host",
				SocksPort: 9999, SocksPassword: "pw", APIPort: 9093,
				TLSDomain: "releases.ubuntu.com", MaxConnections: 1000,
				Clients: []TelemtClient{
					{Name: "a", Secret: "eeaa000000000000000000000000000000"},
					{Name: "b", Secret: "eebb000000000000000000000000000000"},
					{Name: "c", Secret: "eecc000000000000000000000000000000"},
				},
			},
			checks: []string{"a = ", "b = ", "c = ", "[access.users]"},
		},
		{
			name: "all_sections_present",
			data: ConfigData{
				ID: 4, Port: 443, PublicHost: "1.1.1.1",
				SocksPort: 1, SocksPassword: "p", APIPort: 0,
				TLSDomain: "example.com", MaxConnections: 100,
			},
			checks: []string{
				"[general]", "[general.modes]", "[general.links]",
				"[server]", "[server.api]", "[censorship]", "[[upstreams]]",
				"tls_emulation = true", "mask = true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toml, err := GenerateConfig(tt.data)
			if tt.wantErr && err == nil {
				t.Errorf("[%s] expected error", tt.name)
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("[%s] unexpected error: %v", tt.name, err)
				return
			}
			for _, check := range tt.checks {
				if !strings.Contains(toml, check) {
					t.Errorf("[%s] missing: %s\nTOML:\n%s", tt.name, check, toml)
				}
			}
		})
	}
}
