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

// All returns the supported core names in display order.
func All() []Name {
	return []Name{Naive, Olcrtc, Qwdtt}
}

// Valid reports whether n is one of the supported core names.
func (n Name) Valid() bool {
	return n == Naive || n == Olcrtc || n == Qwdtt
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

// configPath returns the rendered config file path of one core.
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

// dataDir returns the isolated state directory of one core. For NaiveProxy it
// is exported to the child as XDG_DATA_HOME so multiple panels/instances on
// one host never fight over the ACME certificate storage. For olcRTC it is
// written into the server YAML `data:` field.
func dataDir(n Name) string {
	return filepath.Join(workDir(), string(n)+"-data")
}

// DataDir is the exported form of dataDir for the web layer (YAML render).
func DataDir(n Name) string {
	return dataDir(n)
}
