// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// bbrSysctlConfPath is shared with x-ui.sh's enable_bbr/disable_bbr (the
// legacy CLI menu) so whichever surface last touched BBR, the other
// recognizes the state and can cleanly restore it.
const bbrSysctlConfPath = "/etc/sysctl.d/99-bbr-x-ui.conf"

// BbrStatus reports whether TCP BBR congestion control (paired with the fq or
// cake queueing discipline) is active on the host. AWG/WireGuard itself rides
// on UDP and is unaffected by this, but every TCP flow the panel proxies —
// Xray's outbound connections, traffic relayed out of an AWG tunnel — benefits
// from BBR on higher-latency or lossy paths versus the kernel's CUBIC default.
type BbrStatus struct {
	Supported         bool   `json:"supported"`
	Enabled           bool   `json:"enabled"`
	CongestionControl string `json:"congestionControl"`
	Qdisc             string `json:"qdisc"`
	Windows           bool   `json:"windows"`
}

var fqOrCake = regexp.MustCompile(`^(fq|cake)$`)

func sysctlGet(name string) string {
	out, err := exec.CommandContext(context.Background(), "sysctl", "-n", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetBbrStatus reads the live sysctl values. Windows always reports
// Windows=true and everything else zero — there is no sysctl there.
func (s *ServerService) GetBbrStatus() BbrStatus {
	if runtime.GOOS == "windows" {
		return BbrStatus{Windows: true}
	}
	cc := sysctlGet("net.ipv4.tcp_congestion_control")
	qdisc := sysctlGet("net.core.default_qdisc")
	available := sysctlGet("net.ipv4.tcp_available_congestion_control")
	return BbrStatus{
		Supported:         strings.Contains(available, "bbr"),
		Enabled:           cc == "bbr" && fqOrCake.MatchString(qdisc),
		CongestionControl: cc,
		Qdisc:             qdisc,
	}
}

// EnableBbr writes /etc/sysctl.d/99-bbr-x-ui.conf (net.core.default_qdisc=fq,
// net.ipv4.tcp_congestion_control=bbr), comments out any conflicting lines in
// /etc/sysctl.conf so a reboot can't shadow the drop-in, and applies the new
// file live. Mirrors x-ui.sh's enable_bbr so the CLI menu and this endpoint
// agree on how BBR gets turned on and what "already enabled" means. Linux-only
// — BBR is a Linux TCP congestion-control module.
func (s *ServerService) EnableBbr() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("BBR tuning is only supported on Linux")
	}
	status := s.GetBbrStatus()
	if !status.Supported {
		return fmt.Errorf("kernel does not expose the bbr congestion control module (try: modprobe tcp_bbr)")
	}
	if status.Enabled {
		return nil
	}
	if _, err := os.Stat("/etc/sysctl.d"); err != nil {
		return fmt.Errorf("/etc/sysctl.d not found: %w", err)
	}
	// First line preserves the pre-BBR qdisc/congestion-control pair (commented
	// out) so DisableBbr can restore exactly what was active before, not just a
	// hardcoded guess.
	content := fmt.Sprintf("#%s:%s\nnet.core.default_qdisc = fq\nnet.ipv4.tcp_congestion_control = bbr\n",
		status.Qdisc, status.CongestionControl)
	if err := os.WriteFile(bbrSysctlConfPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", bbrSysctlConfPath, err)
	}
	commentSysctlConf("net.core.default_qdisc", "net.ipv4.tcp_congestion_control")
	// Apply only our drop-in file; `sysctl --system` would re-apply every
	// sysctl file on the host and surface unrelated errors from the distro's
	// own defaults (same reasoning as x-ui.sh's enable_bbr, see issue #5160).
	if out, err := exec.CommandContext(context.Background(), "sysctl", "-p", bbrSysctlConfPath).CombinedOutput(); err != nil {
		return fmt.Errorf("sysctl -p %s: %w: %s", bbrSysctlConfPath, err, strings.TrimSpace(string(out)))
	}
	if !s.GetBbrStatus().Enabled {
		return fmt.Errorf("sysctl applied but BBR did not activate — check kernel module support")
	}
	return nil
}

// DisableBbr restores whatever net.core.default_qdisc / tcp_congestion_control
// were active before EnableBbr ran (recovered from the first, commented line
// of 99-bbr-x-ui.conf), then removes the drop-in. When that file is absent
// (BBR was turned on some other way, or already off) it falls back to the
// kernel's historical CUBIC/pfifo_fast defaults, matching x-ui.sh's
// disable_bbr fallback. Linux-only.
func (s *ServerService) DisableBbr() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("BBR tuning is only supported on Linux")
	}
	qdisc, cc := "pfifo_fast", "cubic"
	hadDropIn := false
	if data, err := os.ReadFile(bbrSysctlConfPath); err == nil {
		hadDropIn = true
		first, _, _ := strings.Cut(string(data), "\n")
		prevQdisc, prevCC, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(first), "#"), ":")
		if ok && prevQdisc != "" {
			qdisc = prevQdisc
		}
		if ok && prevCC != "" {
			cc = prevCC
		}
	}
	if out, err := exec.CommandContext(context.Background(), "sysctl", "-w", "net.core.default_qdisc="+qdisc).CombinedOutput(); err != nil {
		logger.Warningf("bbr: restore net.core.default_qdisc=%s: %v: %s", qdisc, err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.CommandContext(context.Background(), "sysctl", "-w", "net.ipv4.tcp_congestion_control="+cc).CombinedOutput(); err != nil {
		logger.Warningf("bbr: restore net.ipv4.tcp_congestion_control=%s: %v: %s", cc, err, strings.TrimSpace(string(out)))
	}
	if hadDropIn {
		if err := os.Remove(bbrSysctlConfPath); err != nil {
			return fmt.Errorf("remove %s: %w", bbrSysctlConfPath, err)
		}
	}
	if s.GetBbrStatus().Enabled {
		return fmt.Errorf("sysctl applied but BBR is still active — check for another config re-enabling it")
	}
	return nil
}

// commentSysctlConf prefixes any live (uncommented) lines starting with the
// given keys in /etc/sysctl.conf with "# ", so a reboot doesn't reapply a
// stale qdisc/congestion-control setting that would shadow our drop-in file.
// Best-effort: a missing or unwritable /etc/sysctl.conf is not an error, it
// just means there is nothing to shadow our drop-in.
func commentSysctlConf(keys ...string) {
	const path = "/etc/sysctl.conf"
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		for _, k := range keys {
			if strings.HasPrefix(line, k) {
				lines[i] = "# " + line
				changed = true
			}
		}
	}
	if !changed {
		return
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		logger.Warning("bbr: comment out stale sysctl.conf lines:", err)
	}
}
