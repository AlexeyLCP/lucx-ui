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

// GatewayUFWAllow: SSH, 80, 443, panel/sub, and inbounds that stay public.
func GatewayUFWAllow(webPort, subPort, sshPort int, rows []PreviewRow, selected map[int]bool, hidePanel bool) []string {
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
		if selected[r.InboundID] && r.Class != ClassSkip {
			continue
		}
		port := r.OldPort
		if r.NewPort > 0 {
			port = r.NewPort
		}
		if port <= 0 {
			continue
		}
		add(fmt.Sprintf("%d/tcp", port))
		add(fmt.Sprintf("%d/udp", port))
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
)

func UFWAvailable() bool {
	if !ufwOSLinux {
		return false
	}
	_, err := ufwLookPath("ufw")
	return err == nil
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

func ApplyUFW(allows []string) error {
	if !UFWAvailable() {
		return fmt.Errorf("ufw not installed")
	}
	for _, a := range allows {
		if err := ufwRun("allow", a); err != nil {
			return err
		}
	}
	if err := ufwRun("default", "deny", "incoming"); err != nil {
		return err
	}
	return ufwRun("--force", "enable")
}

// AllowUFW opens one public port for a naive inbound that stays off the mux.
func AllowUFW(port int) error {
	if port <= 0 || !UFWAvailable() {
		return nil
	}
	if err := ufwRun("allow", fmt.Sprintf("%d/tcp", port)); err != nil {
		return err
	}
	return ufwRun("allow", fmt.Sprintf("%d/udp", port))
}

func RevertUFW(wasActive bool) error {
	if !UFWAvailable() || wasActive {
		return nil
	}
	return ufwRun("disable")
}
