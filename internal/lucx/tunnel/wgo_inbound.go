// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// wgosocksClientCap is the panel-side cap on socks5-users per inbound until
// the engine MAX_SOCKS5_USERS is verified on a live binary (spec §10.1).
const wgosocksClientCap = 512

// WgosocksClientLocalPort is the default loopback SOCKS5 port rendered into
// client configs (the client obfuscator listens here; apps point at it).
const WgosocksClientLocalPort = 1080

// WgosocksClient is one panel client of a wgosocks inbound: an RFC 1929
// login/password pair validated server-side by the engine (socks5-users).
type WgosocksClient struct {
	Email    string `json:"email"`
	Login    string `json:"login"`
	Password string `json:"password"`
	Enable   *bool  `json:"enable"`
}

func (c WgosocksClient) enabled() bool { return c.Enable == nil || *c.Enable }

// WgosocksSettings is the settings JSON of a wgosocks inbound. The public
// TCP listen port is inbound.Port (anytls precedent).
type WgosocksSettings struct {
	Key         string           `json:"key"`
	Masking     string           `json:"masking"`
	MediaSsrc   string           `json:"mediaSsrc"`
	MediaPt     int              `json:"mediaPt"`
	MediaClock  int              `json:"mediaClock"`
	MaxDummy    int              `json:"maxDummy"`
	IdleTimeout int              `json:"idleTimeout"`
	MaxClients  int              `json:"maxClients"`
	Clients     []WgosocksClient `json:"clients"`
}

// WgosocksConfigFromInbound parses a wgosocks inbound's settings. ok is
// false for nil/other-protocol inbounds and malformed JSON.
func WgosocksConfigFromInbound(ib *model.Inbound) (WgosocksSettings, bool) {
	var s WgosocksSettings
	if ib == nil || ib.Protocol != model.Wgosocks {
		return s, false
	}
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
		return s, false
	}
	return s, true
}

// ValidateWgosocks checks settings plus the listen port. reserved reports
// whether a port is taken by another inbound (proto "tcp"); may be nil.
func ValidateWgosocks(s WgosocksSettings, port int, reserved func(port int, proto string) bool) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("wgosocks: port must be 1..65535")
	}
	key := strings.TrimSpace(s.Key)
	if len(key) < 1 || len(key) > 255 {
		return fmt.Errorf("wgosocks: key must be 1..255 chars")
	}
	switch s.Masking {
	case WgoMaskNone, WgoMaskStun, WgoMaskMedia, WgoMaskTLS, WgoMaskAuto:
	default:
		return fmt.Errorf("wgosocks: unknown masking %q", s.Masking)
	}
	if s.MaxDummy < 0 || s.MaxDummy > 1024 {
		return fmt.Errorf("wgosocks: maxDummy must be 0..1024")
	}
	if s.IdleTimeout < 5 || s.IdleTimeout > 86400 {
		return fmt.Errorf("wgosocks: idleTimeout must be 5..86400 seconds")
	}
	if s.MaxClients < 1 || s.MaxClients > 1024 {
		return fmt.Errorf("wgosocks: maxClients must be 1..1024")
	}
	if s.MediaPt != 0 && (s.MediaPt < 96 || s.MediaPt > 127) {
		return fmt.Errorf("wgosocks: mediaPt must be 96..127")
	}
	if s.MediaClock != 0 && (s.MediaClock < 1 || s.MediaClock > 90000) {
		return fmt.Errorf("wgosocks: mediaClock must be 1..90000")
	}
	if len(s.Clients) > wgosocksClientCap {
		return fmt.Errorf("wgosocks: at most %d clients per inbound", wgosocksClientCap)
	}
	logins := map[string]bool{}
	emails := map[string]bool{}
	for i, c := range s.Clients {
		label := c.Email
		if label == "" {
			label = fmt.Sprintf("#%d", i+1)
		}
		if strings.TrimSpace(c.Email) == "" {
			return fmt.Errorf("wgosocks: client %s: email is required", label)
		}
		if emails[c.Email] {
			return fmt.Errorf("wgosocks: duplicate client email %s", c.Email)
		}
		emails[c.Email] = true
		if len(c.Login) < 1 || len(c.Login) > 255 || strings.TrimSpace(c.Login) != c.Login {
			return fmt.Errorf("wgosocks: client %s: login must be 1..255 chars, no padding", label)
		}
		if len(c.Password) < 1 || len(c.Password) > 255 {
			return fmt.Errorf("wgosocks: client %s: password must be 1..255 chars", label)
		}
		if logins[c.Login] {
			return fmt.Errorf("wgosocks: duplicate login %q", c.Login)
		}
		logins[c.Login] = true
	}
	if reserved != nil && reserved(port, "tcp") {
		return fmt.Errorf("wgosocks: TCP port %d already reserved", port)
	}
	return nil
}

// RenderWgosocksConf renders the engine INI for one wgosocks inbound:
// mode=socks5, role=server, one section, socks5-users from enabled clients
// (sorted by login — stable output, stable fingerprint).
func RenderWgosocksConf(s WgosocksSettings, port int) string {
	users := make([]WgosocksClient, 0, len(s.Clients))
	for _, c := range s.Clients {
		if c.enabled() {
			users = append(users, c)
		}
	}
	sort.Slice(users, func(a, b int) bool { return users[a].Login < users[b].Login })
	var b strings.Builder
	fmt.Fprintf(&b, "[wgosocks]\n")
	fmt.Fprintf(&b, "mode = socks5\n")
	fmt.Fprintf(&b, "role = server\n")
	fmt.Fprintf(&b, "source-lport = %d\n", port)
	fmt.Fprintf(&b, "key = %s\n", s.Key)
	fmt.Fprintf(&b, "masking = %s\n", s.Masking)
	if s.Masking == WgoMaskMedia {
		if v := strings.TrimSpace(s.MediaSsrc); v != "" {
			fmt.Fprintf(&b, "media-ssrc = %s\n", v)
		}
		if s.MediaPt != 0 {
			fmt.Fprintf(&b, "media-pt = %d\n", s.MediaPt)
		}
		if s.MediaClock != 0 {
			fmt.Fprintf(&b, "media-clock = %d\n", s.MediaClock)
		}
	}
	fmt.Fprintf(&b, "max-dummy = %d\n", s.MaxDummy)
	fmt.Fprintf(&b, "idle-timeout = %d\n", s.IdleTimeout*1000)
	fmt.Fprintf(&b, "verbose = 1\n")
	if len(users) > 0 {
		pairs := make([]string, 0, len(users))
		for _, u := range users {
			pairs = append(pairs, u.Login+":"+u.Password)
		}
		fmt.Fprintf(&b, "socks5-users = %s\n", strings.Join(pairs, ","))
	}
	return b.String()
}

// WgosocksInstanceFromInbound derives the supervised engine instance of one
// wgosocks inbound (TCP probe on the listen port).
func WgosocksInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	s, ok := WgosocksConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	return Instance{
		Core:       Wgosocks,
		Key:        WgosocksKey(ib.Id),
		Enabled:    ib.Enable,
		ConfigText: RenderWgosocksConf(s, ib.Port),
		ProbePort:  ib.Port,
	}, true
}

// WgosocksClientConf renders the client payload of one wgosocks client:
// [instance] (role=client) + separate [socks5] credentials — the engine
// binary must not see unknown keys, so login/password stay outside the
// instance section (spec §2.2). The client app reads [socks5] to build
// socks5://login:password@127.0.0.1:<source-lport>.
func WgosocksClientConf(s WgosocksSettings, port int, host string, c WgosocksClient) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[instance]\n")
	fmt.Fprintf(&b, "mode = socks5\n")
	fmt.Fprintf(&b, "role = client\n")
	fmt.Fprintf(&b, "source-if = 127.0.0.1\n")
	fmt.Fprintf(&b, "source-lport = %d\n", WgosocksClientLocalPort)
	fmt.Fprintf(&b, "target = %s:%d\n", host, port)
	fmt.Fprintf(&b, "key = %s\n", s.Key)
	fmt.Fprintf(&b, "masking = %s\n", s.Masking)
	fmt.Fprintf(&b, "verbose = INFO\n")
	if s.Masking == WgoMaskMedia {
		if v := strings.TrimSpace(s.MediaSsrc); v != "" {
			fmt.Fprintf(&b, "media-ssrc = %s\n", v)
		}
	}
	fmt.Fprintf(&b, "\n[socks5]\n")
	fmt.Fprintf(&b, "login = %s\n", c.Login)
	fmt.Fprintf(&b, "password = %s\n", c.Password)
	return b.String()
}
