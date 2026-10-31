// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type TelemtConfig struct {
	Remark         string
	Enabled        bool
	Port           int
	TlsDomain      string
	Listen         string
	UseMiddleProxy bool
	Mask           bool
	FastMode       bool
	Users          []TelemtUser
}

type TelemtUser struct {
	Name   string
	Secret string
	Enable bool
}

func TelemtKey(id int) string { return "telemt-" + strconv.Itoa(id) }

type telemtSettings struct {
	TlsDomain      string `json:"tlsDomain"`
	Listen         string `json:"listen"`
	UseMiddleProxy *bool  `json:"useMiddleProxy"`
	Mask           *bool  `json:"mask"`
	FastMode       *bool  `json:"fastMode"`
}

func TelemtConfigFromInbound(ib *model.Inbound) (TelemtConfig, bool) {
	if ib == nil || ib.Protocol != model.Telemt {
		return TelemtConfig{}, false
	}
	s := telemtSettings{}
	_ = json.Unmarshal([]byte(ib.Settings), &s)
	cfg := TelemtConfig{
		Remark:         ib.Remark,
		Enabled:        ib.Enable,
		Port:           ib.Port,
		TlsDomain:      strings.TrimSpace(s.TlsDomain),
		Listen:         strings.TrimSpace(s.Listen),
		UseMiddleProxy: true,
		Mask:           true,
		FastMode:       true,
	}
	if s.UseMiddleProxy != nil {
		cfg.UseMiddleProxy = *s.UseMiddleProxy
	}
	if s.Mask != nil {
		cfg.Mask = *s.Mask
	}
	if s.FastMode != nil {
		cfg.FastMode = *s.FastMode
	}
	if cfg.Listen == "" {
		cfg.Listen = "0.0.0.0"
	}
	return cfg, true
}

func TelemtInstancesFromInbound(ib *model.Inbound) ([]Instance, bool) {
	cfg, ok := TelemtConfigFromInbound(ib)
	if !ok {
		return nil, false
	}
	inst := Instance{Core: Telemt, Key: TelemtKey(ib.Id), Enabled: false}
	if !cfg.Enabled || cfg.Port < 1 || cfg.TlsDomain == "" {
		return []Instance{inst}, true
	}
	var parsed struct {
		Clients []struct {
			Email  string `json:"email"`
			Secret string `json:"secret"`
			Enable bool   `json:"enable"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		return []Instance{inst}, true
	}
	for _, client := range parsed.Clients {
		if client.Enable && strings.TrimSpace(client.Email) != "" && strings.TrimSpace(client.Secret) != "" {
			cfg.Users = append(cfg.Users, TelemtUser{Name: client.Email, Secret: client.Secret, Enable: true})
		}
	}
	if len(cfg.Users) == 0 {
		return []Instance{inst}, true
	}
	return []Instance{{
		Core:       Telemt,
		Key:        TelemtKey(ib.Id),
		Enabled:    true,
		ConfigText: cfg.Render(),
		ProbePort:  cfg.Port,
	}}, true
}

func (c TelemtConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("telemt: port must be in 1..65535")
	}
	if strings.TrimSpace(c.TlsDomain) == "" || strings.ContainsAny(c.TlsDomain, " /:#@") {
		return errors.New("telemt: tlsDomain must be a bare hostname")
	}
	for _, user := range c.Users {
		if strings.TrimSpace(user.Name) == "" || strings.TrimSpace(user.Secret) == "" {
			return errors.New("telemt: user name and secret are required")
		}
	}
	return nil
}

func (c TelemtConfig) Render() string {
	if err := c.Validate(); err != nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[general]\nuse_middle_proxy = %t\nfast_mode = %t\n\n", c.UseMiddleProxy, c.FastMode)
	fmt.Fprintf(&b, "[general.modes]\nclassic = false\nsecure = false\ntls = true\n\n")
	fmt.Fprintf(&b, "[general.links]\nshow = \"*\"\npublic_port = %d\n\n", c.Port)
	fmt.Fprintf(&b, "[server]\nport = %d\n", c.Port)
	if c.Listen != "" && c.Listen != "0.0.0.0" {
		b.WriteString("\n[[server.listeners]]\nip = ")
		fmt.Fprintf(&b, "%q\n", c.Listen)
	}
	fmt.Fprintf(&b, "\n[censorship]\ntls_domain = %q\nmask = %t\n\n[access.users]\n", c.TlsDomain, c.Mask)
	for _, user := range c.Users {
		if user.Enable {
			fmt.Fprintf(&b, "%s = %q\n", telemtTomlKey(user.Name), user.Secret)
		}
	}
	return b.String()
}

func telemtTomlKey(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
