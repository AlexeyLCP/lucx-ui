// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// killStrayTunnelProcesses terminates tunnel-core sidecars left running by a
// previous x-ui run and returns how many were killed.
//
// The panel starts one process per tunnel inbound outside its own lifecycle.
// A graceful shutdown runs Manager.StopAll (web.go), but an ungraceful one —
// OOM killer, `kill -9`, a panic, `systemctl kill` — does not, and on Linux a
// child is not tied to the parent (unlike Windows, where attachChildLifetime
// puts every child in a kill-on-exit job object). The survivor keeps holding
// the inbound's listen port, so the next panel start cannot bind and the
// inbound comes up dead with a bind error nobody asked for.
//
// x-ui is the sole owner of these binaries, so anything alive at startup whose
// executable matches one of our core binary names is an orphan and is safe to
// kill before we start our own. This mirrors killStrayMtgProcesses in
// internal/mtproto, which solved the same problem for the mtg sidecar.
//
// Deliberately not SysProcAttr.Pdeathsig: the parent-death signal is delivered
// when the *OS thread* that forked the child exits, and the Go runtime moves
// goroutines between threads freely, so a child can be SIGKILLed at random
// while the panel is perfectly healthy. Sweeping at startup has no such
// failure mode.
//
// Matching is on the executable's base name so it is independent of the bin
// folder, and falls back to argv[0] because an update that replaced the binary
// makes /proc/<pid>/exe read as "<path> (deleted)".
func killStrayTunnelProcesses(binaryPaths []string) int {
	wanted := make(map[string]struct{}, len(binaryPaths))
	for _, path := range binaryPaths {
		path = filepath.Clean(path)
		if path == "" || path == "." || path == string(filepath.Separator) {
			continue
		}
		wanted[path] = struct{}{}
	}
	if len(wanted) == 0 {
		return 0
	}
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	killed := 0
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		if !matchesWantedBinary(pid, wanted) {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil {
			killed++
		}
	}
	return killed
}

func matchesWantedBinary(pid int, wanted map[string]struct{}) bool {
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return false
	}
	path := filepath.Clean(strings.TrimSuffix(exe, " (deleted)"))
	_, ok := wanted[path]
	return ok
}

// procExeBase returns the base name of /proc/<pid>/exe, or "" if unreadable.
func procExeBase(pid int) string {
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return ""
	}
	return filepath.Base(strings.TrimSuffix(exe, " (deleted)"))
}

// cmdlineArgv0Base returns the base name of argv[0] from /proc/<pid>/cmdline,
// the reliable fallback when the binary has been replaced or exe is unreadable.
func cmdlineArgv0Base(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return ""
	}
	argv0 := data
	if i := strings.IndexByte(string(data), 0); i >= 0 {
		argv0 = data[:i]
	}
	if len(argv0) == 0 {
		return ""
	}
	return filepath.Base(string(argv0))
}
