// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestKillStrayTunnelProcessesIgnoresEmptyInput(t *testing.T) {
	for _, paths := range [][]string{nil, {}, {""}, {"."}, {"/"}} {
		if n := killStrayTunnelProcesses(paths); n != 0 {
			t.Fatalf("killStrayTunnelProcesses(%v) = %d, want 0", paths, n)
		}
	}
}

// The sweep must not touch a process that merely lives next to ours or has a
// similar name — only an exact base-name match is one of our cores.
func TestKillStrayTunnelProcessesSparesUnrelatedNames(t *testing.T) {
	proc := startNamedHelper(t, "not-a-tunnel-core")

	if n := killStrayTunnelProcesses([]string{"/opt/x-ui/bin/caddy-naive-linux-amd64"}); n != 0 {
		t.Fatalf("killStrayTunnelProcesses reported %d kill(s), want 0", n)
	}
	if !processAlive(proc.Process.Pid) {
		t.Fatal("the unrelated helper was killed, want it left alone")
	}
}

// A process whose executable base name matches a core binary is an orphan of a
// previous run and must be killed, whatever directory it was started from —
// the panel is the only thing that ever runs these binaries.
func TestKillStrayTunnelProcessesKillsMatchingName(t *testing.T) {
	proc := startNamedHelper(t, "qwdtt-linux-amd64")
	done := make(chan struct{})
	go func() {
		_ = proc.Wait()
		close(done)
	}()

	if n := killStrayTunnelProcesses([]string{proc.Path}); n != 1 {
		t.Fatalf("killStrayTunnelProcesses = %d, want 1", n)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the matching helper is still running 10s after the sweep")
	}
}

// The bin folder holds several cores at once, so the sweep takes a list and
// must match against every entry in it.
func TestKillStrayTunnelProcessesMatchesAnyCore(t *testing.T) {
	proc := startNamedHelper(t, "mieru-linux-amd64")
	done := make(chan struct{})
	go func() {
		_ = proc.Wait()
		close(done)
	}()

	paths := []string{
		"/opt/x-ui/bin/caddy-naive-linux-amd64",
		"/opt/x-ui/bin/olcrtc-linux-amd64",
		proc.Path,
		"/opt/x-ui/bin/trusttunnel-linux-amd64",
	}
	if n := killStrayTunnelProcesses(paths); n != 1 {
		t.Fatalf("killStrayTunnelProcesses = %d, want 1", n)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the matching helper is still running 10s after the sweep")
	}
}

func TestProcExeBaseReadsOwnBinary(t *testing.T) {
	if got := procExeBase(os.Getpid()); got == "" {
		t.Fatal(`procExeBase for the test process = "", want the test binary name`)
	}
}

func TestCmdlineArgv0BaseReadsOwnBinary(t *testing.T) {
	if got := cmdlineArgv0Base(os.Getpid()); got == "" {
		t.Fatal(`cmdlineArgv0Base for the test process = "", want the test binary name`)
	}
}

func TestProcExeBaseOnDeadPid(t *testing.T) {
	if got := procExeBase(-1); got != "" {
		t.Fatalf("procExeBase(-1) = %q, want an empty string", got)
	}
	if got := cmdlineArgv0Base(-1); got != "" {
		t.Fatalf("cmdlineArgv0Base(-1) = %q, want an empty string", got)
	}
}

// startNamedHelper runs a long-lived process whose executable is literally
// named name, so /proc/<pid>/exe and argv[0] both carry it. A shell script
// would not do: the kernel runs the interpreter, so /proc/<pid>/exe reads as
// /bin/sh and the sweep would never see the name under test.
func startNamedHelper(t *testing.T, name string) *exec.Cmd {
	t.Helper()
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skipf("sleep binary not available: %v", err)
	}
	payload, err := os.ReadFile(sleep)
	if err != nil {
		t.Skipf("cannot read %s: %v", sleep, err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, payload, 0o755); err != nil {
		t.Fatalf("write helper binary: %v", err)
	}

	cmd := exec.Command(path, "600")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	waitForProcExe(t, cmd.Process.Pid, name)
	return cmd
}

// waitForProcExe blocks until /proc/<pid>/exe reports name, so the sweep is
// never run against a process the kernel has not finished exec'ing.
func waitForProcExe(t *testing.T, pid int, name string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if procExeBase(pid) == name {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("/proc/%d/exe never reported %q", pid, name)
}

// processAlive reports whether pid still exists, using the null signal.
func processAlive(pid int) bool {
	return syscall.Kill(pid, syscall.Signal(0)) == nil
}
