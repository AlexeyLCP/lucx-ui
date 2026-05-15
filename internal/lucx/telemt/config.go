// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telemt

import (
	"bytes"
	"fmt"
	"text/template"
)

type ConfigData struct {
	ID             int
	Port           int
	PublicHost     string
	SocksPort      int
	SocksPassword  string
	APIPort        int
	TLSDomain      string
	MaxConnections int
	Clients        []TelemtClient
}

const telemtTOMLTemplate = `[general]
use_middle_proxy = true
fast_mode = true
log_level = "normal"
data_path = "/var/lib/telemt/telemt-{{.ID}}"

[general.modes]
classic = false
secure = false
tls = true

[general.links]
show = "*"
public_host = "{{.PublicHost}}"
public_port = {{.Port}}

[server]
port = {{.Port}}
listen_addr_ipv4 = "0.0.0.0"
max_connections = {{.MaxConnections}}

[server.api]
enabled = true
listen = "127.0.0.1:{{.APIPort}}"

[censorship]
tls_domain = "{{.TLSDomain}}"
mask = true
mask_host = "{{.TLSDomain}}"
tls_emulation = true

[[upstreams]]
type = "socks5"
address = "127.0.0.1:{{.SocksPort}}"
username = "telemt"
password = "{{.SocksPassword}}"
weight = 1
enabled = true
{{if .Clients}}
[access.users]
{{range .Clients}}{{.Name}} = "{{.Secret}}"
{{end}}{{end}}`

func GenerateConfig(data ConfigData) (string, error) {
	t, err := template.New("telemt").Parse(telemtTOMLTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
