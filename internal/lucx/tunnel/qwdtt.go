// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QwdttConfig is the operator-facing configuration of the qWDTT core
// (SpaceNeuroX/proxy-turn-vk-android server.go — WireGuard over VK TURN).
// The binary takes pure CLI flags (no config file); state lives in ConfigDir
// (passwords.json, wg-keys.dat). Sidecar pin: SpaceNeuroX v1.4.2 (see release.yml).
type QwdttConfig struct {
	Remark  string `json:"remark"`
	Enabled bool   `json:"enabled"`

	// ListenAddr is the DTLS client entry, e.g. "0.0.0.0:56000".
	ListenAddr string `json:"listenAddr"`
	// WGPort is the internal userspace WireGuard UDP port.
	WGPort int `json:"wgPort"`
	// Password is the main shared password (auto-generated when empty on save).
	Password string `json:"password"`
	// DNS is pushed to clients (bare IP, no port — unlike olcrtc).
	DNS string `json:"dns"`
	// ConfigDir holds passwords.json + wg-keys.dat. Empty → DataDir(Qwdtt).
	ConfigDir string `json:"configDir"`
	// ListenRaw enables the optional raw-IP path (no WG), e.g. "0.0.0.0:56003".
	ListenRaw string `json:"listenRaw"`
	// ListenDirect is the optional direct RTP-obfs listener (empty = off).
	ListenDirect string `json:"listenDirect"`

	// SubHost is "ip:dtlsPort" advertised to clients (empty → detect on save
	// is left to the operator / UI placeholder).
	SubHost string `json:"subHost"`
	// VkHashes is a comma-separated list of live VK call hashes the client
	// needs to open a TURN path. Operational burden stays with the operator.
	VkHashes string `json:"vkHashes"`
	// ClientPort is the Android-side local TUN port in the share link
	// (not a server port). Default 9000.
	ClientPort int `json:"clientPort"`
	// Workers is workersPerHash in the client JSON profile. Default 16.
	Workers int `json:"workers"`

	// RouteThroughXray steers wdtt0/wdttraw0 into an Xray TUN inbound (policy
	// routing) so egress uses Xray routing / outboundTag. Default true.
	RouteThroughXray bool `json:"routeThroughXray"`
	// OutboundTag optional force-route target (empty = Xray default kettle).
	OutboundTag string `json:"outboundTag"`
}

// DefaultQwdttConfig returns sensible defaults for a fresh qWDTT core.
func DefaultQwdttConfig() QwdttConfig {
	return QwdttConfig{
		ListenAddr:       "0.0.0.0:56000",
		WGPort:           56001,
		DNS:              "8.8.8.8",
		ListenRaw:        "0.0.0.0:56003",
		ClientPort:       9000,
		Workers:          16,
		RouteThroughXray: true,
	}
}

// Merge fills zero fields of c from the defaults.
func (c QwdttConfig) Merge() QwdttConfig {
	def := DefaultQwdttConfig()
	if c.ListenAddr == "" {
		c.ListenAddr = def.ListenAddr
	}
	if c.WGPort == 0 {
		c.WGPort = def.WGPort
	}
	if c.DNS == "" {
		c.DNS = def.DNS
	}
	if c.ListenRaw == "" {
		c.ListenRaw = def.ListenRaw
	}
	if c.ClientPort == 0 {
		c.ClientPort = def.ClientPort
	}
	if c.Workers == 0 {
		c.Workers = def.Workers
	}
	return c
}

// Validate checks the config for internal consistency against the upstream
// wdtt-server CLI.
func (c QwdttConfig) Validate() error {
	if _, port, err := net.SplitHostPort(c.ListenAddr); err != nil || port == "" {
		return errors.New("qwdtt: listenAddr must be host:port (e.g. 0.0.0.0:56000)")
	}
	if c.WGPort < 1 || c.WGPort > 65535 {
		return errors.New("qwdtt: wgPort must be in 1..65535")
	}
	if strings.TrimSpace(c.DNS) == "" {
		return errors.New("qwdtt: dns is required")
	}
	// DNS is a bare host for the server flag (no port).
	if strings.Contains(c.DNS, ":") {
		if _, _, err := net.SplitHostPort(c.DNS); err == nil {
			return errors.New("qwdtt: dns must be a bare host/IP (no port), e.g. 8.8.8.8")
		}
	}
	if raw := strings.TrimSpace(c.ListenRaw); raw != "" {
		if _, _, err := net.SplitHostPort(raw); err != nil {
			return errors.New("qwdtt: listenRaw must be host:port or empty")
		}
	}
	if dir := strings.TrimSpace(c.ListenDirect); dir != "" {
		if _, _, err := net.SplitHostPort(dir); err != nil {
			return errors.New("qwdtt: listenDirect must be host:port or empty")
		}
	}
	if c.ClientPort < 1 || c.ClientPort > 65535 {
		return errors.New("qwdtt: clientPort must be in 1..65535")
	}
	if c.Workers < 1 || c.Workers > 64 {
		return errors.New("qwdtt: workers must be in 1..64")
	}
	return nil
}

// EnsurePassword returns c with a generated main password when empty.
func (c QwdttConfig) EnsurePassword() (QwdttConfig, error) {
	if strings.TrimSpace(c.Password) != "" {
		return c, nil
	}
	pass, err := generateQwdttPassword(16)
	if err != nil {
		return c, err
	}
	c.Password = pass
	return c, nil
}

// EnsureSubHost fills SubHost with "<outboundIPv4>:<dtlsPort>" when empty so
// ClientURI / subscription always have a peer after save. Dial-based probe
// (no HTTP); fails open (leaves empty) when the host has no outbound route.
func (c QwdttConfig) EnsureSubHost() QwdttConfig {
	if strings.TrimSpace(c.SubHost) != "" {
		return c
	}
	ip := detectOutboundIPv4()
	if ip == "" {
		return c
	}
	c.SubHost = net.JoinHostPort(ip, strconv.Itoa(c.publicDTLSPort()))
	return c
}

// detectOutboundIPv4 returns the IPv4 the kernel would use for external
// traffic (UDP dial to a public DNS, no packets needed beyond connect).
func detectOutboundIPv4() string {
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(context.Background(), "udp4", "8.8.8.8:53")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return ""
	}
	ip := addr.IP.To4()
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}

// ResolveConfigDir returns the effective state directory.
func (c QwdttConfig) ResolveConfigDir() string {
	if d := strings.TrimSpace(c.ConfigDir); d != "" {
		return d
	}
	return DataDir(Qwdtt)
}

// BuildArgs converts the config into the argv passed to the binary
// (flags only — no subcommand). Matches server.go flag set.
func (c QwdttConfig) BuildArgs() []string {
	args := []string{
		"-listen", c.ListenAddr,
		"-wg-port", strconv.Itoa(c.WGPort),
		"-config-dir", c.ResolveConfigDir(),
	}
	if p := strings.TrimSpace(c.Password); p != "" {
		args = append(args, "-password", p)
	}
	if d := strings.TrimSpace(c.DNS); d != "" {
		args = append(args, "-dns", d)
	}
	if raw := strings.TrimSpace(c.ListenRaw); raw != "" {
		args = append(args, "-listen-raw", raw)
	}
	if dir := strings.TrimSpace(c.ListenDirect); dir != "" {
		args = append(args, "-listen-direct", dir)
	}
	return args
}

// publicDTLSPort extracts the listen port from ListenAddr, defaulting to 56000.
func (c QwdttConfig) publicDTLSPort() int {
	if _, port, err := net.SplitHostPort(c.ListenAddr); err == nil {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			return p
		}
	}
	return 56000
}

// peerHost returns the host:dtlsPort advertised to clients.
func (c QwdttConfig) peerHost() string {
	if h := strings.TrimSpace(c.SubHost); h != "" {
		return h
	}
	return ""
}

// ClientURI renders the qwdtt://config?... share link understood by the
// SpaceNeuroX Android client. Empty password or peer yields "".
func (c QwdttConfig) ClientURI() string {
	peer := c.peerHost()
	pass := strings.TrimSpace(c.Password)
	if peer == "" || pass == "" {
		return ""
	}
	name := strings.TrimSpace(c.Remark)
	if name == "" {
		name = "qWDTT"
	}
	q := url.Values{}
	q.Set("name", name)
	q.Set("peer", peer)
	if h := strings.TrimSpace(c.VkHashes); h != "" {
		q.Set("hashes", h)
	}
	q.Set("workers", strconv.Itoa(c.Workers))
	q.Set("port", strconv.Itoa(c.ClientPort))
	q.Set("pass", pass)
	return "qwdtt://config?" + q.Encode()
}

// LegacyURI renders the classic wdtt://ip:dtls:wg:local:pass:hash form.
// Requires SubHost, password and at least one VK hash.
func (c QwdttConfig) LegacyURI() string {
	peer := c.peerHost()
	pass := strings.TrimSpace(c.Password)
	hash := strings.TrimSpace(strings.Split(c.VkHashes, ",")[0])
	if peer == "" || pass == "" || hash == "" {
		return ""
	}
	// peer may already carry :port — strip to host for the legacy layout.
	host := peer
	if h, _, err := net.SplitHostPort(peer); err == nil {
		host = h
	}
	return fmt.Sprintf("wdtt://%s:%d:%d:%d:%s:%s",
		host, c.publicDTLSPort(), c.WGPort, c.ClientPort, pass, hash)
}

// QwdttSubProfile is one profile inside a qWDTT subscription JSON document.
type QwdttSubProfile struct {
	Name     string `json:"name"`
	Peer     string `json:"peer"`
	Hashes   string `json:"hashes"`
	Workers  int    `json:"workers"`
	Port     int    `json:"port"`
	Password string `json:"password"`
}

// QwdttSubscription is the public JSON document the Android client imports
// (SpaceNeuroX subscription format).
type QwdttSubscription struct {
	SubscriptionName string            `json:"subscriptionName"`
	Description      string            `json:"description,omitempty"`
	Version          int               `json:"version"`
	UpdatedAt        string            `json:"updatedAt"`
	Profiles         []QwdttSubProfile `json:"profiles"`
}

// Subscription builds the Android subscription document. Returns an error
// when peer host or password is missing.
func (c QwdttConfig) Subscription() (QwdttSubscription, error) {
	peer := c.peerHost()
	pass := strings.TrimSpace(c.Password)
	if peer == "" {
		return QwdttSubscription{}, errors.New("qwdtt: subHost is required for a subscription")
	}
	if pass == "" {
		return QwdttSubscription{}, errors.New("qwdtt: password is required for a subscription")
	}
	name := strings.TrimSpace(c.Remark)
	if name == "" {
		name = "qWDTT"
	}
	return QwdttSubscription{
		SubscriptionName: name,
		Description:      "qWDTT tunnel via " + peer,
		Version:          1,
		UpdatedAt:        time.Now().Format("2006-01-02"),
		Profiles: []QwdttSubProfile{{
			Name:     name,
			Peer:     peer,
			Hashes:   strings.TrimSpace(c.VkHashes),
			Workers:  c.Workers,
			Port:     c.ClientPort,
			Password: pass,
		}},
	}, nil
}

// SubscriptionJSON is a pretty-printed Subscription for the panel UI.
func (c QwdttConfig) SubscriptionJSON() (string, error) {
	sub, err := c.Subscription()
	if err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(sub, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// generateQwdttPassword returns n random chars from a URL-safe alphabet
// (matches the server's generated-password charset spirit, without 0/O/1/l).
func generateQwdttPassword(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, raw := range b {
		out[i] = alphabet[int(raw)%len(alphabet)]
	}
	return string(out), nil
}
