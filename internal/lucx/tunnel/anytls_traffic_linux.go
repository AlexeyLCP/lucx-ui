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

const anytlsAcctChain = "LUCX_ANYTLS_ACCT"

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
}

func ensureAnytlsAcct(key string, port int) {
	if port <= 0 || key == "" {
		return
	}
	ensureAnytlsAcctChain()
	comment := anytlsAcctComment(key)
	p := strconv.Itoa(port)
	ensureIptablesAcct("INPUT", "--dport", p, comment)
	ensureIptablesAcct("OUTPUT", "--sport", p, comment)
}

func ensureAnytlsAcctChain() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if exec.CommandContext(ctx, "iptables", "-L", anytlsAcctChain, "-n").Run() == nil {
		return
	}
	_ = exec.CommandContext(ctx, "iptables", "-N", anytlsAcctChain).Run()
}

func ensureIptablesAcct(chain, portFlag, port, comment string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rule := func(action, target string) *exec.Cmd {
		return exec.CommandContext(ctx, "iptables", action, chain, "-p", "tcp", portFlag, port, "-m", "comment", "--comment", comment, "-j", target)
	}
	if rule("-C", anytlsAcctChain).Run() == nil {
		return
	}
	_ = rule("-D", "RETURN").Run()
	_ = rule("-I", anytlsAcctChain).Run()
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
