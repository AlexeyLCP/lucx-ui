// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

/* Runner modes. "unit" = transient systemd unit started by systemd-run
(kernel-enforced namespaces, mounts and limits regardless of uid), "process"
= the in-process child runner of stages 1-2. A plugin that asks for CPU
capping requires the unit runner; a plugin that asks for netns or a memory
cap may fall back to the process runner, which is already fail-closed on
hosts where its enforcement is impossible (verified live 2026-11-01). */

const (
	runnerUnit    = "unit"
	runnerProcess = "process"
)

var (
	systemdRunBin   = "systemd-run"
	systemctlBin    = "systemctl"
	runCommand      = func(cmd *exec.Cmd) error { return cmd.Run() }
	commandOutput   = func(cmd *exec.Cmd) ([]byte, error) { cmd.Stderr = nil; return cmd.Output() }
	unitRunningFile = func() bool { _, e := os.Stat("/run/systemd/system"); return e == nil }
	selectRunner    = func(m Manifest) string {
		if runtime.GOOS == "linux" && lookPathUnit() {
			return runnerUnit
		}
		return runnerProcess
	}
)

func unitName(id string) string { return "lucx-plugin-" + id }

func lookPathUnit() bool {
	if _, err := exec.LookPath(systemdRunBin); err != nil {
		return false
	}
	if _, err := exec.LookPath(systemctlBin); err != nil {
		return false
	}
	return unitRunningFile()
}

/*
	unitProps builds the systemd-run property list for one manifest. The unit

runs as the panel's user (no DynamicUser yet: plugin code and data keep their
panel-side ownership); confinement is namespace/mount/limit based. Private
network isolates from the host; the plugin's own directory stays writable
through ReadWritePaths under ProtectSystem=strict; logs land in the same
plugin.log as the child runner.
*/
func unitProps(m Manifest, plugDir, dataDir, log string) []string {
	props := []string{
		"NoNewPrivileges=true",
		"ProtectSystem=strict",
		"ReadWritePaths=" + dataDir,
		"ProtectHome=true",
		"PrivateTmp=true",
		"PrivateDevices=true",
		"ProtectKernelTunables=true",
		"ProtectKernelModules=true",
		"ProtectKernelLogs=true",
		"ProtectControlGroups=true",
		"ProtectClock=true",
		"ProtectHostname=true",
		"RestrictNamespaces=yes",
		"RestrictRealtime=true",
		"LockPersonality=true",
		"CapabilityBoundingSet=",
		"AmbientCapabilities=",
		"KillSignal=SIGINT",
		"FinalKillSignal=SIGKILL",
		"TimeoutStopSec=3",
		"Restart=no",
		"WorkingDirectory=" + plugDir,
		"StandardOutput=append:" + log,
		"StandardError=append:" + log,
	}
	if denyNetwork(m) {
		props = append(props, "PrivateNetwork=yes")
	}
	if mb := memLimit(m); mb > 0 {
		props = append(props, "MemoryMax="+strconv.Itoa(mb)+"M", "LimitAS="+strconv.Itoa(mb)+"M")
	}
	if m.Resources != nil && m.Resources.MaxCPUPercent > 0 {
		props = append(props, "CPUQuota="+strconv.Itoa(m.Resources.MaxCPUPercent)+"%")
	}
	if needsSocket(m) {
		/* The events/sub-after socket lives in a dedicated runtime directory: systemd
		creates and owns it for the unit, removes it on stop, and its socket
		is unreachable for other users. */
		props = append(props,
			"RuntimeDirectory=x-ui-plug-"+m.ID,
			"RuntimeDirectoryMode=0700",
			"Environment=LUCX_PLUGIN_SOCKET="+socketPath(m, runnerUnit))
	}
	for _, kv := range pluginEnv(m) {
		props = append(props, "Environment="+kv)
	}
	out := make([]string, 0, len(props)*2)
	for _, p := range props {
		out = append(out, "-p", p)
	}
	return append(out,
		"-p", "Environment=LUCX_PLUGIN_ID="+m.ID,
		"-p", "Environment=LUCX_PLUGIN_VERSION="+m.Version,
		"-p", "Environment=LUCX_PLUGIN_DATA="+dataDir)
}

/*
	startUnit launches the plugin inside a transient unit. systemd-run returns

after the unit is queued, so a follow-up state probe decides whether the
start really happened; a failed unit surfaces its Result as the start
error.
*/
func startUnit(p *managed, m Manifest, bin string) error {
	/* systemd 252 rejects relative -p paths silently (exit 1), so the unit
	runner must hand PID1 absolute, cwd-resolved paths — the child runner
	resolves the same layout against the panel cwd. */
	plugDir, err := filepath.Abs(dir(m.ID))
	if err != nil {
		return err
	}
	props := unitProps(m, plugDir, filepath.Join(plugDir, "data"), filepath.Join(plugDir, "plugin.log"))
	_ = runCommand(exec.CommandContext(context.Background(), systemctlBin,
		"reset-failed", unitName(m.ID)))
	args := []string{
		"--unit=" + unitName(m.ID), "--collect", "--no-ask-password",
		"--quiet",
	}
	args = append(args, props...)
	cmd := exec.CommandContext(context.Background(), systemdRunBin, append(args, bin)...)
	out, err := commandOutput(cmd)
	if err != nil {
		detail := strings.TrimSpace(string(out))
		msg := fmt.Errorf("systemd unit runner: %w", err)
		if detail != "" {
			msg = fmt.Errorf("systemd unit runner: %w: %s", err, detail)
		}
		_, _ = appendPluginLog(m.ID, msg.Error())
		return msg
	}
	if state := unitState(unitName(m.ID)); !state.running && state.detail != "" {
		_, _ = appendPluginLog(m.ID, state.detail)
		return errors.New(state.detail)
	}
	p.mode = runnerUnit
	p.unit = unitName(m.ID)
	if p.done == nil {
		p.done = make(chan struct{})
	}
	return nil
}

func appendPluginLog(id, text string) (int, error) {
	f, err := os.OpenFile(logPath(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.WriteString(text + "\n")
}

type unitStatus struct {
	running bool
	pid     int
	detail  string
}

/*
	unitState reads the transient unit live. It doubles as the supervision

probe after a panel restart: the unit may outlive the panel process.
*/
func unitState(name string) unitStatus {
	out, err := commandOutput(exec.CommandContext(context.Background(), systemctlBin,
		"show", name, "-p", "ActiveState", "-p", "MainPID", "-p", "Result", "-p", "ExecMainStatus"))
	if err != nil {
		return unitStatus{running: false, detail: "systemd unit probe failed"}
	}
	st := unitStatus{}
	for _, line := range strings.Split(string(bytes.TrimSpace(out)), "\n") {
		k, v, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch k {
		case "ActiveState":
			st.running = v == "active" || v == "activating"
		case "MainPID":
			st.pid, _ = strconv.Atoi(v)
		case "Result":
			if v != "success" {
				st.detail = "unit result: " + v
			}
		case "ExecMainStatus":
			if st.detail != "" {
				st.detail += fmt.Sprintf(" (exit status %s)", v)
			}
		}
	}
	if st.pid == 0 || st.pid < 0 {
		st.pid = 0
	}
	return st
}

func stopUnit(name string) error {
	if err := runCommand(exec.CommandContext(context.Background(), systemctlBin, "stop", name)); err != nil {
		return err
	}
	return nil
}

/*
	waitUnit blocks until the unit leaves active state or the grace period

ends, mirroring the child runner's SIGINT then SIGKILL flow.
*/
func waitUnit(name string) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !unitState(name).running {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = runCommand(exec.CommandContext(context.Background(), systemctlBin, "kill", "--signal=SIGKILL", name))
}

/* selectRunner picks the start mode via the package var above (seam for
tests). The CPU cap exists only in the unit runner, so a plugin demanding it
pins the mode; on hosts without a system systemd manager everything falls
back to the process runner. */

/*
	cpuRequiresUnit reports whether the manifest can only be honored by the

unit runner.
*/
func cpuRequiresUnit(m Manifest) bool {
	return m.Resources != nil && m.Resources.MaxCPUPercent > 0
}
