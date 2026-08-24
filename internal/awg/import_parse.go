// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strconv"
	"strings"

	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

const (
	ImportSourceMulti   = "awg-multi"
	ImportSourceToolza3 = "awg-toolza3"
	ImportSourceDocker  = "amnezia-docker"
	ImportSourceConf    = "awg-conf"
)

const suspendIP = "127.0.0.2"

// ServerConf is one server-side AmneziaWG/WireGuard .conf (interface + peers).
type ServerConf struct {
	PrivateKey             string
	Address                string
	ListenPort             int
	MTU                    int
	DNS                    string
	Jc, Jmin, Jmax         int
	S1, S2, S3, S4         int
	H1, H2, H3, H4         string
	I1, I2, I3, I4, I5     string
	HeaderProtectionKey    string
	ContentPaddingAddition AwgTimer
	RekeyAfterTime         AwgTimer
	RekeyTimeout           AwgTimer
	RejectAfterTime        AwgTimer
	KeepaliveTimeout       AwgTimer
	MaxHandshakeAttempts   AwgTimer
	RandomTrailers         bool
	DisableCookies         bool
	AwgVersion             string
	Peers                  []ServerPeer
}

// ServerPeer is one [Peer] block from a server .conf.
type ServerPeer struct {
	Name       string
	PublicKey  string
	PSK        string
	AllowedIPs string
	Keepalive  AwgTimer
	Suspended  bool
	OrigIPs    string
	Expiry     int64
}

// ClientKeyFile is a recovered client private key matched by public key.
type ClientKeyFile struct {
	PrivateKey         string
	Path               string
	DNS                string
	I1, I2, I3, I4, I5 string
	Name               string
	AllowedIPs         string
}

// ParseServerConf reads a server awg-quick .conf. Comments immediately inside
// a [Peer] block become the peer name (toolza `# name` / `# expires=`).
func ParseServerConf(text string) ServerConf {
	var s ServerConf
	section := ""
	var peer ServerPeer
	peerNameSet := false
	flushPeer := func() {
		if peer.PublicKey == "" {
			peer = ServerPeer{}
			peerNameSet = false
			return
		}
		if strings.Contains(peer.AllowedIPs, suspendIP) {
			peer.Suspended = true
		}
		s.Peers = append(s.Peers, peer)
		peer = ServerPeer{}
		peerNameSet = false
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if section == "peer" {
				flushPeer()
			}
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			if section != "peer" {
				continue
			}
			comment := strings.TrimSpace(strings.TrimLeft(line, "#;"))
			if comment == "" || strings.EqualFold(comment, strings.TrimPrefix(xuiManagedMarker, "# ")) {
				continue
			}
			if strings.HasPrefix(comment, "expires=") {
				if ts, err := strconv.ParseInt(strings.TrimPrefix(comment, "expires="), 10, 64); err == nil {
					peer.Expiry = ts
				}
				continue
			}
			if strings.HasPrefix(comment, "orig_ips=") {
				peer.OrigIPs = strings.TrimSpace(strings.TrimPrefix(comment, "orig_ips="))
				continue
			}
			if !peerNameSet {
				peer.Name = comment
				peerNameSet = true
			}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch section {
		case "interface":
			applyInterfaceKey(&s, key, val)
		case "peer":
			switch key {
			case "PublicKey":
				peer.PublicKey = val
			case "PresharedKey":
				peer.PSK = val
			case "AllowedIPs":
				peer.AllowedIPs = val
			case "PersistentKeepalive":
				peer.Keepalive = AwgTimer(val)
			}
		}
	}
	if section == "peer" {
		flushPeer()
	}
	s.AwgVersion = detectAwgVersion(s)
	if i := strings.Index(s.Address, ","); i >= 0 {
		s.Address = strings.TrimSpace(s.Address[:i])
	}
	return s
}

func applyInterfaceKey(s *ServerConf, key, val string) {
	switch key {
	case "PrivateKey":
		s.PrivateKey = val
	case "Address":
		s.Address = val
	case "ListenPort":
		s.ListenPort, _ = strconv.Atoi(val)
	case "MTU":
		s.MTU, _ = strconv.Atoi(val)
	case "DNS":
		if first, _, comma := strings.Cut(val, ","); comma {
			s.DNS = strings.TrimSpace(first)
		} else {
			s.DNS = val
		}
	case "Jc":
		s.Jc, _ = strconv.Atoi(val)
	case "Jmin":
		s.Jmin, _ = strconv.Atoi(val)
	case "Jmax":
		s.Jmax, _ = strconv.Atoi(val)
	case "S1":
		s.S1, _ = strconv.Atoi(val)
	case "S2":
		s.S2, _ = strconv.Atoi(val)
	case "S3":
		s.S3, _ = strconv.Atoi(val)
	case "S4":
		s.S4, _ = strconv.Atoi(val)
	case "H1":
		s.H1 = val
	case "H2":
		s.H2 = val
	case "H3":
		s.H3 = val
	case "H4":
		s.H4 = val
	case "I1":
		s.I1 = val
	case "I2":
		s.I2 = val
	case "I3":
		s.I3 = val
	case "I4":
		s.I4 = val
	case "I5":
		s.I5 = val
	case "HeaderProtectionKey":
		s.HeaderProtectionKey = val
	case "ContentPaddingAddition":
		s.ContentPaddingAddition = AwgTimer(val)
	case "RekeyAfterTime":
		s.RekeyAfterTime = AwgTimer(val)
	case "RekeyTimeout":
		s.RekeyTimeout = AwgTimer(val)
	case "RejectAfterTime":
		s.RejectAfterTime = AwgTimer(val)
	case "KeepaliveTimeout":
		s.KeepaliveTimeout = AwgTimer(val)
	case "MaxHandshakeAttempts":
		s.MaxHandshakeAttempts = AwgTimer(val)
	case "RandomTrailers":
		s.RandomTrailers = parseConfBool(val)
	case "DisableCookies":
		s.DisableCookies = parseConfBool(val)
	}
}

func parseConfBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1", "on":
		return true
	default:
		return false
	}
}

func detectAwgVersion(s ServerConf) string {
	hRange := strings.Contains(s.H1, "-") || strings.Contains(s.H2, "-") ||
		strings.Contains(s.H3, "-") || strings.Contains(s.H4, "-")
	switch {
	case s.RandomTrailers || s.DisableCookies:
		return "3.1"
	case s.HeaderProtectionKey != "" || !s.ContentPaddingAddition.IsZero() || !s.RekeyAfterTime.IsZero() ||
		!s.RekeyTimeout.IsZero() || !s.RejectAfterTime.IsZero() || !s.KeepaliveTimeout.IsZero() ||
		!s.MaxHandshakeAttempts.IsZero():
		return "3"
	case s.S3 != 0 || s.S4 != 0 || s.I1 != "" || s.I2 != "" || s.I3 != "" || s.I4 != "" || s.I5 != "" || hRange:
		return "2"
	default:
		return "1.5"
	}
}

// ParseClientKeyFile extracts the interface private key (and optional DNS / I1–I5)
// from a client .conf so import can fill QR/export fields.
func ParseClientKeyFile(text string) ClientKeyFile {
	var k ClientKeyFile
	section := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok || section != "interface" {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "PrivateKey":
			k.PrivateKey = val
		case "DNS":
			if first, _, comma := strings.Cut(val, ","); comma {
				k.DNS = strings.TrimSpace(first)
			} else {
				k.DNS = val
			}
		case "I1":
			k.I1 = val
		case "I2":
			k.I2 = val
		case "I3":
			k.I3 = val
		case "I4":
			k.I4 = val
		case "I5":
			k.I5 = val
		}
	}
	return k
}

// PublicKeyOf returns the base64 public key for a private key, or "".
func PublicKeyOf(privateKey string) string {
	pub, err := wgutil.PublicKeyFromPrivate(privateKey)
	if err != nil {
		return ""
	}
	return pub
}

func sanitizeEmail(name, allowedIPs, publicKey string, used map[string]struct{}) string {
	base := slugEmail(name)
	if base == "" {
		base = "awg-" + firstHost(allowedIPs)
	}
	if base == "awg-" {
		if len(publicKey) >= 8 {
			base = "awg-" + publicKey[:8]
		} else {
			base = "awg-peer"
		}
	}
	base = strings.ToLower(base)
	email := base
	for i := 2; ; i++ {
		if _, ok := used[email]; !ok {
			used[email] = struct{}{}
			return email
		}
		email = base + "-" + strconv.Itoa(i)
	}
}

func slugEmail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-._")
}

func firstHost(allowedIPs string) string {
	first, _, _ := strings.Cut(allowedIPs, ",")
	first = strings.TrimSpace(first)
	host, _, _ := strings.Cut(first, "/")
	return strings.TrimSpace(host)
}

func parseUnixExpiry(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	if ts < 1_000_000_000_000 {
		return ts * 1000
	}
	return ts
}
