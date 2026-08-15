// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// HostStatus is the dashboard-level AWG snapshot (module + running ifaces).
type HostStatus struct {
	ModuleLoaded bool     `json:"moduleLoaded"`
	ModuleAwg3   bool     `json:"moduleAwg3"`
	Version      string   `json:"version"`
	Interfaces   int      `json:"interfaces"`
	Ifnames      []string `json:"ifnames,omitempty"`
}

var awgToolsVersionRe = regexp.MustCompile(`(?i)v?(\d+\.\d+(?:\.\d+)?(?:\.\d+)?)`)

// CollectHostStatus probes the kernel module, tools version, and how many
// managed AWG interfaces are currently up (inbound awgN + outbound awgo-N).
func CollectHostStatus() HostStatus {
	hs := HostStatus{}
	if runtime.GOOS != "linux" {
		return hs
	}
	if _, err := os.Stat("/sys/module/amneziawg"); err == nil {
		hs.ModuleLoaded = true
	}
	hs.ModuleAwg3 = ModuleSupportsAwg3()
	hs.Version = toolsVersion()
	mgr := GetManager()
	in := mgr.RunningIfnames()
	out := mgr.RunningClientIfnames()
	hs.Ifnames = make([]string, 0, len(in)+len(out))
	hs.Ifnames = append(hs.Ifnames, in...)
	hs.Ifnames = append(hs.Ifnames, out...)
	for i := 0; i < len(hs.Ifnames); i++ {
		for j := i + 1; j < len(hs.Ifnames); j++ {
			if hs.Ifnames[j] < hs.Ifnames[i] {
				hs.Ifnames[i], hs.Ifnames[j] = hs.Ifnames[j], hs.Ifnames[i]
			}
		}
	}
	hs.Interfaces = len(hs.Ifnames)
	return hs
}

// RunningInterfaceCount returns how many inbound awgN ifaces are up.
func (m *Manager) RunningInterfaceCount() int {
	return len(m.RunningIfnames())
}

// RunningIfnames returns sorted names of UP inbound awgN interfaces.
func (m *Manager) RunningIfnames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.procs))
	for _, cur := range m.procs {
		if cur != nil && cur.proc != nil && cur.proc.IsRunning() {
			if cur.ifname != "" {
				names = append(names, cur.ifname)
			} else {
				names = append(names, "?")
			}
		}
	}
	// simple insertion order is fine for small N; sort for stable tooltip
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}

func toolsVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "awg", "version").CombinedOutput()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if m := awgToolsVersionRe.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	// Fall back to first token after "v" if present.
	if i := strings.Index(strings.ToLower(s), "v"); i >= 0 && i+1 < len(s) {
		rest := s[i+1:]
		if j := strings.IndexAny(rest, " \t\n-"); j > 0 {
			return rest[:j]
		}
		return rest
	}
	return s
}
