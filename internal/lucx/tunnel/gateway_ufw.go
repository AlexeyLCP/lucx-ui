// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
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
		if r.OldPort <= 0 {
			continue
		}
		add(fmt.Sprintf("%d/tcp", r.OldPort))
		add(fmt.Sprintf("%d/udp", r.OldPort))
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
		cmd := exec.Command("ufw", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
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
	cmd := exec.Command("ufw", "status")
	out, err := cmd.Output()
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

func RevertUFW(wasActive bool) error {
	if !UFWAvailable() || wasActive {
		return nil
	}
	return ufwRun("disable")
}
