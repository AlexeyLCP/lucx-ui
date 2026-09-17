// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// PreviewRow is one inbound the wizard may change.
type PreviewRow struct {
	InboundID   int      `json:"inboundId"`
	Remark      string   `json:"remark"`
	Protocol    string   `json:"protocol"`
	Class       string   `json:"class"`
	SNI         string   `json:"sni"`
	SNIs        []string `json:"snis,omitempty"`
	OldListen   string   `json:"oldListen"`
	NewListen   string   `json:"newListen"`
	OldPort     int      `json:"oldPort"`
	NewPort     int      `json:"newPort"`
	HostAddress string   `json:"hostAddress"`
	HostPort    int      `json:"hostPort"`
	StealDest   string   `json:"stealDest,omitempty"`
	Note        string   `json:"note,omitempty"`
}

func publicListen(listen string) string {
	s := strings.TrimSpace(listen)
	if s == "" || s == "0.0.0.0" || s == "::" || s == "0.0.0.0/0" {
		return "0.0.0.0"
	}
	return s
}

func IsLoopbackListen(listen string) bool {
	s := strings.TrimSpace(listen)
	return s == "127.0.0.1" || s == "::1"
}

func parseStream(raw string) (network, security string) {
	var s struct {
		Network  string `json:"network"`
		Security string `json:"security"`
	}
	_ = json.Unmarshal([]byte(raw), &s)
	return strings.ToLower(strings.TrimSpace(s.Network)), strings.ToLower(strings.TrimSpace(s.Security))
}

func streamServerName(raw string) string {
	ns := streamServerNames(raw)
	if len(ns) == 0 {
		return ""
	}
	return ns[0]
}

func streamServerNames(raw string) []string {
	var s struct {
		RealitySettings struct {
			ServerNames []string `json:"serverNames"`
			Dest        string   `json:"dest"`
			Target      string   `json:"target"`
		} `json:"realitySettings"`
		TLSSettings struct {
			ServerName string `json:"serverName"`
		} `json:"tlsSettings"`
	}
	_ = json.Unmarshal([]byte(raw), &s)
	var out []string
	seen := map[string]bool{}
	add := func(n string) {
		n = nginxMapKey(n)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	for _, n := range s.RealitySettings.ServerNames {
		add(n)
	}
	dest := s.RealitySettings.Dest
	if dest == "" {
		dest = s.RealitySettings.Target
	}
	add(destHost(dest))
	add(s.TLSSettings.ServerName)
	return out
}

func destHost(dest string) string {
	dest = strings.ToLower(strings.TrimSpace(dest))
	if dest == "" {
		return ""
	}
	if i := strings.LastIndex(dest, ":"); i > 0 {
		dest = dest[:i]
	}
	return dest
}

// Classify returns passthrough, caddy, or empty (skip).
func Classify(ib *model.Inbound) (class, sni string) {
	if ib == nil {
		return "", ""
	}
	switch ib.Protocol {
	case model.Cover:
		cfg, _ := CoverConfigFromInbound(ib)
		return ClassCaddy, cfg.Hostname
	case model.Naive:
		cfg, ok := ConfigFromInbound(ib)
		if !ok {
			return "", ""
		}
		return ClassCaddy, cfg.Domain
	case model.Tproxy:
		cfg, ok := TproxyConfigFromInbound(ib)
		if !ok {
			return "", ""
		}
		return ClassCaddy, cfg.Hostname
	case model.Anytls:
		cfg, ok := AnytlsConfigFromInbound(ib)
		if !ok {
			return "", ""
		}
		return ClassPassthrough, cfg.SNI
	case model.TrustTunnel:
		cfg, ok := TrustTunnelConfigFromInbound(ib)
		if !ok {
			return "", ""
		}
		return ClassPassthrough, cfg.Hostname
	case model.Gateway, model.AWG, model.Olcrtc, model.Qwdtt, model.Csqtt,
		model.Mieru, model.MTProto, model.Tunnel, model.WireGuard,
		model.Hysteria:
		return "", ""
	}
	netw, sec := parseStream(ib.StreamSettings)
	if sec == "reality" || sec == "tls" {
		return ClassPassthrough, streamServerName(ib.StreamSettings)
	}
	switch netw {
	case "ws", "xhttp", "grpc", "httpupgrade":
		return ClassCaddy, ""
	}
	return "", ""
}

// SetRealityDest writes dest+target in stream JSON. Empty dest is a no-op.
func SetRealityDest(stream, dest string) string {
	dest = strings.TrimSpace(dest)
	if dest == "" || strings.TrimSpace(stream) == "" {
		return stream
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(stream), &m); err != nil || m == nil {
		return stream
	}
	raw, _ := m["realitySettings"].(map[string]any)
	if raw == nil {
		raw = map[string]any{}
	}
	raw["dest"] = dest
	raw["target"] = dest
	m["realitySettings"] = raw
	out, err := json.Marshal(m)
	if err != nil {
		return stream
	}
	return string(out)
}

func coverLoopbackPort(rows []PreviewRow) int {
	for _, r := range rows {
		if r.Protocol == string(model.Cover) {
			return r.NewPort
		}
	}
	return 0
}

// ResolveGatewayPublicHost: request, then saved settings, then first classified SNI.
func ResolveGatewayPublicHost(req, saved string, others []*model.Inbound) string {
	if h := strings.ToLower(strings.TrimSpace(req)); h != "" {
		return h
	}
	if h := strings.ToLower(strings.TrimSpace(saved)); h != "" {
		return h
	}
	for _, o := range others {
		if _, sni := Classify(o); sni != "" {
			return sni
		}
	}
	return ""
}

// BuildPreview lists inbounds the mask may move behind nginx.
func BuildPreview(gatewayPort int, publicHost string, others []*model.Inbound) []PreviewRow {
	if gatewayPort <= 0 {
		gatewayPort = gatewayDefaultPort
	}
	publicHost = strings.ToLower(strings.TrimSpace(publicHost))
	used := map[int]bool{gatewayPort: true}
	var rows []PreviewRow
	for _, ib := range others {
		if ib == nil || ib.Protocol == model.Gateway {
			continue
		}
		class, sni := Classify(ib)
		if class == "" {
			if ib.Enable && ib.Port > 0 && !IsLoopbackListen(ib.Listen) {
				listen := publicListen(ib.Listen)
				rows = append(rows, PreviewRow{
					InboundID: ib.Id, Remark: ib.Remark, Protocol: string(ib.Protocol),
					Class: ClassSkip, OldListen: listen, NewListen: listen,
					OldPort: ib.Port, NewPort: ib.Port, Note: "no SNI, stays public",
				})
			}
			continue
		}
		if sni == "" {
			sni = publicHost
		}
		oldListen := publicListen(ib.Listen)
		oldPort := ib.Port
		newListen := "127.0.0.1"
		newPort := oldPort
		if oldPort == gatewayPort || (oldListen != "127.0.0.1" && oldListen != "::1" && oldPort == gatewayPort) {
			newPort = nextFreePort(class, used)
		}
		if oldPort <= 0 {
			newPort = nextFreePort(class, used)
		}
		used[newPort] = true
		hostAddr := publicHost
		if hostAddr == "" {
			hostAddr = sni
		}
		row := PreviewRow{
			InboundID:   ib.Id,
			Remark:      ib.Remark,
			Protocol:    string(ib.Protocol),
			Class:       class,
			SNI:         sni,
			SNIs:        streamServerNames(ib.StreamSettings),
			OldListen:   oldListen,
			NewListen:   newListen,
			OldPort:     oldPort,
			NewPort:     newPort,
			HostAddress: hostAddr,
			HostPort:    gatewayDefaultPort,
		}
		if class == ClassCaddy && sni == "" {
			row.Note = "needs Cover or publicHost"
		}
		rows = append(rows, row)
	}
	if p := coverLoopbackPort(rows); p > 0 {
		steal := gatewayLoopbackDest(p)
		for i := range rows {
			if rows[i].Class == ClassPassthrough {
				rows[i].StealDest = steal
			}
		}
	}
	return rows
}

func nextFreePort(class string, used map[int]bool) int {
	base := gatewayPassthroughBase
	if class == ClassCaddy {
		base = gatewayCaddyBase
	}
	for p := base; p < base+200; p++ {
		if !used[p] {
			return p
		}
	}
	return base
}

func CoverFallback(rows []PreviewRow, selected map[int]bool) string {
	for _, r := range rows {
		if r.Protocol != string(model.Cover) {
			continue
		}
		if selected != nil && !selected[r.InboundID] {
			continue
		}
		return gatewayLoopbackDest(r.NewPort)
	}
	return ""
}

func RoutesFromPreview(rows []PreviewRow, selected map[int]bool) []GatewayRoute {
	var out []GatewayRoute
	seen := map[string]bool{}
	for _, r := range rows {
		if selected != nil && !selected[r.InboundID] {
			continue
		}
		names := r.SNIs
		if len(names) == 0 && r.SNI != "" {
			names = []string{r.SNI}
		}
		dest := gatewayLoopbackDest(r.NewPort)
		for _, sni := range names {
			sni = nginxMapKey(sni)
			if sni == "" || seen[sni] {
				continue
			}
			seen[sni] = true
			out = append(out, GatewayRoute{SNI: sni, Dest: dest})
		}
	}
	return out
}
