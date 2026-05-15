// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"bytes"
	"fmt"
	"text/template"
)

// TemplateData holds variables for PostUp/PostDown script rendering.
type TemplateData struct {
	AWGInterface   string
	TUNInterface   string
	AWGServerIP    string
	AWGSubnet      string
	AWGPort        int
	RouteTable     string
	RouteTableName string
	RoutePref      int
	MTU            int
}

const postUpTemplate = `#!/bin/bash
set -e
# LucX-UI AWG PostUp - generated, do not edit
# Interface: {{.AWGInterface}} -> TUN: {{.TUNInterface}}

ip addr add {{.AWGServerIP}}/24 dev {{.AWGInterface}}
ip link set {{.AWGInterface}} mtu {{.MTU}}
ip link set {{.AWGInterface}} up

ip rule add from {{.AWGSubnet}} table {{.RouteTable}} pref {{.RoutePref}} 2>/dev/null || true
ip route add default dev {{.TUNInterface}} table {{.RouteTable}} 2>/dev/null || true

iptables -A FORWARD -i {{.AWGInterface}} -o {{.TUNInterface}} -j ACCEPT
iptables -A FORWARD -i {{.TUNInterface}} -o {{.AWGInterface}} -j ACCEPT
iptables -t nat -A POSTROUTING -s {{.AWGSubnet}} -o {{.TUNInterface}} -j MASQUERADE
`

const postDownTemplate = `#!/bin/bash
set +e
# LucX-UI AWG PostDown - generated, do not edit
# Interface: {{.AWGInterface}}

iptables -t nat -D POSTROUTING -s {{.AWGSubnet}} -o {{.TUNInterface}} -j MASQUERADE 2>/dev/null || true
iptables -D FORWARD -i {{.AWGInterface}} -o {{.TUNInterface}} -j ACCEPT 2>/dev/null || true
iptables -D FORWARD -i {{.TUNInterface}} -o {{.AWGInterface}} -j ACCEPT 2>/dev/null || true

ip route del default dev {{.TUNInterface}} table {{.RouteTable}} 2>/dev/null || true
ip rule del from {{.AWGSubnet}} table {{.RouteTable}} pref {{.RoutePref}} 2>/dev/null || true

ip link del {{.AWGInterface}} 2>/dev/null || true

sed -i '/^{{.RouteTable}} {{.RouteTableName}}$/d' /etc/iproute2/rt_tables 2>/dev/null || true
`

func RenderPostUp(data TemplateData) (string, error) {
	return renderTemplate("PostUp", postUpTemplate, data)
}

func RenderPostDown(data TemplateData) (string, error) {
	return renderTemplate("PostDown", postDownTemplate, data)
}

func renderTemplate(name, tmpl string, data TemplateData) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}
