// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

import (
	"os/exec"
	"strconv"
	"strings"
)

func tproxyMtproxyComment(id int) string {
	return "lucx-tproxy-mt-" + strconv.Itoa(id)
}

func EnsureMtproxyLocalOnly(id int) {
	comment := tproxyMtproxyComment(id)
	_ = clearMtproxyIptables(comment)
	for _, off := range []int{0, 1} {
		port := strconv.Itoa(tproxyLoopback(id, off))
		_ = exec.Command("iptables", "-I", "INPUT", "!", "-i", "lo", "-p", "tcp", "--dport", port, "-m", "comment", "--comment", comment, "-j", "DROP").Run()
	}
}

func ClearMtproxyLocalOnly(id int) {
	_ = clearMtproxyIptables(tproxyMtproxyComment(id))
}

func clearMtproxyIptables(comment string) error {
	out, err := exec.Command("iptables-save").Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, comment) || !strings.HasPrefix(line, "-A INPUT") {
			continue
		}
		args := append([]string{"-D", "INPUT"}, strings.Fields(strings.TrimPrefix(line, "-A INPUT "))...)
		_ = exec.Command("iptables", args...).Run()
	}
	return nil
}
