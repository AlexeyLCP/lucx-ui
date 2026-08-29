//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func anytlsSessionCount(port int) int {
	n := 0
	if b, err := os.ReadFile("/proc/net/tcp"); err == nil {
		n += parseTCPEstablished(string(b), port, true)
	}
	if b, err := os.ReadFile("/proc/net/tcp6"); err == nil {
		n += parseTCPEstablished(string(b), port, true)
	}
	return n
}

func ensureAnytlsAcct(key string, port int) {
	if port <= 0 || key == "" {
		return
	}
	comment := anytlsAcctComment(key)
	p := strconv.Itoa(port)
	ensureIptablesReturn("INPUT", "--dport", p, comment)
	ensureIptablesReturn("OUTPUT", "--sport", p, comment)
}

func ensureIptablesReturn(chain, portFlag, port, comment string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, "iptables", "-C", chain, "-p", "tcp", portFlag, port, "-m", "comment", "--comment", comment, "-j", "RETURN")
	if check.Run() == nil {
		return
	}
	ins := exec.CommandContext(ctx, "iptables", "-I", chain, "-p", "tcp", portFlag, port, "-m", "comment", "--comment", comment, "-j", "RETURN")
	_ = ins.Run()
}

func anytlsByteCounters(key string) (up, down int64, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "iptables-save", "-c").Output()
	if err != nil {
		return 0, 0, false
	}
	return parseIptablesSave(string(out), anytlsAcctComment(key))
}
