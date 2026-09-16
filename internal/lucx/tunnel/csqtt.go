// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	csqttDefaultListen = "0.0.0.0:46000"
	csqttDefaultDNS    = "77.88.8.8,77.88.8.1"
	csqttWebPort       = 46002
	csqttWebUser       = "lucx"
)

// CsqttConfig is the operator-facing configuration of the CSQTT core
// (amurcanov/csqtt rust-server — TURN/RTP, not qWDTT). CLI flags only.
// Binary is operator-supplied (Cores) until release.yml ships a musl build.
type CsqttConfig struct {
	Remark     string `json:"remark"`
	Enabled    bool   `json:"enabled"`
	ListenAddr string `json:"listenAddr"`
	Password   string `json:"password"`
	WebPass    string `json:"webPass,omitempty"`
	SubHost    string `json:"subHost"`
	VkHashes   string `json:"vkHashes"`
	ConfigDir  string `json:"configDir"`
}

func DefaultCsqttConfig() CsqttConfig {
	return CsqttConfig{ListenAddr: csqttDefaultListen}
}

func (c CsqttConfig) Merge() CsqttConfig {
	if c.ListenAddr == "" {
		c.ListenAddr = csqttDefaultListen
	}
	return c
}

func (c CsqttConfig) Validate() error {
	if _, port, err := net.SplitHostPort(c.ListenAddr); err != nil || port == "" {
		return errors.New("csqtt: listenAddr must be host:port (e.g. 0.0.0.0:46000)")
	}
	return nil
}

func (c CsqttConfig) EnsurePassword() (CsqttConfig, error) {
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

func (c CsqttConfig) EnsureWebPass() (CsqttConfig, error) {
	if strings.TrimSpace(c.WebPass) != "" {
		return c, nil
	}
	pass, err := generateQwdttPassword(24)
	if err != nil {
		return c, err
	}
	c.WebPass = pass
	return c, nil
}

func (c CsqttConfig) publicPort() int {
	if _, port, err := net.SplitHostPort(c.ListenAddr); err == nil {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			return p
		}
	}
	return 46000
}

func (c CsqttConfig) EnsureSubHost() CsqttConfig {
	if strings.TrimSpace(c.SubHost) != "" {
		return c
	}
	ip := detectOutboundIPv4()
	if ip == "" {
		return c
	}
	c.SubHost = ip
	return c
}

func (c CsqttConfig) WithPeerHost(host string) CsqttConfig {
	host = strings.TrimSpace(host)
	if host == "" {
		return c
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	c.SubHost = host
	return c
}

func (c CsqttConfig) ResolveConfigDir() string {
	fallback := DataDir(Csqtt)
	d := strings.TrimSpace(c.ConfigDir)
	if d == "" {
		return fallback
	}
	abs, err := filepath.Abs(d)
	if err != nil {
		return fallback
	}
	root, err := filepath.Abs(workDir())
	if err != nil {
		return fallback
	}
	sep := string(filepath.Separator)
	if abs == root || strings.HasPrefix(abs, root+sep) {
		return abs
	}
	return fallback
}

func (c CsqttConfig) BuildArgs() []string {
	args := []string{
		"--listen", c.ListenAddr,
		"--web-port", strconv.Itoa(csqttWebPort),
		"--config-dir", c.ResolveConfigDir(),
		"--web-user", csqttWebUser,
		"--dns", csqttDefaultDNS,
	}
	if p := strings.TrimSpace(c.Password); p != "" {
		args = append(args, "--password", p)
	}
	if w := strings.TrimSpace(c.WebPass); w != "" {
		args = append(args, "--web-pass", w)
	}
	return args
}

func (c CsqttConfig) shareHost() string {
	h := strings.TrimSpace(c.SubHost)
	if h == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	return h
}

func csqttHashList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, "+", ",")
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
		if len(out) == 6 {
			break
		}
	}
	return out
}

func (c CsqttConfig) ClientURI() string {
	host := c.shareHost()
	pass := strings.TrimSpace(c.Password)
	if host == "" || pass == "" {
		return ""
	}
	q := url.Values{}
	q.Set("v", "2")
	q.Set("host", host)
	q.Set("peer", strconv.Itoa(c.publicPort()))
	q.Set("password", pass)
	if hashes := csqttHashList(c.VkHashes); len(hashes) > 0 {
		q.Set("hashes", strings.Join(hashes, "+"))
	}
	return "csqtt://connect?" + q.Encode()
}

func CsqttListenPort(cfg CsqttConfig) int {
	return cfg.publicPort()
}

func (c CsqttConfig) String() string {
	return fmt.Sprintf("csqtt listen=%s", c.ListenAddr)
}
