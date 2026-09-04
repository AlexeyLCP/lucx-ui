// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

import (
	"context"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func tproxyMtproxyComment(id int) string {
	return "lucx-tproxy-mt-" + strconv.Itoa(id)
}

func EnsureMtproxyLocalOnly(id int) {
	comment := tproxyMtproxyComment(id)
	_ = clearMtproxyIptables(comment)
	for _, off := range []int{0, 1} {
		port := strconv.Itoa(tproxyLoopback(id, off))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = exec.CommandContext(ctx, "iptables", "-I", "INPUT", "!", "-i", "lo", "-p", "tcp", "--dport", port, "-m", "comment", "--comment", comment, "-j", "DROP").Run()
		cancel()
	}
}

func ClearMtproxyLocalOnly(id int) {
	_ = clearMtproxyIptables(tproxyMtproxyComment(id))
}

// EnsureMtproxyXrayRedirect hijacks MTProxy's DC TCP (uid mtproxy, not
// loopback) into a local dokodemo-door. Stock MTProxy has no SOCKS egress;
// this is the wrap. All tproxy inbounds share that uid — one redirect.
func EnsureMtproxyXrayRedirect(port int) {
	comment := tproxyXrayComment()
	_ = clearIptablesNatOutput(comment)
	if port <= 0 {
		return
	}
	u, err := user.Lookup(mtproxyEngineUser)
	if err != nil {
		logger.Warningf("tunnel: tproxy xray redirect skipped: no %s user", mtproxyEngineUser)
		return
	}
	if !mtproxyRedirectUIDOK(u.Uid) {
		logger.Warningf("tunnel: tproxy xray redirect skipped: %s uid=%s (root would hijack all TCP)", mtproxyEngineUser, u.Uid)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := mtproxyXrayRedirectArgs(u.Uid, port)
	if err := exec.CommandContext(ctx, "iptables", args...).Run(); err != nil {
		logger.Warningf("tunnel: tproxy xray redirect iptables: %v", err)
	}
}

func ClearMtproxyXrayRedirect() {
	_ = clearIptablesNatOutput(tproxyXrayComment())
}

func clearMtproxyIptables(comment string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "iptables-save").Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, comment) || !strings.HasPrefix(line, "-A INPUT") {
			continue
		}
		args := append([]string{"-D", "INPUT"}, strings.Fields(strings.TrimPrefix(line, "-A INPUT "))...)
		c2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		_ = exec.CommandContext(c2, "iptables", args...).Run()
		cancel2()
	}
	return nil
}

func clearIptablesNatOutput(comment string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "iptables-save", "-t", "nat").Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, comment) || !strings.HasPrefix(line, "-A OUTPUT") {
			continue
		}
		args := append([]string{"-t", "nat", "-D", "OUTPUT"}, strings.Fields(strings.TrimPrefix(line, "-A OUTPUT "))...)
		c2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		_ = exec.CommandContext(c2, "iptables", args...).Run()
		cancel2()
	}
	return nil
}
