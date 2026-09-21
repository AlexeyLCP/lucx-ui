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
	NoProxy     bool     `json:"noProxy,omitempty"`
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
		n = sniMapKey(n)
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

// Classified is what the SNI gateway needs to know about one inbound: route
// class, SNI, PROXY-v1 capability (NoProxy) and a note for the preview table.
type Classified struct {
	Class   string
	SNI     string
	NoProxy bool
	Note    string
}

// udpOnlyNetwork marks Xray stream transports that never see a TCP listener.
func udpOnlyNetwork(netw string) bool {
	switch netw {
	case "kcp", "mkcp", "quic":
		return true
	}
	return false
}

// InboundUsesTCP reports whether the inbound holds a TCP listener that would
// fight the gateway for :443. UDP-only and behindCover inbounds own none.
func InboundUsesTCP(ib *model.Inbound) bool {
	if ib == nil || SettingsBehindCover(ib.Protocol, ib.Settings) {
		return false
	}
	switch ib.Protocol {
	case model.WireGuard, model.AmneziaWG, model.AWG, model.Hysteria, model.TUIC,
		model.Olcrtc, model.Qwdtt, model.Csqtt, model.Protocol("tun"):
		return false
	}
	netw, _ := parseStream(ib.StreamSettings)
	return !udpOnlyNetwork(netw)
}

// ClassifyInbound classifies one inbound for the SNI gateway.
func ClassifyInbound(ib *model.Inbound) Classified {
	if ib == nil {
		return Classified{}
	}
	switch ib.Protocol {
	case model.Cover:
		cfg, _ := CoverConfigFromInbound(ib)
		return Classified{Class: ClassCaddy, SNI: cfg.Hostname}
	case model.Naive:
		cfg, ok := ConfigFromInbound(ib)
		if !ok {
			return Classified{}
		}
		if cfg.BehindCover {
			return Classified{Note: "fronted by its cover site"}
		}
		if cfg.UseRawConfig {
			return Classified{Note: "raw Caddyfile owns its listener"}
		}
		c := Classified{Class: ClassCaddy, SNI: cfg.Domain}
		if cfg.UseAcme {
			c.Note = "auto TLS can't renew behind masking; apply rewrites it to the panel cert"
		}
		return c
	case model.Tproxy:
		cfg, ok := TproxyConfigFromInbound(ib)
		if !ok || cfg.BehindCover {
			return Classified{Note: "fronted by its cover site"}
		}
		return Classified{Class: ClassCaddy, SNI: cfg.Hostname}
	case model.Anytls:
		cfg, ok := AnytlsConfigFromInbound(ib)
		if !ok {
			return Classified{}
		}
		return Classified{
			Class: ClassPassthrough, SNI: cfg.SNI, NoProxy: true,
			Note: "backend can't parse PROXY — client IP hidden",
		}
	case model.TrustTunnel:
		cfg, ok := TrustTunnelConfigFromInbound(ib)
		if !ok {
			return Classified{}
		}
		return Classified{
			Class: ClassPassthrough, SNI: cfg.Hostname, NoProxy: true,
			Note: "backend can't parse PROXY — client IP hidden",
		}
	case model.Gateway, model.AWG, model.AmneziaWG, model.Olcrtc, model.Qwdtt,
		model.Csqtt, model.Mieru, model.MTProto, model.Tunnel, model.WireGuard,
		model.Hysteria, model.TUIC:
		return Classified{}
	}
	netw, sec := parseStream(ib.StreamSettings)
	if sec == "reality" || sec == "tls" {
		if udpOnlyNetwork(netw) {
			return Classified{Note: "UDP transport — SNI mux is TCP only"}
		}
		c := Classified{Class: ClassPassthrough, SNI: streamServerName(ib.StreamSettings)}
		if netw == "xhttp" || netw == "splithttp" || !XrayAcceptsProxyProtocol(ib.Protocol) {
			c.NoProxy = true
			c.Note = "backend can't parse PROXY — client IP hidden"
		}
		return c
	}
	if sec == "" {
		switch netw {
		case "ws", "xhttp", "splithttp", "grpc", "httpupgrade":
			return Classified{Note: "no TLS — SNI can't route it"}
		}
	}
	return Classified{}
}

// Classify returns passthrough, caddy, or empty (skip).
func Classify(ib *model.Inbound) (class, sni string) {
	c := ClassifyInbound(ib)
	return c.Class, c.SNI
}

func XrayAcceptsProxyProtocol(p model.Protocol) bool {
	switch p {
	case model.VLESS, model.VMESS, model.Trojan, model.Shadowsocks:
		return true
	default:
		return false
	}
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

func setSettingsKey(ib *model.Inbound, key, val string) {
	var m map[string]any
	_ = json.Unmarshal([]byte(ib.Settings), &m)
	if m == nil {
		m = map[string]any{}
	}
	m[key] = val
	out, err := json.Marshal(m)
	if err != nil {
		return
	}
	ib.Settings = string(out)
}

func setStreamServerName(stream, sni string, reality bool) string {
	raw := strings.TrimSpace(stream)
	if raw == "" {
		raw = "{}"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return stream
	}
	if reality {
		rs, _ := m["realitySettings"].(map[string]any)
		if rs == nil {
			rs = map[string]any{}
		}
		rs["serverNames"] = []string{sni}
		m["realitySettings"] = rs
	} else {
		ts, _ := m["tlsSettings"].(map[string]any)
		if ts == nil {
			ts = map[string]any{}
		}
		ts["serverName"] = sni
		m["tlsSettings"] = ts
	}
	out, err := json.Marshal(m)
	if err != nil {
		return stream
	}
	return string(out)
}

// SetInboundSNI writes sni into the inbound's own settings (Cover/Naive/tproxy/AnyTLS)
// or stream (REALITY serverNames / TLS serverName). Empty sni is a no-op.
func SetInboundSNI(ib *model.Inbound, sni string) {
	sni = sniMapKey(sni)
	if ib == nil || sni == "" {
		return
	}
	switch ib.Protocol {
	case model.Cover, model.Tproxy, model.TrustTunnel:
		setSettingsKey(ib, "hostname", sni)
	case model.Naive:
		setSettingsKey(ib, "domain", sni)
	case model.Anytls:
		setSettingsKey(ib, "sni", sni)
	default:
		_, sec := parseStream(ib.StreamSettings)
		switch sec {
		case "reality":
			ib.StreamSettings = setStreamServerName(ib.StreamSettings, sni, true)
		case "tls":
			ib.StreamSettings = setStreamServerName(ib.StreamSettings, sni, false)
		}
	}
}

func SNIClash(rows []PreviewRow, selected map[int]bool) string {
	seen := map[string]bool{}
	for _, r := range rows {
		if selected != nil && !selected[r.InboundID] {
			continue
		}
		if r.Class == ClassSkip {
			continue
		}
		k := sniMapKey(r.SNI)
		if k == "" {
			continue
		}
		if seen[k] {
			return k
		}
		seen[k] = true
	}
	return ""
}

func xrayTransportSettingsKey(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "ws":
		return "wsSettings"
	case "httpupgrade":
		return "httpupgradeSettings"
	case "tcp", "raw", "":
		return "tcpSettings"
	default:
		return ""
	}
}

// SetAcceptProxyProtocol toggles the transport flag Xray needs when the SNI mux
// sends PROXY protocol. Empty dest-like no-op if JSON is broken.
func SetAcceptProxyProtocol(stream string, on bool) string {
	raw := strings.TrimSpace(stream)
	if raw == "" {
		if !on {
			return stream
		}
		raw = "{}"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m == nil {
		return stream
	}
	netw, _ := m["network"].(string)
	key := xrayTransportSettingsKey(netw)
	if key == "" {
		so, _ := m["sockopt"].(map[string]any)
		if on {
			if so == nil {
				so = map[string]any{}
			}
			so["acceptProxyProtocol"] = true
			m["sockopt"] = so
		} else if so != nil {
			delete(so, "acceptProxyProtocol")
			if len(so) == 0 {
				delete(m, "sockopt")
			} else {
				m["sockopt"] = so
			}
		}
	} else {
		tr, _ := m[key].(map[string]any)
		if tr == nil {
			tr = map[string]any{}
		}
		if on {
			tr["acceptProxyProtocol"] = true
		} else {
			delete(tr, "acceptProxyProtocol")
		}
		if len(tr) == 0 {
			delete(m, key)
		} else {
			m[key] = tr
		}
	}
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

// BuildPreview lists inbounds the mask may move behind 443.
// bindIP set → keep port 443 on loopback (Cover first if it is on 443).
func BuildPreview(gatewayPort int, publicHost string, others []*model.Inbound, bindIP string) []PreviewRow {
	if gatewayPort <= 0 {
		gatewayPort = gatewayDefaultPort
	}
	publicHost = strings.ToLower(strings.TrimSpace(publicHost))
	keep443 := strings.TrimSpace(bindIP) != ""
	used := map[int]bool{}
	if !keep443 {
		used[gatewayPort] = true
	}
	cover443 := false
	if keep443 {
		for _, ib := range others {
			if ib != nil && ib.Protocol == model.Cover && ib.Port == gatewayPort {
				cover443 = true
				break
			}
		}
	}
	slot443 := false
	var rows []PreviewRow
	for _, ib := range others {
		if ib == nil || ib.Protocol == model.Gateway {
			continue
		}
		cf := ClassifyInbound(ib)
		class, sni := cf.Class, cf.SNI
		if class == "" {
			if ib.Enable && ib.Port > 0 && !IsLoopbackListen(ib.Listen) {
				listen := publicListen(ib.Listen)
				note := cf.Note
				if note == "" {
					note = "no SNI, stays public"
				}
				if ib.Port == gatewayPort && InboundUsesTCP(ib) {
					note = "TCP :443 conflicts with the gateway port"
				}
				rows = append(rows, PreviewRow{
					InboundID: ib.Id, Remark: ib.Remark, Protocol: string(ib.Protocol),
					Class: ClassSkip, OldListen: listen, NewListen: listen,
					OldPort: ib.Port, NewPort: ib.Port, Note: note,
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
		if oldPort == gatewayPort {
			take := keep443 && !slot443
			if take && cover443 {
				take = ib.Protocol == model.Cover
			}
			if take {
				newPort = gatewayPort
				slot443 = true
			} else {
				newPort = nextFreePort(class, used)
			}
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
			NoProxy:     cf.NoProxy,
			Note:        cf.Note,
		}
		if class == ClassCaddy && cf.SNI == "" && publicHost == "" {
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

func previewRouteNames(r PreviewRow) []string {
	if len(r.SNIs) > 0 {
		return r.SNIs
	}
	if r.SNI != "" {
		return []string{r.SNI}
	}
	return nil
}

func appendPreviewRoutes(out []GatewayRoute, seen map[string]bool, rows []PreviewRow, selected map[int]bool, caddy bool) []GatewayRoute {
	for _, r := range rows {
		if selected != nil && !selected[r.InboundID] {
			continue
		}
		if caddy != (r.Class == ClassCaddy) {
			continue
		}
		dest := gatewayLoopbackDest(r.NewPort)
		for _, sni := range previewRouteNames(r) {
			sni = sniMapKey(sni)
			if sni == "" || seen[sni] {
				continue
			}
			seen[sni] = true
			out = append(out, GatewayRoute{SNI: sni, Dest: dest, NoProxy: r.NoProxy})
		}
	}
	return out
}

func RoutesFromPreview(rows []PreviewRow, selected map[int]bool) []GatewayRoute {
	seen := map[string]bool{}
	out := appendPreviewRoutes(nil, seen, rows, selected, true)
	return appendPreviewRoutes(out, seen, rows, selected, false)
}
