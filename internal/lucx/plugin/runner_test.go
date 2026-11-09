// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func forceRunner(t *testing.T, mode string) {
	old := selectRunner
	selectRunner = func(m Manifest) string { return mode }
	t.Cleanup(func() { selectRunner = old })
}

func fakeSystemd(t *testing.T, outputs map[string]string) {
	oldOut, oldRun := commandOutput, runCommand
	stderr := ""
	commandOutput = func(cmd *exec.Cmd) ([]byte, error) {
		if len(cmd.Args) < 2 {
			return nil, errors.New("unexpected command")
		}
		out, ok := outputs[cmd.Args[0]]
		if !ok {
			return nil, fmt.Errorf("unexpected binary %q", cmd.Args[0])
		}
		return []byte(out), nil
	}
	runCommand = func(cmd *exec.Cmd) error {
		if cmd.Path != "/usr/bin/false" {
			return nil
		}
		return fmt.Errorf("fake failure: %s", stderr)
	}
	t.Cleanup(func() { commandOutput, runCommand = oldOut, oldRun })
}

func writePluginDir(t *testing.T, id, manifest, entry string) {
	td := t.TempDir()
	old := binFolder
	binFolder = func() string { return td }
	t.Cleanup(func() { binFolder = old })
	plugDir := dir(id)
	if err := os.MkdirAll(plugDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath(id), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugDir, "run.sh"), []byte(entry), 0o700); err != nil {
		t.Fatal(err)
	}
}

func cpuManifest(id string, cpu int) Manifest {
	return Manifest{ID: id, Name: "Cpu", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "run.sh", Resources: &Resources{MaxCPUPercent: cpu}}
}

func TestUnitPropsMatchManifest(t *testing.T) {
	plugDir, err := filepath.Abs(dir("demo"))
	if err != nil {
		t.Fatal(err)
	}
	base := Manifest{ID: "demo", Name: "Demo", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "run.sh"}
	props := unitProps(base, plugDir, filepath.Join(plugDir, "data"), filepath.Join(plugDir, "plugin.log"))
	joined := strings.Join(props, " ")
	must := []string{
		"NoNewPrivileges=true", "ProtectSystem=strict", "PrivateTmp=true",
		"ReadWritePaths=" + filepath.Join(plugDir, "data"),
		"KillSignal=SIGINT", "TimeoutStopSec=3", "Restart=no",
		"WorkingDirectory=" + plugDir, "StandardOutput=append:", "Environment=LUCX_PLUGIN_ID=demo",
		"Environment=LUCX_PLUGIN_DATA=" + filepath.Join(plugDir, "data"),
	}
	for _, want := range must {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing property %q in %v", want, props)
		}
	}
	for _, guard := range []string{"PrivateNetwork=yes", "CPUQuota=", "MemoryMax="} {
		if strings.Contains(joined, guard) {
			t.Fatalf("unsolicited confinement property %q in %v", guard, props)
		}
	}
	iso := base
	iso.Permissions = map[string][]string{"network": {networkNone}, "panelApi": {}, "bot": {}}
	iso.Resources = &Resources{MaxMemoryMB: 64, MaxCPUPercent: 30}
	props = unitProps(iso, plugDir, filepath.Join(plugDir, "data"), filepath.Join(plugDir, "plugin.log"))
	joined = strings.Join(props, " ")
	for _, want := range []string{"PrivateNetwork=yes", "MemoryMax=64M", "LimitAS=64M", "CPUQuota=30%"} {
		if !strings.Contains(joined, want) {
			t.Fatal("enforcement property missing: " + want)
		}
	}
	flagPairs := 0
	for i := 0; i < len(props)-1; i++ {
		if props[i] == "-p" {
			flagPairs++
		}
	}
	if flagPairs != len(props)/2 {
		t.Fatalf("props are not -p value pairs: %v", props)
	}
}

func TestUnitRejectedValues(t *testing.T) {
	for _, bad := range []int{-5, 101, 1000} {
		if err := validateManifest(cpuManifest("demo", bad)); err == nil {
			t.Fatalf("accepted maxCPUPercent %d", bad)
		}
	}
	if err := validateManifest(cpuManifest("demo", 0)); err != nil {
		t.Fatalf("absent CPU limit rejected: %v", err)
	}
	if err := validateManifest(cpuManifest("demo", 100)); err != nil {
		t.Fatalf("full CPU cap rejected: %v", err)
	}
	c := cpuManifest("demo", 25)
	if !cpuRequiresUnit(c) || !needsEnforcement(c) {
		t.Fatal("CPU-only manifest does not require the unit runner")
	}
}

func TestUnitFailedStartSurfacesResult(t *testing.T) {
	fakeSystemd(t, map[string]string{
		systemdRunBin: "",
	})
	commandOutput = func(cmd *exec.Cmd) ([]byte, error) {
		if cmd.Args[0] == systemdRunBin {
			return []byte("Failed to start transient service"), nil
		}
		if cmd.Args[0] == systemctlBin {
			return []byte("ActiveState=failed\nMainPID=0\nResult=exit-code\nExecMainStatus=1\n"), nil
		}
		return nil, fmt.Errorf("unexpected binary %q", cmd.Args[0])
	}
	writePluginDir(t, "demo", "", "")
	m := cpuManifest("demo", 30)
	p := &managed{}
	if err := startUnit(p, m, filepath.Join(dir(m.ID), m.Entrypoint)); err == nil || !strings.Contains(err.Error(), "exit-code") {
		t.Fatalf("unit failure not surfaced: %v", err)
	}
	if p.mode != "" || p.unit != "" {
		t.Fatalf("a dead unit must not take over supervision: %v", p)
	}
}

func TestUnitStateParsesOutput(t *testing.T) {
	out := map[string]string{
		systemctlBin: "ActiveState=active\nMainPID=" + strconv.Itoa(os.Getpid()) + "\nResult=success\nExecMainStatus=0\n",
	}
	fakeSystemd(t, nil)
	commandOutput = func(cmd *exec.Cmd) ([]byte, error) { return []byte(out[systemctlBin]), nil }
	st := unitState(unitName("demo"))
	if !st.running || st.pid != os.Getpid() || st.detail != "" {
		t.Fatalf("active unit misparsed: %+v", st)
	}
	out[systemctlBin] = "ActiveState=failed\nMainPID=0\nResult=exit-code\nExecMainStatus=13\n"
	st = unitState(unitName("demo"))
	if st.running || st.pid != 0 || !strings.Contains(st.detail, "exit-code") {
		t.Fatalf("failed unit misparsed: %+v", st)
	}
}

func TestStartRefusesCPUWithoutUnitRunner(t *testing.T) {
	forceRunner(t, runnerProcess)
	writePluginDir(t, "cpu-demo", `{"id":"cpu-demo","name":"Cpu","version":"1.0.0","apiVersion":1,"type":"sidecar","entrypoint":"run.sh","resources":{"maxCPUPercent":30}}`, "#!/bin/sh\ntrue\n")
	m := cpuManifest("cpu-demo", 30)
	if err := validateManifest(m); err != nil {
		t.Fatalf("manifest rejected: %v", err)
	}
	if _, err := Load("cpu-demo"); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if err := Start("cpu-demo"); err == nil || !strings.Contains(err.Error(), "systemd unit runner") {
		t.Fatalf("CPU-only plugin must be refused by the process runner, got: %v", err)
	}
	if s, _ := Get("cpu-demo"); s.Running {
		t.Fatal("refused plugin reported running")
	}
}

func TestUnitStopWaitsForInactive(t *testing.T) {
	fakeSystemd(t, map[string]string{systemctlBin: "ActiveState=inactive\nMainPID=0\nResult=success\n"})
	p := &managed{mode: runnerUnit, unit: unitName("demo")}
	oldRun := runCommand
	stops := 0
	runCommand = func(cmd *exec.Cmd) error { stops++; return nil }
	t.Cleanup(func() { runCommand = oldRun })
	if err := stopUnit(p.unit); err != nil {
		t.Fatal(err)
	}
	if stops != 1 {
		t.Fatalf("stop did not invoke systemctl: %d", stops)
	}
	waitUnit(p.unit)
	if stops != 1 {
		t.Fatalf("inactive unit was killed again: %d", stops)
	}
}
