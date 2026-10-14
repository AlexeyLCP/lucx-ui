// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// GatewayUFWAllow: SSH, 80, 443, panel/sub (unless hidden), and inbounds that
// stay public. moved marks inbounds whose port is withdrawn behind the mask —
// selected maskables, protocols hidden inside the site, and hidden naive.
// Those ports must never re-appear here: Apply re-adds everything in this
// list, so a leak here survives re-applying (lucx.297).
func GatewayUFWAllow(webPort, subPort, sshPort int, rows []PreviewRow, moved map[int]bool, hidePanel bool) []string {
	seen := map[string]bool{}
	var out []string
	add := func(spec string) {
		if spec == "" || seen[spec] {
			return
		}
		seen[spec] = true
		out = append(out, spec)
	}
	add("22/tcp")
	if sshPort > 0 && sshPort != 22 {
		add(fmt.Sprintf("%d/tcp", sshPort))
	}
	add("80/tcp")
	add("443/tcp")
	if !hidePanel {
		if webPort > 0 && webPort != 80 && webPort != 443 {
			add(fmt.Sprintf("%d/tcp", webPort))
		}
		if subPort > 0 && subPort != webPort && subPort != 80 && subPort != 443 {
			add(fmt.Sprintf("%d/tcp", subPort))
		}
	}
	for _, r := range rows {
		if moved[r.InboundID] {
			continue
		}
		port := r.OldPort
		if r.NewPort > 0 {
			port = r.NewPort
		}
		if port <= 0 || port == 22 || port == sshPort {
			continue
		}
		add(fmt.Sprintf("%d/tcp", port))
		add(fmt.Sprintf("%d/udp", port))
	}
	return out
}

// ufwSpecPort parses "22/tcp". SSH stays tcp-only; a delete of 22/udp can
// take the tcp half with it on some ufw versions.
func ufwSpecPort(spec string) (int, bool) {
	proto := strings.LastIndexByte(spec, '/')
	if proto <= 0 {
		return 0, false
	}
	n, err := strconv.Atoi(spec[:proto])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func ufwSSHSpec(spec string, sshPort int) bool {
	n, ok := ufwSpecPort(spec)
	return ok && (n == 22 || (sshPort > 0 && n == sshPort))
}

// GatewayUFWClose is the allow rules Apply must delete: the panel/sub ports
// when they are hidden, and the ports of every moved inbound. Everything the
// allow list keeps stays open.
func GatewayUFWClose(webPort, subPort, sshPort int, rows []PreviewRow, moved map[int]bool, hidePanel bool) []string {
	keep := map[string]bool{}
	for _, spec := range GatewayUFWAllow(webPort, subPort, sshPort, rows, moved, hidePanel) {
		keep[spec] = true
	}
	seen := map[string]bool{}
	var out []string
	add := func(spec string) {
		if spec == "" || seen[spec] || keep[spec] || ufwSSHSpec(spec, sshPort) {
			return
		}
		seen[spec] = true
		out = append(out, spec)
	}
	if hidePanel {
		if webPort > 0 {
			add(fmt.Sprintf("%d/tcp", webPort))
			add(fmt.Sprintf("%d/udp", webPort))
		}
		if subPort > 0 {
			add(fmt.Sprintf("%d/tcp", subPort))
			add(fmt.Sprintf("%d/udp", subPort))
		}
	}
	for _, r := range rows {
		if !moved[r.InboundID] {
			continue
		}
		for _, port := range []int{r.OldPort, r.NewPort} {
			if port <= 0 {
				continue
			}
			add(fmt.Sprintf("%d/tcp", port))
			add(fmt.Sprintf("%d/udp", port))
		}
	}
	return out
}

// GatewayUFWRestore is what Revert re-allows when UFW was already active:
// the panel/sub ports if they were hidden, and each moved inbound's old port.
func GatewayUFWRestore(webPort, subPort int, snap []GatewaySnapshotRow, hidePanel bool) []string {
	seen := map[string]bool{}
	var out []string
	add := func(spec string) {
		if spec == "" || seen[spec] {
			return
		}
		seen[spec] = true
		out = append(out, spec)
	}
	if hidePanel {
		if webPort > 0 {
			add(fmt.Sprintf("%d/tcp", webPort))
		}
		if subPort > 0 && subPort != webPort {
			add(fmt.Sprintf("%d/tcp", subPort))
		}
	}
	for _, sr := range snap {
		if sr.Port <= 0 || sr.Port == 22 {
			continue
		}
		add(fmt.Sprintf("%d/tcp", sr.Port))
		add(fmt.Sprintf("%d/udp", sr.Port))
	}
	return out
}

func parseSSHDPort(cfg string) int {
	port := 22
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 2 && strings.EqualFold(f[0], "Port") {
			if n, err := strconv.Atoi(f[1]); err == nil && n > 0 && n <= 65535 {
				port = n
			}
		}
	}
	return port
}

func SSHDPort() int {
	b, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return 22
	}
	return parseSSHDPort(string(b))
}

var (
	ufwOSLinux  = runtime.GOOS == "linux"
	ufwLookPath = exec.LookPath
	ufwRun      = func(args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "ufw", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				return err
			}
			return fmt.Errorf("%s: %w", msg, err)
		}
		return nil
	}
	ufwInstall = func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		apt := func(args ...string) error {
			cmd := exec.CommandContext(ctx, "apt-get", args...)
			cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
			out, err := cmd.CombinedOutput()
			if err != nil {
				msg := strings.TrimSpace(string(out))
				if msg == "" {
					return err
				}
				return fmt.Errorf("%s: %w", msg, err)
			}
			return nil
		}
		if err := apt("update"); err != nil {
			return err
		}
		return apt("install", "-y", "ufw")
	}
)

func UFWAvailable() bool {
	if !ufwOSLinux {
		return false
	}
	_, err := ufwLookPath("ufw")
	return err == nil
}

func ensureUFWInstalled() error {
	if _, err := ufwLookPath("apt-get"); err != nil {
		return fmt.Errorf("ufw not installed and apt-get not found: %w", err)
	}
	return ufwInstall()
}

func UFWActive() bool {
	if !UFWAvailable() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ufw", "status").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Status: active")
}

func ApplyUFW(allows, close []string) error {
	if !UFWAvailable() {
		if err := ensureUFWInstalled(); err != nil {
			return fmt.Errorf("ufw not installed, apt-get install failed: %w", err)
		}
	}
	for _, spec := range close {
		if ufwSSHSpec(spec, 22) {
			continue
		}
		// Missing rule is not a failure: the port was never allowed.
		_ = ufwRun("--force", "delete", "allow", spec)
	}
	for _, a := range allows {
		if err := ufwAllowSpec(a); err != nil {
			return err
		}
	}
	if err := ufwRun("default", "deny", "incoming"); err != nil {
		return err
	}
	return ufwRun("--force", "enable")
}

// ufwAllowSpec keeps SSH as one 22/tcp rule. 22/udp is dropped: on some ufw
// versions deleting that half takes the tcp rule with it.
func ufwAllowSpec(spec string) error {
	if spec != "22/tcp" && ufwSSHSpec(spec, 22) {
		return nil
	}
	return ufwRun("allow", spec)
}

// AllowUFW opens one public port for a naive inbound that stays off the mux.
func AllowUFW(port int) error {
	if port <= 0 || !UFWAvailable() {
		return nil
	}
	if err := ufwAllowSpec(fmt.Sprintf("%d/tcp", port)); err != nil {
		return err
	}
	return ufwAllowSpec(fmt.Sprintf("%d/udp", port))
}

func RevertUFW(wasActive bool, restore []string) error {
	if !UFWAvailable() {
		return nil
	}
	if !wasActive {
		return ufwRun("disable")
	}
	for _, spec := range restore {
		if err := ufwAllowSpec(spec); err != nil {
			return err
		}
	}
	return nil
}
