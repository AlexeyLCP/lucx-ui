// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const Gateway Name = "gateway"

const (
	gatewayDefaultPort     = 443
	gatewayDropBackend     = "127.0.0.1:1"
	gatewayPassthroughBase = 1443
	gatewayCaddyBase       = 8443
)

const (
	ClassPassthrough = "passthrough"
	ClassCaddy       = "caddy"
	ClassSkip        = "skip"
)

// GatewayRoute is one SNI → loopback backend for nginx stream.
type GatewayRoute struct {
	SNI  string `json:"sni"`
	Dest string `json:"dest"`
}

// GatewaySnapshotRow is enough to undo one inbound after Apply.
type GatewaySnapshotRow struct {
	InboundID      int    `json:"inboundId"`
	Listen         string `json:"listen"`
	Port           int    `json:"port"`
	StreamSettings string `json:"streamSettings,omitempty"`
	HostID         int    `json:"hostId,omitempty"`
}

// GatewayConfig lives in inbound settings. Snapshot empty = mask not applied.
type GatewayConfig struct {
	Remark       string               `json:"remark"`
	Enabled      bool                 `json:"enabled"`
	PublicHost   string               `json:"publicHost"`
	BindIP       string               `json:"bindIP,omitempty"`
	Routes       []GatewayRoute       `json:"routes"`
	Snapshot     []GatewaySnapshotRow `json:"snapshot"`
	Fallback     string               `json:"fallback,omitempty"`
	UFW          bool                 `json:"ufw,omitempty"`
	UFWWasActive bool                 `json:"ufwWasActive,omitempty"`
	HidePanel    bool                 `json:"hidePanel,omitempty"`
	PanelRoutes  []CoverRoute         `json:"panelRoutes,omitempty"`
}

func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{}
}

func (c GatewayConfig) Merge() GatewayConfig {
	c.PublicHost = strings.ToLower(strings.TrimSpace(c.PublicHost))
	if c.Routes == nil {
		c.Routes = []GatewayRoute{}
	}
	if c.Snapshot == nil {
		c.Snapshot = []GatewaySnapshotRow{}
	}
	return c
}

func (c GatewayConfig) Applied() bool {
	return len(c.Snapshot) > 0
}

func nginxMapKey(sni string) string {
	sni = strings.ToLower(strings.TrimSpace(sni))
	sni = strings.ReplaceAll(sni, `"`, "")
	sni = strings.ReplaceAll(sni, " ", "")
	return sni
}

// LocalIPv4 is the IPv4 of the default route. Empty if unknown.
func LocalIPv4() string {
	d := &net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(context.Background(), "udp4", "1.1.1.1:53")
	if err != nil {
		return ""
	}
	defer c.Close()
	host, _, err := net.SplitHostPort(c.LocalAddr().String())
	if err != nil {
		return ""
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return ""
	}
	return ip.String()
}

// RenderNginxConf is stream ssl_preread by SNI. Empty fallback = drop.
// bindIP set → listen IP:port so loopback:port stays free for backends.
func RenderNginxConf(listenPort int, pidPath string, routes []GatewayRoute, fallback, bindIP string) string {
	if listenPort <= 0 {
		listenPort = gatewayDefaultPort
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		fallback = gatewayDropBackend
	}
	listen := strconv.Itoa(listenPort)
	if ip := strings.TrimSpace(bindIP); ip != "" {
		listen = ip + ":" + listen
	}
	var b strings.Builder
	b.WriteString("worker_processes 1;\n")
	b.WriteString("error_log stderr error;\n")
	if strings.TrimSpace(pidPath) != "" {
		b.WriteString("pid " + pidPath + ";\n")
	}
	b.WriteString("events { worker_connections 256; }\n")
	b.WriteString("stream {\n")
	b.WriteString("\tmap $ssl_preread_server_name $lucx_gw {\n")
	seen := map[string]bool{}
	for _, r := range routes {
		k := nginxMapKey(r.SNI)
		d := strings.TrimSpace(r.Dest)
		if k == "" || d == "" || seen[k] {
			continue
		}
		seen[k] = true
		b.WriteString("\t\t" + k + " " + d + ";\n")
	}
	b.WriteString("\t\tdefault " + fallback + ";\n")
	b.WriteString("\t}\n")
	b.WriteString("\tserver {\n")
	b.WriteString("\t\tlisten " + listen + ";\n")
	b.WriteString("\t\tssl_preread on;\n")
	b.WriteString("\t\tproxy_protocol on;\n")
	b.WriteString("\t\tproxy_pass $lucx_gw;\n")
	b.WriteString("\t\tproxy_timeout 1d;\n")
	b.WriteString("\t\tproxy_connect_timeout 5s;\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")
	return b.String()
}

func gatewayLoopbackDest(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}
