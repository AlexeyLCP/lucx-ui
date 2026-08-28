// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// AwgTimer is the stored form of an AWG3 device-level timer/padding value.
// The kernel's u16_range_t (device.h) and the tools' u16_range_from_string
// accept BOTH a single integer ("150") and an inclusive range ("100-500"),
// randomizing within the range at rekey just like H1-H4 — so the value is kept
// as a string end-to-end and written to the .conf verbatim. UnmarshalJSON
// tolerates a JSON number as well (legacy inbounds / panel defaults store 0 as
// a number), normalizing it to its string form.
type AwgTimer string

var awgTimerPat = regexp.MustCompile(`^[0-9]+(-[0-9]+)?$`)

func (t *AwgTimer) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*t = ""
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		s = strings.TrimSpace(str)
	}
	if s != "" && !awgTimerPat.MatchString(s) {
		*t = ""
		return nil
	}
	*t = AwgTimer(s)
	return nil
}

func confValue(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == 0 {
			return -1
		}
		return r
	}, v)
}

// IsZero reports whether the value is empty or a zero (the kernel built-in
// WireGuard default), in which case the renderer omits the line.
func (t AwgTimer) IsZero() bool {
	s := strings.TrimSpace(string(t))
	return s == "" || s == "0" || s == "0-0"
}

// Instance is the desired runtime configuration of one AWG inbound: the kernel
// interface, its obfuscation parameters, and the set of peers that should be
// present. The manager drives the running kernel interface toward this state.
type Instance struct {
	Id         int
	Tag        string
	Listen     string
	Port       int
	Ifname     string // e.g. "awg1"
	MTU        int
	DNS        string
	Address    string // server tunnel address, e.g. "10.8.0.1/24"; written to [Interface].Address
	PrivateKey string
	// Obfuscation (matches AWGParams fields; sourced from inbound.Settings).
	Jc   int
	Jmin int
	Jmax int
	S1   int
	S2   int
	S3   int
	S4   int
	H1   string
	H2   string
	H3   string
	H4   string
	I1   string
	I2   string
	I3   string
	I4   string
	I5   string
	// HeaderProtectionKey is the AWG3 (AmneziaWG 3) 32-byte ChaCha20 header
	// protection key, base64-encoded (same shape as a WireGuard private key).
	// Upstream kernel module v3.0.20260731 + tools v3.0.20260730 parse the
	// field in setconf. The .conf renderer writes it only when IsAwg3Plus
	// (the inbound opts into AWG3); for older versions it stays empty and
	// is omitted so v1/v2 kernels keep accepting the config.
	HeaderProtectionKey string
	// AWG3 device-level timers/padding. Stored as AwgTimer (a string that may
	// be a single value "150" or an inclusive range "100-500" — the kernel
	// u16_range_t accepts both and randomizes within a range at rekey). Empty /
	// "0" = kernel built-in WG constant; the renderer omits it. Written to the
	// .conf only when non-zero AND IsAwg3Plus.
	ContentPaddingAddition AwgTimer
	RekeyAfterTime         AwgTimer
	RekeyTimeout           AwgTimer
	RejectAfterTime        AwgTimer
	KeepaliveTimeout       AwgTimer
	MaxHandshakeAttempts   AwgTimer
	// AWG 3.1 device flags. Written only when AwgVersion == "3.1" AND
	// ModuleSupportsAwg31(); omitted when false so v3.0 tools keep accepting
	// the config (they reject "Line unrecognized: RandomTrailers=...").
	RandomTrailers bool
	DisableCookies bool
	// AwgVersion is the AmneziaWG protocol version this inbound targets:
	// "1.5" (legacy, Jc/Jmin/Jmax + S1/S2 + H1-H4 only), "2" (adds S3/S4 +
	// optional I1-I5, Android 2.0.1), "3" (adds HeaderProtectionKey,
	// desktop 5.0.0.5 / Android 3.0.1), or "3.1" (adds RandomTrailers /
	// DisableCookies, module+tools v3.1.20260812). The server .conf is
	// generated for this version; client configs may be exported at the same
	// version or lower. Defaults to "2" when empty (pre-lucx.50 inbounds).
	AwgVersion string
	// Peers expected on the interface. Each entry maps to one [Peer] in the
	// generated .conf and is reconciled against the kernel state.
	Peers []PeerSpec
	// RouteThroughXray, when set, tells the Xray config builder to inject a
	// TUN inbound for this AWG interface so decrypted packets flow through
	// Xray's routing rules. Mirrors mtproto's RouteThroughXray.
	RouteThroughXray bool
	OutboundTag      string
}

// PeerSpec is one desired peer on an AWG interface.
type PeerSpec struct {
	PrivateKey     string // client Curve25519 private key (stored so we can render a full client .conf/share-link, mirroring WireGuard)
	PublicKey      string // client Curve25519 public key (stored as Client.ID / clients[].publicKey)
	PSK            string // PresharedKey (stored as Client.Password / clients[].preSharedKey)
	Keepalive      AwgTimer
	AllowedIPs     string
	Email          string
	ForwardedPorts string
}

// fingerprint changes whenever a device-level .conf field changes, so
// ensureLocked restarts awg-quick. Peers are NOT included — adding/removing a
// client uses awg syncconf (SyncPeers) so existing handshakes survive.
// DNS and I1-I5 are client-export-only and do not go into the server .conf.
func (inst Instance) fingerprint() string {
	parts := []string{
		inst.Ifname,
		strconv.Itoa(inst.Port),
		inst.PrivateKey,
		strconv.Itoa(inst.MTU),
		inst.Address,
		strconv.Itoa(inst.Jc),
		strconv.Itoa(inst.Jmin),
		strconv.Itoa(inst.Jmax),
		strconv.Itoa(inst.S1),
		strconv.Itoa(inst.S2),
		strconv.Itoa(inst.S3),
		strconv.Itoa(inst.S4),
		inst.H1,
		inst.H2,
		inst.H3,
		inst.H4,
		inst.HeaderProtectionKey,
		string(inst.ContentPaddingAddition),
		string(inst.RekeyAfterTime),
		string(inst.RekeyTimeout),
		string(inst.RejectAfterTime),
		string(inst.KeepaliveTimeout),
		string(inst.MaxHandshakeAttempts),
		strconv.FormatBool(inst.RandomTrailers),
		strconv.FormatBool(inst.DisableCookies),
		inst.AwgVersion,
		strconv.FormatBool(inst.RouteThroughXray),
		inst.OutboundTag,
	}
	return strings.Join(parts, "|")
}

func (inst Instance) peerFingerprint() string {
	parts := make([]string, 0, len(inst.Peers)*4)
	for _, p := range inst.Peers {
		parts = append(parts, p.PrivateKey, p.PublicKey, p.PSK, p.AllowedIPs)
	}
	return strings.Join(parts, "|")
}

// InstanceFromInbound derives a desired Instance from an AWG inbound. Returns
// false when the inbound is not a usable AWG inbound (wrong protocol, missing
// server key, etc.).
func InstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	if ib == nil || ib.Protocol != model.AWG {
		return Instance{}, false
	}
	var s struct {
		PrivateKey          string `json:"privateKey"`
		MTU                 int    `json:"mtu"`
		DNS                 string `json:"dns"`
		Address             string `json:"address"`
		Jc                  int    `json:"jc"`
		Jmin                int    `json:"jmin"`
		Jmax                int    `json:"jmax"`
		S1                  int    `json:"s1"`
		S2                  int    `json:"s2"`
		S3                  int    `json:"s3"`
		S4                  int    `json:"s4"`
		H1                  string `json:"h1"`
		H2                  string `json:"h2"`
		H3                  string `json:"h3"`
		H4                  string `json:"h4"`
		I1                  string `json:"i1"`
		I2                  string `json:"i2"`
		I3                  string `json:"i3"`
		I4                  string `json:"i4"`
		I5                  string `json:"i5"`
		HeaderProtectionKey string `json:"headerProtectionKey"`
		AwgVersion          string `json:"awgVersion"`
		RouteThroughXray    bool   `json:"routeThroughXray"`
		OutboundTag         string `json:"outboundTag"`
		Clients             []struct {
			PublicKey      string   `json:"publicKey"`
			PrivateKey     string   `json:"privateKey"`
			PreSharedKey   string   `json:"preSharedKey"`
			AllowedIPs     []string `json:"allowedIPs"`
			KeepAlive      AwgTimer `json:"keepAlive"`
			Email          string   `json:"email"`
			ForwardedPorts string   `json:"forwardedPorts"`
			ID             string   `json:"id"`
			Password       string   `json:"password"`
			Enable         *bool    `json:"enable"`
		} `json:"clients"`
		// AWG3 device-level timers/padding. AwgTimer unmarshals a JSON number
		// (legacy) or a string ("150" / "100-500" range) so native kernel ranges
		// pass through untouched.
		ContentPaddingAddition AwgTimer `json:"contentPaddingAddition"`
		RekeyAfterTime         AwgTimer `json:"rekeyAfterTime"`
		RekeyTimeout           AwgTimer `json:"rekeyTimeout"`
		RejectAfterTime        AwgTimer `json:"rejectAfterTime"`
		KeepaliveTimeout       AwgTimer `json:"keepaliveTimeout"`
		MaxHandshakeAttempts   AwgTimer `json:"maxHandshakeAttempts"`
		RandomTrailers         bool     `json:"randomTrailers"`
		DisableCookies         bool     `json:"disableCookies"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
		return Instance{}, false
	}
	if s.PrivateKey == "" {
		return Instance{}, false
	}
	inst := Instance{
		Id:     ib.Id,
		Tag:    ib.Tag,
		Listen: ib.Listen,
		Port:   ib.Port,
		Ifname: ifnameFor(ib.Id),
		// 1420 = 1500 (typical Ethernet) minus WireGuard/AWG overhead, the
		// throughput-optimal fallback on a normal VPS. Only used when an
		// inbound's settings JSON omits mtu entirely (pre-lucx MTU field,
		// or hand-crafted JSON); the panel form always sends an explicit
		// value (1420 default, operator can drop to 1320 for mobile/CGNAT).
		MTU:                    orDefault(s.MTU, 1420),
		DNS:                    s.DNS,
		Address:                s.Address,
		PrivateKey:             s.PrivateKey,
		Jc:                     s.Jc,
		Jmin:                   s.Jmin,
		Jmax:                   s.Jmax,
		S1:                     s.S1,
		S2:                     s.S2,
		S3:                     s.S3,
		S4:                     s.S4,
		H1:                     s.H1,
		H2:                     s.H2,
		H3:                     s.H3,
		H4:                     s.H4,
		I1:                     s.I1,
		I2:                     s.I2,
		I3:                     s.I3,
		I4:                     s.I4,
		I5:                     s.I5,
		HeaderProtectionKey:    s.HeaderProtectionKey,
		AwgVersion:             NormalizeAWGVersion(s.AwgVersion),
		RouteThroughXray:       s.RouteThroughXray,
		OutboundTag:            s.OutboundTag,
		ContentPaddingAddition: s.ContentPaddingAddition,
		RekeyAfterTime:         s.RekeyAfterTime,
		RekeyTimeout:           s.RekeyTimeout,
		RejectAfterTime:        s.RejectAfterTime,
		KeepaliveTimeout:       s.KeepaliveTimeout,
		MaxHandshakeAttempts:   s.MaxHandshakeAttempts,
		RandomTrailers:         s.RandomTrailers,
		DisableCookies:         s.DisableCookies,
	}
	for _, c := range s.Clients {
		// Skip disabled clients. enable is a pointer so we can distinguish
		// absent (treat as enabled, for legacy inbounds) from explicit false.
		if c.Enable != nil && !*c.Enable {
			continue
		}
		pub := strings.TrimSpace(c.PublicKey)
		psk := strings.TrimSpace(c.PreSharedKey)
		if pub == "" {
			pub = strings.TrimSpace(c.ID) // legacy field
		}
		// Form clients always carry password (16-char NumLower). That is NOT
		// a PSK — awg syncconf then dies: "Key is not the correct length".
		// Legacy AWG stored the real PSK in password only as the id/password
		// pair (no publicKey).
		if psk == "" && strings.TrimSpace(c.PublicKey) == "" {
			psk = strings.TrimSpace(c.Password)
		}
		if pub == "" {
			continue
		}
		allowed := "0.0.0.0/0, ::/0"
		if len(c.AllowedIPs) > 0 && c.AllowedIPs[0] != "" {
			allowed = strings.Join(c.AllowedIPs, ", ")
		}
		inst.Peers = append(inst.Peers, PeerSpec{
			PrivateKey:     c.PrivateKey,
			PublicKey:      pub,
			PSK:            psk,
			Keepalive:      c.KeepAlive,
			AllowedIPs:     allowed,
			Email:          c.Email,
			ForwardedPorts: c.ForwardedPorts,
		})
	}
	return inst, true
}

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// NormalizeAWGVersion canonicalizes the stored awgVersion: "1.5"/"2"/"3"/"3.1"
// pass through, anything else (including "" for pre-lucx.50 inbounds) falls
// back to "2". Version "2" is the safe default — it matches what every
// shipped LucX-UI release before AWG3 targeted, emits no HeaderProtectionKey,
// and is accepted by the current kernel module without S-range constraints.
// Exported so the web/service layer (inboundAwgHints) shares the single rule.
func NormalizeAWGVersion(v string) string {
	switch v {
	case "1.5", "2", "3", "3.1":
		return v
	default:
		return "2"
	}
}

// CollapseTimerForVersion returns a keepalive/timer string safe for the given
// AWG protocol version. Pre-v3 tools parse PersistentKeepalive as a single
// integer and reject "15-25". Empty/zero → "".
func CollapseTimerForVersion(raw, version string) string {
	s := strings.TrimSpace(raw)
	if s == "" || s == "0" || s == "0-0" {
		return ""
	}
	if !IsAwg3Plus(version) {
		if i := strings.IndexByte(s, '-'); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	return s
}

var awgHFieldRe = regexp.MustCompile(`^[0-9]+(-[0-9]+)?$`)

// ValidateObfuscationFields rejects garbage H1-H4 and range-form H on a 1.5
// inbound (v1.x awg-quick: "Unable to parse H1"). Empty H is allowed (generator
// always fills them; blank means the operator has not generated yet).
func ValidateObfuscationFields(version, h1, h2, h3, h4 string) error {
	ver := NormalizeAWGVersion(version)
	for i, h := range []string{h1, h2, h3, h4} {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if !awgHFieldRe.MatchString(h) {
			return fmt.Errorf("awg: H%d is not an integer or lo-hi range", i+1)
		}
		if ver == "1.5" && strings.Contains(h, "-") {
			return fmt.Errorf("awg: H%d is a range but awgVersion is 1.5 — regenerate obfuscation", i+1)
		}
	}
	return nil
}

func timerHi(t AwgTimer) int64 {
	s := strings.TrimSpace(string(t))
	if s == "" || s == "0" || s == "0-0" {
		return 0
	}
	if i := strings.LastIndexByte(s, '-'); i >= 0 {
		s = strings.TrimSpace(s[i+1:])
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func onlineTTLSeconds(inst Instance) int64 {
	ttl := int64(handshakeOnlineTTL)
	if hi := timerHi(inst.RekeyAfterTime); hi+60 > ttl {
		ttl = hi + 60
	}
	return ttl
}

// IsAwg3Plus reports whether v includes the AWG3 field set (HPK + device
// timers). "3.1" is a superset of "3".
func IsAwg3Plus(v string) bool {
	switch NormalizeAWGVersion(v) {
	case "3", "3.1":
		return true
	default:
		return false
	}
}

// IsAwg31 reports whether v is exactly the 3.1 ceiling (RandomTrailers /
// DisableCookies). A v3 inbound must not emit those lines.
func IsAwg31(v string) bool {
	return NormalizeAWGVersion(v) == "3.1"
}

// parseAwgToolsVersion extracts major.minor from an `awg version` banner
// ("amneziawg-tools v3.1.20260812 - https://amnezia.org" → 3, 1). Returns
// -1, -1 when no v<digits> token exists. Missing minor is treated as 0
// ("v3" → 3, 0).
func parseAwgToolsVersion(s string) (major, minor int) {
	for i := 0; i+1 < len(s); i++ {
		if s[i] != 'v' || s[i+1] < '0' || s[i+1] > '9' {
			continue
		}
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		maj, err := strconv.Atoi(s[i+1 : j])
		if err != nil {
			return -1, -1
		}
		min := 0
		if j < len(s) && s[j] == '.' {
			k := j + 1
			for k < len(s) && s[k] >= '0' && s[k] <= '9' {
				k++
			}
			if k > j+1 {
				min, _ = strconv.Atoi(s[j+1 : k])
			}
		}
		return maj, min
	}
	return -1, -1
}

func awgToolsAtLeast(s string, wantMajor, wantMinor int) bool {
	maj, min := parseAwgToolsVersion(s)
	if maj < 0 {
		return false
	}
	if maj != wantMajor {
		return maj > wantMajor
	}
	return min >= wantMinor
}

// ifnameFor returns the canonical AWG interface name for an inbound id.
// Linux limits interface names to 15 chars; "awg" + id fits for id < 10^12.
func ifnameFor(id int) string {
	return "awg" + strconv.Itoa(id)
}
