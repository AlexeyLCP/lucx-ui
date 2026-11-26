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
	"sync"
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

func EnsureMtproxyXraySocks(socksPort int) {
	redirectOwnerEnsure("tproxy", socksPort)
}

func ClearMtproxyXraySocks() {
	redirectOwnerClear("tproxy")
}

// redirectOwners guards the shared uid REDIRECT rule (lucx-mtproxy user,
// comment lucx-tproxy-xray) against cross-protocol flips: a routed AnyTLS and
// a routed tproxy both live on the same rule/listener, so one owner's
// reconcile must not clear what the other still needs (lucx.273).
var redirectOwners struct {
	mu  sync.Mutex
	set map[string]int
}

func redirectOwnerEnsure(owner string, socksPort int) {
	if socksPort <= 0 {
		return
	}
	redirectOwners.mu.Lock()
	if redirectOwners.set == nil {
		redirectOwners.set = map[string]int{}
	}
	redirectOwners.set[owner] = socksPort
	redirectOwners.mu.Unlock()
	// ponytail: one shared bridge target (last Ensure wins) — two routed
	// protocols with different socks ports flap the target; split per-owner
	// listeners if that combination shows up in the field.
	EnsureMtproxyXraySocksRule(socksPort)
}

func redirectOwnerClear(owner string) {
	redirectOwners.mu.Lock()
	empty := redirectOwners.set == nil
	if !empty {
		delete(redirectOwners.set, owner)
		empty = len(redirectOwners.set) == 0
	}
	redirectOwners.mu.Unlock()
	if empty {
		ClearMtproxyXraySocksRule()
	}
}

// EnsureMtproxyXraySocksRule is the physical half (listener + iptables rule).
func EnsureMtproxyXraySocksRule(socksPort int) {
	clearTproxyTunLeftovers()
	if socksPort <= 0 {
		return
	}
	u, err := user.Lookup(mtproxyEngineUser)
	if err != nil {
		logger.Warningf("tunnel: tproxy xray socks skipped: no %s user", mtproxyEngineUser)
		return
	}
	if !mtproxyRedirectUIDOK(u.Uid) {
		logger.Warningf("tunnel: tproxy xray socks skipped: %s uid=%s", mtproxyEngineUser, u.Uid)
		return
	}
	startTproxySocksBridge(tproxySocksRedirectPort, socksPort)
	_ = clearIptablesNatOutput(tproxyXrayComment())
	clearTproxyUdpRules()
	clearKillswitchRules()
	if err := runIptables("iptables", mtproxyXrayRedirectArgs(u.Uid, tproxySocksRedirectPort)); err != nil {
		logger.Warningf("tunnel: tproxy xray socks iptables: %v", err)
	}
	if err := runIptables("ip6tables", mtproxyXrayRedirectArgs6(u.Uid, tproxySocksRedirectPort)); err != nil {
		logger.Warningf("tunnel: tproxy xray socks ip6tables: %v", err)
	}
	ensureTproxyUdpRules(u.Uid, tproxySocksRedirectPort)
	v4Rules, v6Rules := mtproxyKillSwitchArgs(u.Uid)
	for _, args := range v4Rules {
		if err := runIptables("iptables", args); err != nil {
			logger.Warningf("tunnel: tproxy kill-switch iptables: %v", err)
		}
	}
	for _, args := range v6Rules {
		if err := runIptables("ip6tables", args); err != nil {
			logger.Warningf("tunnel: tproxy kill-switch ip6tables: %v", err)
		}
	}
}

// ensureTproxyUdpRules installs the UDP TPROXY half: mark the uid's
// non-loopback udp in mangle OUTPUT, keep the policy route alive
// (flushes lose it), and TPROXY the return path off lo into the hidden
// dokodeme. Runs on every reconcile (idempotent: comment-scoped clears +
// del-then-add rule, replace route).
func ensureTproxyUdpRules(uid string, onPort int) {
	ensureTproxyPolicyRoutes()
	if err := runIptables("iptables", mtproxyTproxyUdpMarkArgs(uid, "127.0.0.0/8")); err != nil {
		logger.Warningf("tunnel: tproxy udp mark iptables: %v", err)
	}
	if err := runIptables("ip6tables", mtproxyTproxyUdpMarkArgs(uid, "::1/128")); err != nil {
		logger.Warningf("tunnel: tproxy udp mark ip6tables: %v", err)
	}
	if err := runIptables("iptables", mtproxyTproxyPreroutingArgs("127.0.0.1", onPort)); err != nil {
		logger.Warningf("tunnel: tproxy udp tproxy iptables: %v", err)
	}
	if err := runIptables("ip6tables", mtproxyTproxyPreroutingArgs("::1", onPort)); err != nil {
		logger.Warningf("tunnel: tproxy udp tproxy ip6tables: %v", err)
	}
}

func runIp(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "ip", args...).Run()
}

func ensureTproxyPolicyRoutes() {
	for _, family := range [][]string{nil, {"-6"}} {
		del := append(append([]string{}, family...), "rule", "del", "fwmark", tproxyUdpMark, "lookup", tproxyUdpTable)
		add := append(append([]string{}, family...), "rule", "add", "fwmark", tproxyUdpMark, "lookup", tproxyUdpTable, "pref", "1802")
		route := append(append([]string{}, family...), "route", "replace", "local", "default", "dev", "lo", "table", tproxyUdpTable, "proto", "242")
		for dup := 0; dup < 3; dup++ {
			if err := runIp(del...); err != nil {
				break
			}
		}
		_ = runIp(add...)
		_ = runIp(route...)
	}
}

func runIptables(bin string, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, bin, args...).Run()
}

func clearKillswitchRules() {
	_ = clearIptablesTableChain("iptables", tproxyXrayComment(), "filter", "OUTPUT")
	_ = clearIptablesTableChain("ip6tables", tproxyXrayComment(), "filter", "OUTPUT")
}

func ClearMtproxyXraySocksRule() {
	_ = clearIptablesNatOutput(tproxyXrayComment())
	clearTproxyUdpRules()
	clearKillswitchRules()
	clearTproxyTunLeftovers()
}

// clearTproxyUdpRules removes the comment-scoped mangle rules (both families,
// OUTPUT and PREROUTING) and the policy rule/route for the UDP TPROXY.
// Route flushes lose the policy rule, so this also runs before every
// re-ensure.
func clearTproxyUdpRules() {
	for _, bin := range []string{"iptables", "ip6tables"} {
		_ = clearIptablesTableChain(bin, tproxyXrayComment(), "mangle", "OUTPUT")
		_ = clearIptablesTableChain(bin, tproxyXrayComment(), "mangle", "PREROUTING")
	}
	for _, family := range [][]string{nil, {"-6"}} {
		del := append(append([]string{}, family...), "rule", "del", "fwmark", tproxyUdpMark, "lookup", tproxyUdpTable)
		for dup := 0; dup < 3; dup++ {
			if err := runIp(del...); err != nil {
				break
			}
		}
		_ = runIp(append(append([]string{}, family...), "route", "del", "local", "default", "dev", "lo", "table", tproxyUdpTable)...)
	}
}

func clearTproxyTunLeftovers() {
	_ = clearIptablesMangleOutput(tproxyXrayComment())
	_ = clearIptablesTableChain("iptables", tproxyXrayComment(), "nat", "POSTROUTING")
	_ = exec.CommandContext(context.Background(), "ip", "rule", "del", "fwmark", "29120", "lookup", "1800").Run()
	_ = exec.CommandContext(context.Background(), "ip", "route", "flush", "table", "1800").Run()
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
	return clearIptablesTableChain("iptables", comment, "nat", "OUTPUT")
}

func clearIptablesMangleOutput(comment string) error {
	return clearIptablesTableChain("iptables", comment, "mangle", "OUTPUT")
}

func clearIptablesTableChain(bin, comment, table, chain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin+"-save", "-t", table).Output()
	if err != nil {
		return err
	}
	prefix := "-A " + chain + " "
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, comment) || !strings.HasPrefix(line, prefix) {
			continue
		}
		args := append([]string{"-t", table, "-D", chain}, strings.Fields(strings.TrimPrefix(line, prefix))...)
		c2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		_ = exec.CommandContext(c2, bin, args...).Run()
		cancel2()
	}
	return nil
}
