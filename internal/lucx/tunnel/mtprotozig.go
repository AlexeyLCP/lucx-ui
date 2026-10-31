// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"fmt"
	"strings"
)

func MtprotoZigKey(id int) string { return fmt.Sprintf("mtproto-zig-%d", id) }

// MtprotoZigConfig renders the standalone mtproto.zig WEB backend. Legacy
// tproxy settings remain untouched until the backend is explicitly selected.
func MtprotoZigConfigFromTproxy(id int, cfg TproxyConfig) (Instance, bool) {
	if cfg.Backend != "mtproto-zig" || !cfg.Enabled || cfg.Hostname == "" || cfg.Secret == "" {
		return Instance{Core: MtprotoZig, Key: MtprotoZigKey(id), Enabled: false}, true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[server]\nport = %d\nmax_connections = 512\n\n", cfg.Port)
	fmt.Fprintf(&b, "[censorship]\ntls_domain = %q\nmask = true\n\n", cfg.Hostname)
	fmt.Fprintf(&b, "[web]\nenabled = true\ndomain = %q\nbackend = \"127.0.0.1:%d\"\n", cfg.Hostname, cfg.Port)
	fmt.Fprintf(&b, "\n[access.users]\nuser = %q\n", cfg.Secret)
	return Instance{Core: MtprotoZig, Key: MtprotoZigKey(id), Enabled: true, ConfigText: b.String(), ProbePort: cfg.Port}, true
}
