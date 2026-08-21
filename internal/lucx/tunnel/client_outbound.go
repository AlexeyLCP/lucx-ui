// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	SidecarProtocolNaive       = "naive"
	SidecarProtocolMieru       = "mieru"
	SidecarProtocolTrustTunnel = "trusttunnel"
)

// SidecarSettings is the Settings JSON of one sidecar_outbounds row.
type SidecarSettings struct {
	SocksPort         int                `json:"socksPort"`
	Link              string             `json:"link"`
	Host              string             `json:"host"`
	Port              int                `json:"port"`
	User              string             `json:"user"`
	Pass              string             `json:"pass"`
	SNI               string             `json:"sni,omitempty"`
	ALPN              string             `json:"alpn,omitempty"`
	Prefix            string             `json:"clientRandomPrefix,omitempty"`
	MTU               int                `json:"mtu,omitempty"`
	Multiplexing      string             `json:"multiplexing,omitempty"`
	HandshakeMode     string             `json:"handshakeMode,omitempty"`
	PortBindings      []MieruPortBinding `json:"portBindings,omitempty"`
	TrafficPatternB64 string             `json:"trafficPatternB64,omitempty"`
}

func (s SidecarSettings) Valid() bool {
	if strings.TrimSpace(s.Host) == "" || strings.TrimSpace(s.User) == "" {
		return false
	}
	if s.SocksPort <= 0 {
		return false
	}
	return true
}

func SidecarManageKey(protocol string, id int) string {
	switch protocol {
	case SidecarProtocolNaive:
		return "naiveout-" + strconv.Itoa(id)
	case SidecarProtocolMieru:
		return "mieruout-" + strconv.Itoa(id)
	case SidecarProtocolTrustTunnel:
		return "ttout-" + strconv.Itoa(id)
	default:
		return "sidecarout-" + strconv.Itoa(id)
	}
}

func SidecarCore(protocol string) (Name, bool) {
	switch protocol {
	case SidecarProtocolNaive:
		return NaiveClient, true
	case SidecarProtocolMieru:
		return MieruClient, true
	case SidecarProtocolTrustTunnel:
		return TrustTunnelClient, true
	default:
		return "", false
	}
}

func DefaultSidecarTag(protocol string, id int) string {
	return SidecarManageKey(protocol, id)
}

func ParseSidecarSettings(o *model.SidecarOutbound) (SidecarSettings, bool) {
	if o == nil {
		return SidecarSettings{}, false
	}
	var s SidecarSettings
	if strings.TrimSpace(o.Settings) != "" {
		if err := json.Unmarshal([]byte(o.Settings), &s); err != nil {
			return SidecarSettings{}, false
		}
	}
	return s, s.Valid()
}

func SettingsJSON(s SidecarSettings) string {
	b, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func BinaryExists(n Name) bool {
	info, err := os.Stat(n.BinaryPath())
	return err == nil && !info.IsDir()
}

func ParseSidecarLink(text string) (protocol string, s SidecarSettings, err error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return "", s, fmt.Errorf("sidecar: empty link")
	}
	switch {
	case strings.HasPrefix(raw, "naive+"):
		s, err = parseNaiveLink(raw)
		return SidecarProtocolNaive, s, err
	case strings.HasPrefix(raw, "mierus://") || strings.HasPrefix(raw, "mieru://"):
		s, err = parseMieruLink(raw)
		return SidecarProtocolMieru, s, err
	case strings.HasPrefix(raw, "tt://"):
		s, err = parseTrustTunnelLink(raw)
		return SidecarProtocolTrustTunnel, s, err
	default:
		return "", s, fmt.Errorf("sidecar: unsupported link (want naive+https://, mierus://, or tt://)")
	}
}

func parseNaiveLink(raw string) (SidecarSettings, error) {
	rest := strings.TrimPrefix(raw, "naive+")
	u, err := url.Parse(rest)
	if err != nil {
		return SidecarSettings{}, fmt.Errorf("sidecar: naive link: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return SidecarSettings{}, fmt.Errorf("sidecar: naive link missing host")
	}
	port := 443
	if p := u.Port(); p != "" {
		port, _ = strconv.Atoi(p)
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	if user == "" {
		return SidecarSettings{}, fmt.Errorf("sidecar: naive link missing user")
	}
	return SidecarSettings{
		Link: raw,
		Host: host,
		Port: port,
		User: user,
		Pass: pass,
	}, nil
}

func parseMieruLink(raw string) (SidecarSettings, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return SidecarSettings{}, fmt.Errorf("sidecar: mieru link: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return SidecarSettings{}, fmt.Errorf("sidecar: mieru link missing host")
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	if user == "" {
		return SidecarSettings{}, fmt.Errorf("sidecar: mieru link missing user")
	}
	q := u.Query()
	s := SidecarSettings{
		Link:              raw,
		Host:              host,
		User:              user,
		Pass:              pass,
		Multiplexing:      q.Get("multiplexing"),
		HandshakeMode:     q.Get("handshake-mode"),
		TrafficPatternB64: q.Get("traffic-pattern"),
	}
	if m := q.Get("mtu"); m != "" {
		s.MTU, _ = strconv.Atoi(m)
	}
	ports := q["port"]
	protos := q["protocol"]
	for i, p := range ports {
		proto := "TCP"
		if i < len(protos) && strings.TrimSpace(protos[i]) != "" {
			proto = strings.ToUpper(strings.TrimSpace(protos[i]))
		}
		if strings.Contains(p, "-") {
			s.PortBindings = append(s.PortBindings, MieruPortBinding{PortRange: p, Protocol: proto})
			continue
		}
		n, _ := strconv.Atoi(p)
		if n > 0 {
			s.PortBindings = append(s.PortBindings, MieruPortBinding{Port: n, Protocol: proto})
			if s.Port == 0 {
				s.Port = n
			}
		}
	}
	if len(s.PortBindings) == 0 && u.Port() != "" {
		n, _ := strconv.Atoi(u.Port())
		if n > 0 {
			s.Port = n
			s.PortBindings = []MieruPortBinding{{Port: n, Protocol: "TCP"}}
		}
	}
	if len(s.PortBindings) == 0 {
		return SidecarSettings{}, fmt.Errorf("sidecar: mieru link missing port")
	}
	return s, nil
}

func parseTrustTunnelLink(raw string) (SidecarSettings, error) {
	if strings.HasPrefix(raw, "tt://?") {
		return SidecarSettings{}, fmt.Errorf("sidecar: official tt://? TLV deep-link is not imported — paste a Throne tt://user:pass@host:port URI")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return SidecarSettings{}, fmt.Errorf("sidecar: trusttunnel link: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return SidecarSettings{}, fmt.Errorf("sidecar: trusttunnel link missing host")
	}
	port := 443
	if p := u.Port(); p != "" {
		port, _ = strconv.Atoi(p)
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	if user == "" {
		return SidecarSettings{}, fmt.Errorf("sidecar: trusttunnel link missing user")
	}
	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = host
	}
	alpn := q.Get("alpn")
	prefix := q.Get("client_random_prefix")
	return SidecarSettings{
		Link:   raw,
		Host:   host,
		Port:   port,
		User:   user,
		Pass:   pass,
		SNI:    sni,
		ALPN:   alpn,
		Prefix: prefix,
	}, nil
}

func RenderSidecarConfig(protocol string, s SidecarSettings) (string, error) {
	if !s.Valid() {
		return "", fmt.Errorf("sidecar: incomplete settings")
	}
	switch protocol {
	case SidecarProtocolNaive:
		return renderNaiveClientJSON(s)
	case SidecarProtocolMieru:
		return renderMieruClientJSON(s)
	case SidecarProtocolTrustTunnel:
		return renderTrustTunnelClientTOML(s)
	default:
		return "", fmt.Errorf("sidecar: unknown protocol %q", protocol)
	}
}

func renderNaiveClientJSON(s SidecarSettings) (string, error) {
	port := s.Port
	if port <= 0 {
		port = 443
	}
	proxy := url.URL{
		Scheme: "https",
		User:   url.UserPassword(s.User, s.Pass),
		Host:   net.JoinHostPort(s.Host, strconv.Itoa(port)),
	}
	cfg := map[string]string{
		"listen": "socks://127.0.0.1:" + strconv.Itoa(s.SocksPort),
		"proxy":  proxy.String(),
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func renderMieruClientJSON(s SidecarSettings) (string, error) {
	bindings := s.PortBindings
	if len(bindings) == 0 && s.Port > 0 {
		bindings = []MieruPortBinding{{Port: s.Port, Protocol: "TCP"}}
	}
	server := map[string]any{
		"portBindings": bindings,
	}
	if ip := net.ParseIP(s.Host); ip != nil {
		server["ipAddress"] = s.Host
		server["domainName"] = ""
	} else {
		server["ipAddress"] = ""
		server["domainName"] = s.Host
	}
	profile := map[string]any{
		"profileName": "default",
		"user": map[string]string{
			"name":     s.User,
			"password": s.Pass,
		},
		"servers": []any{server},
	}
	if s.MTU > 0 {
		profile["mtu"] = s.MTU
	}
	if m := strings.TrimSpace(s.Multiplexing); m != "" {
		profile["multiplexing"] = map[string]string{"level": m}
	}
	if h := strings.TrimSpace(s.HandshakeMode); h != "" {
		profile["handshakeMode"] = h
	}
	cfg := map[string]any{
		"profiles":        []any{profile},
		"activeProfile":   "default",
		"rpcPort":         0,
		"socks5Port":      s.SocksPort,
		"loggingLevel":    "INFO",
		"socks5ListenLAN": false,
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func renderTrustTunnelClientTOML(s SidecarSettings) (string, error) {
	port := s.Port
	if port <= 0 {
		port = 443
	}
	addr := net.JoinHostPort(s.Host, strconv.Itoa(port))
	sni := strings.TrimSpace(s.SNI)
	if sni == "" {
		sni = s.Host
	}
	proto := "http2"
	if strings.EqualFold(strings.TrimSpace(s.ALPN), "h3") {
		proto = "http3"
	}
	var b strings.Builder
	b.WriteString("loglevel = \"info\"\n")
	b.WriteString("vpn_mode = \"selective\"\n")
	b.WriteString("killswitch_enabled = false\n")
	b.WriteString("post_quantum_group_enabled = true\n")
	b.WriteString("exclusions = []\n\n")
	b.WriteString("[endpoint]\n")
	fmt.Fprintf(&b, "hostname = %q\n", sni)
	fmt.Fprintf(&b, "addresses = [%q]\n", addr)
	b.WriteString("has_ipv6 = true\n")
	fmt.Fprintf(&b, "username = %q\n", s.User)
	fmt.Fprintf(&b, "password = %q\n", s.Pass)
	if p := strings.TrimSpace(s.Prefix); p != "" {
		fmt.Fprintf(&b, "client_random = %q\n", p)
	}
	b.WriteString("skip_verification = false\n")
	fmt.Fprintf(&b, "upstream_protocol = %q\n\n", proto)
	b.WriteString("[listener.socks]\n")
	fmt.Fprintf(&b, "address = %q\n", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.SocksPort)))
	return b.String(), nil
}

func InstanceFromSidecarOutbound(o *model.SidecarOutbound) (Instance, bool) {
	if o == nil || !o.Enable {
		return Instance{}, false
	}
	core, ok := SidecarCore(o.Protocol)
	if !ok {
		return Instance{}, false
	}
	s, ok := ParseSidecarSettings(o)
	if !ok {
		return Instance{}, false
	}
	cfg, err := RenderSidecarConfig(o.Protocol, s)
	if err != nil {
		return Instance{}, false
	}
	key := SidecarManageKey(o.Protocol, o.Id)
	inst := Instance{
		Core:       core,
		Key:        key,
		Enabled:    true,
		ConfigText: cfg,
		ProbePort:  s.SocksPort,
	}
	switch core {
	case NaiveClient:
		inst.Args = []string{configPathFor(key, core)}
	case MieruClient:
		inst.Args = []string{"run"}
	case TrustTunnelClient:
		inst.Args = []string{"--config", configPathFor(key, core)}
	}
	return inst, true
}
