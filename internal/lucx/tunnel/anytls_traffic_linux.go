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

func dropLegacyAnytlsReturn() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "iptables-save").Output()
	if err != nil {
		return
	}
	for _, args := range legacyAnytlsReturnArgs(string(out)) {
		_ = exec.CommandContext(ctx, "iptables", args...).Run()
	}
	_ = exec.CommandContext(ctx, "iptables", "-F", "LUCX_ANYTLS_ACCT").Run()
	_ = exec.CommandContext(ctx, "iptables", "-X", "LUCX_ANYTLS_ACCT").Run()
}

func ensureAnytlsAcct(key string, port int) {
	if port <= 0 || key == "" {
		return
	}
	comment := anytlsAcctComment(key)
	p := strconv.Itoa(port)
	ensureIptablesAcct("INPUT", "--dport", p, comment)
	ensureIptablesAcct("OUTPUT", "--sport", p, comment)
}

func ensureIptablesAcct(chain, portFlag, port, comment string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	base := []string{chain, "-p", "tcp", portFlag, port, "-m", "comment", "--comment", comment}
	mark := append(append([]string{}, base...), "-j", "MARK", "--set-xmark", "0x0/0x0")
	if exec.CommandContext(ctx, "iptables", append([]string{"-C"}, mark...)...).Run() == nil {
		return
	}
	_ = exec.CommandContext(ctx, "iptables", append([]string{"-D"}, append(append([]string{}, base...), "-j", "RETURN")...)...).Run()
	_ = exec.CommandContext(ctx, "iptables", append([]string{"-D"}, append(append([]string{}, base...), "-j", "LUCX_ANYTLS_ACCT")...)...).Run()
	_ = exec.CommandContext(ctx, "iptables", append([]string{"-I"}, mark...)...).Run()
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
