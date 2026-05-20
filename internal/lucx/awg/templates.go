// Copyright (c) 2025 LucX-UI Project.

package awg

import (
	"bytes"
	"fmt"
	"text/template"
)

const defaultSOCKS5Port = 10808

type TemplateData struct {
	AWGInterface   string
	AWGServerIP    string
	AWGSubnet      string
	AWGPort        int
	RouteTable     string
	RouteTableName string
	RoutePref      int
	MTU            int
}

// PostUp is minimal: just create iface, bring up, add routing.
// tun2socks is started separately by manager.go via os/exec.
const postUpTemplate = `#!/bin/bash
set -e
# LUCX-AWG-ROUTING: PostUp for {{.AWGInterface}}
ip link add {{.AWGInterface}} type amneziawg 2>/dev/null || true
ip addr add {{.AWGServerIP}}/24 dev {{.AWGInterface}} 2>/dev/null || true
ip link set {{.AWGInterface}} mtu {{.MTU}}
ip link set {{.AWGInterface}} up
sysctl -w net.ipv4.ip_forward=1 -q
`

const postDownTemplate = `#!/bin/bash
set +e
ip link del {{.AWGInterface}} 2>/dev/null || true
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
