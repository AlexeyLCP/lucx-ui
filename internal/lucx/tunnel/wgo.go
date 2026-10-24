// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// wgobfs engine: Ground-Zerro/Phobos src/phobos-obfuscator (GPL-3.0 external
// binary, derived from ClusterM/wg-obfuscator). One binary serves both cores:
// Wgo (WireGuard transport obfuscation presets on a kernel-WG inbound) and
// Wgosocks (SOCKS5-exit inbound). Config/link formats per
// docs/superpowers/specs/2026-10-24-wg-obfuscator-design.md — no Phobos panel
// (AGPL) code is copied, only the binary and the documented formats.

// Masking modes accepted by the engine's `masking` directive.
const (
	WgoMaskNone  = "NONE"
	WgoMaskStun  = "STUN"
	WgoMaskMedia = "MEDIA"
	WgoMaskTLS   = "TLS" // socks5 mode only
	WgoMaskAuto  = "AUTO"
)

// WgoPreset is one obfuscation listener in front of a kernel WireGuard
// inbound: its own public UDP port, XOR key and masking mode (Phobos preset).
type WgoPreset struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ExtPort           int    `json:"extPort"`
	Key               string `json:"key"`
	Masking           string `json:"masking"`
	MediaSsrc         string `json:"mediaSsrc"`
	MediaPt           int    `json:"mediaPt"`
	MediaClock        int    `json:"mediaClock"`
	MaxDummy          int    `json:"maxDummy"`
	IdleTimeout       int    `json:"idleTimeout"` // seconds; rendered ms server-side
	ClientWgLocalPort int    `json:"clientWgLocalPort"`
	IsDefault         bool   `json:"isDefault"`
}

// WgoObfuscation is the optional `obfuscation` block of a kernel WireGuard
// inbound settings JSON. Absent on vanilla inbounds — feature hidden.
type WgoObfuscation struct {
	Enabled         bool        `json:"enabled"`
	BlockDirectPort bool        `json:"blockDirectPort"`
	Presets         []WgoPreset `json:"presets"`
}

// ParseObfuscation reads the obfuscation block from a kernel WireGuard
// inbound. ok is false when the key is absent or not valid JSON — callers
// treat that as "feature off" and must not error (Rule 0b overlay).
func ParseObfuscation(ib *model.Inbound) (WgoObfuscation, bool) {
	var o WgoObfuscation
	if ib == nil {
		return o, false
	}
	var s struct {
		Obfuscation *WgoObfuscation `json:"obfuscation"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil || s.Obfuscation == nil {
		return o, false
	}
	return *s.Obfuscation, true
}

// ValidateObfuscation checks a parsed obfuscation block. wgPort is the kernel
// WireGuard listen port (presets must not collide with it); reserved reports
// whether a port is already taken by another inbound (proto "udp"/"tcp") and
// may be nil in tests.
func ValidateObfuscation(o WgoObfuscation, wgPort int, reserved func(port int, proto string) bool) error {
	if !o.Enabled {
		return nil
	}
	if len(o.Presets) == 0 {
		return fmt.Errorf("wgo: at least one preset is required")
	}
	seenPort := map[int]string{}
	seenName := map[string]bool{}
	defaults := 0
	for i, p := range o.Presets {
		label := p.Name
		if label == "" {
			label = fmt.Sprintf("preset #%d", i+1)
		}
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("wgo: preset %s: name is required", label)
		}
		if p.ExtPort < 1 || p.ExtPort > 65535 {
			return fmt.Errorf("wgo: preset %s: extPort must be 1..65535", label)
		}
		if p.ExtPort == wgPort {
			return fmt.Errorf("wgo: preset %s: extPort must differ from the WireGuard port", label)
		}
		if other, dup := seenPort[p.ExtPort]; dup {
			return fmt.Errorf("wgo: preset %s: port %d already used by preset %s", label, p.ExtPort, other)
		}
		seenPort[p.ExtPort] = label
		if sectionName(p, i) == "" || seenName[sectionName(p, i)] {
			return fmt.Errorf("wgo: preset %s: duplicate section name", label)
		}
		seenName[sectionName(p, i)] = true
		key := strings.TrimSpace(p.Key)
		if len(key) < 1 || len(key) > 255 {
			return fmt.Errorf("wgo: preset %s: key must be 1..255 chars", label)
		}
		switch p.Masking {
		case WgoMaskNone, WgoMaskStun, WgoMaskMedia, WgoMaskAuto:
		case WgoMaskTLS:
			return fmt.Errorf("wgo: preset %s: TLS masking is socks5-only", label)
		default:
			return fmt.Errorf("wgo: preset %s: unknown masking %q", label, p.Masking)
		}
		if p.MaxDummy < 0 || p.MaxDummy > 1024 {
			return fmt.Errorf("wgo: preset %s: maxDummy must be 0..1024", label)
		}
		if p.IdleTimeout < 5 || p.IdleTimeout > 86400 {
			return fmt.Errorf("wgo: preset %s: idleTimeout must be 5..86400 seconds", label)
		}
		if p.ClientWgLocalPort < 1 || p.ClientWgLocalPort > 65535 {
			return fmt.Errorf("wgo: preset %s: clientWgLocalPort must be 1..65535", label)
		}
		if p.MediaPt != 0 && (p.MediaPt < 96 || p.MediaPt > 127) {
			return fmt.Errorf("wgo: preset %s: mediaPt must be 96..127", label)
		}
		if p.MediaClock != 0 && (p.MediaClock < 1 || p.MediaClock > 90000) {
			return fmt.Errorf("wgo: preset %s: mediaClock must be 1..90000", label)
		}
		if p.IsDefault {
			defaults++
		}
		if reserved != nil && reserved(p.ExtPort, "udp") {
			return fmt.Errorf("wgo: preset %s: UDP port %d already reserved", label, p.ExtPort)
		}
	}
	if defaults != 1 {
		return fmt.Errorf("wgo: exactly one preset must be default")
	}
	return nil
}

// sectionName sanitizes a preset name into an INI section identifier
// ([a-z0-9-], lowercase; disallowed runs collapse to one hyphen, trimmed).
// "presetN" when nothing usable remains; index i keeps fallbacks unique.
func sectionName(p WgoPreset, i int) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(p.Name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return fmt.Sprintf("preset%d", i+1)
	}
	return name
}

// RenderWgoConf renders the wgobfs INI config for one kernel WireGuard
// inbound: a section per preset, all forwarding to the loopback WireGuard
// port. Sections are emitted sorted by name so the output is stable.
func RenderWgoConf(o WgoObfuscation, wgPort int) string {
	type named struct {
		name   string
		preset WgoPreset
	}
	items := make([]named, 0, len(o.Presets))
	for i, p := range o.Presets {
		items = append(items, named{sectionName(p, i), p})
	}
	sort.Slice(items, func(a, b int) bool { return items[a].name < items[b].name })
	var b strings.Builder
	for _, it := range items {
		p := it.preset
		fmt.Fprintf(&b, "[%s]\n", it.name)
		fmt.Fprintf(&b, "mode = wireguard\n")
		fmt.Fprintf(&b, "source-lport = %d\n", p.ExtPort)
		fmt.Fprintf(&b, "target = 127.0.0.1:%d\n", wgPort)
		fmt.Fprintf(&b, "key = %s\n", p.Key)
		fmt.Fprintf(&b, "masking = %s\n", p.Masking)
		if p.Masking == WgoMaskMedia {
			if s := strings.TrimSpace(p.MediaSsrc); s != "" {
				fmt.Fprintf(&b, "media-ssrc = %s\n", s)
			}
			if p.MediaPt != 0 {
				fmt.Fprintf(&b, "media-pt = %d\n", p.MediaPt)
			}
			if p.MediaClock != 0 {
				fmt.Fprintf(&b, "media-clock = %d\n", p.MediaClock)
			}
		}
		fmt.Fprintf(&b, "max-dummy = %d\n", p.MaxDummy)
		fmt.Fprintf(&b, "idle-timeout = %d\n", p.IdleTimeout*1000)
		fmt.Fprintf(&b, "verbose = 1\n")
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

// InstanceSection renders the client-side [instance] block for one preset —
// the Phobos client config convention (spec §2.2): the client obfuscator
// listens on loopback clientWgLocalPort and forwards to host:extPort.
// idle-timeout stays in seconds here (client parser), unlike the server conf.
func InstanceSection(p WgoPreset, host string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[instance]\n")
	fmt.Fprintf(&b, "source-if = 127.0.0.1\n")
	fmt.Fprintf(&b, "source-lport = %d\n", p.ClientWgLocalPort)
	fmt.Fprintf(&b, "target = %s:%d\n", host, p.ExtPort)
	fmt.Fprintf(&b, "key = %s\n", p.Key)
	fmt.Fprintf(&b, "masking = %s\n", p.Masking)
	fmt.Fprintf(&b, "verbose = INFO\n")
	fmt.Fprintf(&b, "idle-timeout = %d\n", p.IdleTimeout)
	fmt.Fprintf(&b, "max-dummy = %d\n", p.MaxDummy)
	if p.Masking == WgoMaskMedia {
		if s := strings.TrimSpace(p.MediaSsrc); s != "" {
			fmt.Fprintf(&b, "media-ssrc = %s\n", s)
		}
	}
	return b.String()
}

// PhobosLink wraps a full client .conf into the phobos:// share URI
// (base64url, no padding, URL-encoded client name fragment).
func PhobosLink(conf, name string) string {
	return "phobos://" + base64.RawURLEncoding.EncodeToString([]byte(conf)) + "#" + escapeURIComponent(name)
}

// escapeURIComponent mirrors JS encodeURIComponent (percent-encode everything
// outside A-Za-z0-9 - _ . ! ~ * ' ( )). url.QueryEscape is not equivalent
// (it encodes spaces as '+').
func escapeURIComponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == '!', r == '~', r == '*', r == '\'', r == '(', r == ')':
			b.WriteRune(r)
		default:
			for _, byteVal := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", byteVal)
			}
		}
	}
	return b.String()
}

// WgoKey is the manager key of the engine process for one kernel WG inbound.
func WgoKey(inboundID int) string { return "wgo-" + strconv.Itoa(inboundID) }
