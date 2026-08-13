// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package tunnel manages external tunnel-server sidecars that run next to the
// Xray core: protocols the panel serves through standalone upstream binaries
// rather than Xray itself. Cores:
//   - NaiveProxy — Caddy + klzgrad/forwardproxy (HTTP/2 padding)
//   - olcRTC — openlibrecommunity/olcrtc (TCP-over-WebRTC via meet rooms)
//   - qWDTT — SpaceNeuroX wdtt-server (WireGuard over VK TURN)
//
// Each core is one supervised process with a panel-rendered config file (or
// CLI args), an isolated data directory and a health probe (process alive;
// TCP/TLS when the core listens). Design references: elector1337/3x-ui-naive,
// Bebrik2283555/Ex3-ui (olcRTC / qWDTT panel integration).
package tunnel

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// Name identifies a managed tunnel core.
type Name string

// Naive is the NaiveProxy core: Caddy + klzgrad/forwardproxy serving an
// HTTP/2 (and optionally HTTP/3) CONNECT forward proxy whose traffic is
// padded to resemble ordinary browser HTTPS.
const Naive Name = "naive"

// Olcrtc is the olcRTC core: TCP-over-WebRTC tunnel riding a legal video-
// call room (Jitsi / Yandex Telemost / WB Stream) as camouflage. Binary
// from openlibrecommunity/olcrtc (WTFPL).
const Olcrtc Name = "olcrtc"

// Qwdtt is the qWDTT core: WireGuard tunnelled through VK Calls TURN
// relays (SpaceNeuroX/proxy-turn-vk-android server.go, GPL-3.0). Needs
// root / CAP_NET_ADMIN (creates TUN + MASQUERADE). Binary ships as an
// external process — not linked into the panel.
const Qwdtt Name = "qwdtt"

// Mieru is the mieru core: mita server (enfein/mieru, GPL-3.0) — a
// censorship-resistant SOCKS/HTTP proxy over a custom TCP/UDP protocol
// (XChaCha20-Poly1305, no TLS). Multi-user; per-panel-client credentials.
const Mieru Name = "mieru"

// TrustTunnel is the TrustTunnel core: trusttunnel_endpoint
// (TrustTunnel/TrustTunnel, Apache-2.0) — VPN protocol by AdGuard VPN,
// traffic indistinguishable from HTTPS (HTTP/1.1 + HTTP/2 + QUIC). Requires
// a trusted TLS certificate (panel ACME certs are reused); multi-client.
const TrustTunnel Name = "trusttunnel"

// All returns the supported core names in display order.
func All() []Name {
	return []Name{Naive, Olcrtc, Qwdtt, Mieru, TrustTunnel}
}

// Valid reports whether n is one of the supported core names.
func (n Name) Valid() bool {
	switch n {
	case Naive, Olcrtc, Qwdtt, Mieru, TrustTunnel:
		return true
	}
	return false
}

// DisplayName returns the human-readable core name for UI and logs.
func (n Name) DisplayName() string {
	switch n {
	case Naive:
		return "NaiveProxy"
	case Olcrtc:
		return "olcRTC"
	case Qwdtt:
		return "qWDTT"
	case Mieru:
		return "mieru"
	case TrustTunnel:
		return "TrustTunnel"
	default:
		return string(n)
	}
}

// BinaryName returns the core binary filename for the current platform,
// following the xray/mtg naming scheme (name-os-arch). Release pipelines
// place the binary under that name in the bin folder.
func (n Name) BinaryName() string {
	var name string
	switch n {
	case Naive:
		// Historical name from lucx.91 — keep for installed hosts.
		name = fmt.Sprintf("caddy-naive-%s-%s", runtime.GOOS, runtime.GOARCH)
	default:
		name = fmt.Sprintf("%s-%s-%s", string(n), runtime.GOOS, runtime.GOARCH)
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// BinaryPath returns the expected location of the core binary, alongside the
// Xray binary.
func (n Name) BinaryPath() string {
	return filepath.Join(config.GetBinFolderPath(), n.BinaryName())
}

// tunnelDir is the per-package state directory (configs + data dirs). It is a
// variable so tests can redirect it into a temp dir.
var tunnelDir = func() string {
	return filepath.Join(config.GetBinFolderPath(), "tunnel")
}

// workDir returns the directory holding the rendered config files.
func workDir() string {
	return tunnelDir()
}

// configPath returns the rendered config file path of one core (legacy single-
// instance layout used by olcRTC / qWDTT and the pre-inbound global Naive).
func configPath(n Name) string {
	switch n {
	case Naive:
		return filepath.Join(workDir(), "naive.caddyfile")
	case Olcrtc:
		return filepath.Join(workDir(), "olcrtc.yaml")
	default:
		return filepath.Join(workDir(), string(n)+".conf")
	}
}

// trustTunnelHostsFileName is the companion hosts.toml filename of one
// TrustTunnel instance (ExtraFiles key; the endpoint takes it as argv[2]).
func trustTunnelHostsFileName(key string) string {
	return key + "-hosts.toml"
}

// configPathFor returns the config path for a managed instance. Multi-inbound
// Naive keys (naive-12) get their own Caddyfile so N inbounds never share one
// process config; legacy single-core keys keep the historical filename.
func configPathFor(key string, n Name) string {
	if key != "" && key != string(n) {
		switch n {
		case Naive:
			return filepath.Join(workDir(), key+".caddyfile")
		case Olcrtc:
			return filepath.Join(workDir(), key+".yaml")
		case Mieru:
			return filepath.Join(workDir(), key+".json")
		case TrustTunnel:
			return filepath.Join(workDir(), key+".toml")
		default:
			return filepath.Join(workDir(), key+".conf")
		}
	}
	return configPath(n)
}

// dataDir returns the isolated state directory of one core. For NaiveProxy it
// is exported to the child as XDG_DATA_HOME so multiple panels/instances on
// one host never fight over the ACME certificate storage. For olcRTC it is
// written into the server YAML `data:` field.
func dataDir(n Name) string {
	return filepath.Join(workDir(), string(n)+"-data")
}

// dataDirFor returns the ACME/state dir for a managed instance. Multi-key
// Naive inbounds each get their own XDG_DATA_HOME so certificates never clash.
func dataDirFor(key string, n Name) string {
	if key != "" && key != string(n) {
		return filepath.Join(workDir(), key+"-data")
	}
	return dataDir(n)
}

// absPath resolves p to an absolute path. mita's gRPC client treats a
// relative MITA_UDS_PATH like "bin/tunnel/…/mita.sock" as host "bin"
// ("invalid (non-empty) authority: bin") and every `get users` scrape fails
// silently — per-client traffic stays at 0 forever (E2E lucx.117).
func absPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// DataDir is the exported form of dataDir for the web layer (YAML render).
func DataDir(n Name) string {
	return dataDir(n)
}

// AccessLogPath is the JSON access_log file for a Naive instance key
// (legacy "naive" or "naive-{id}"). RenderCaddyfile writes here; CollectNaiveTraffic
// tails it for online/traffic.
func AccessLogPath(key string) string {
	return filepath.Join(dataDirFor(key, Naive), "access.json")
}
