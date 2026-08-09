// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package tunnel manages external tunnel-server sidecars that run next to the
// Xray core: protocols the panel serves through standalone upstream binaries
// rather than Xray itself. The first core is NaiveProxy — Caddy with the
// forward_proxy plugin from klzgrad/forwardproxy (naive branch, HTTP/2
// padding). Each core is one supervised process with a panel-rendered config
// file, an isolated data directory and a three-level health probe (process
// alive -> TCP listening -> TLS responding).
//
// The package is the sidecar mechanics only (process + config + probes);
// persistence and the HTTP API live in the web layer (service/tunnel.go,
// controller/tunnel.go), mirroring the mtproto split. Design reference for
// the Caddyfile pitfalls (admin off, bind normalization, per-instance data
// dirs): elector1337/3x-ui-naive.
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

// All returns the supported core names in display order.
func All() []Name {
	return []Name{Naive}
}

// Valid reports whether n is one of the supported core names.
func (n Name) Valid() bool {
	return n == Naive
}

// DisplayName returns the human-readable core name for UI and logs.
func (n Name) DisplayName() string {
	switch n {
	case Naive:
		return "NaiveProxy"
	default:
		return string(n)
	}
}

// BinaryName returns the core binary filename for the current platform,
// following the xray/mtg naming scheme (name-os-arch). Release pipelines
// place the binary under that name in the bin folder.
func (n Name) BinaryName() string {
	name := fmt.Sprintf("caddy-%s-%s-%s", string(n), runtime.GOOS, runtime.GOARCH)
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
	default:
		return filepath.Join(workDir(), string(n)+".conf")
	}
}

// dataDir returns the isolated state directory of one core. For NaiveProxy it
// is exported to the child as XDG_DATA_HOME so multiple panels/instances on
// one host never fight over the ACME certificate storage.
func dataDir(n Name) string {
	return filepath.Join(workDir(), string(n)+"-data")
}
