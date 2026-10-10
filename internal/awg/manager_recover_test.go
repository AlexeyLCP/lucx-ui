// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recoveryFake fakes the kernel side of the manager for stale-interface tests:
// a fake netdir entry stands in for the /sys/class/net device, awgQuickFunc
// creates/removes it on up/down, deleteNetdev does the same harder.
type recoveryFake struct {
	netDir      string
	downErr     error
	startCalls  int
	deleteCalls int
}

func newRecoveryFake(t *testing.T) *recoveryFake {
	t.Helper()
	withTempConfigDir(t)
	f := &recoveryFake{netDir: t.TempDir()}
	origNet, origQuick, origDel := netClassBase, awgQuickFunc, deleteNetdev
	netClassBase = f.netDir
	awgQuickFunc = func(verb, confPath string) ([]byte, error) {
		name := strings.TrimSuffix(filepath.Base(confPath), ".conf")
		path := filepath.Join(f.netDir, name)
		switch verb {
		case "up":
			f.startCalls++
			if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
				return nil, err
			}
			return []byte("up " + name + "\n"), nil
		case "down":
			if f.downErr != nil {
				return []byte("down fail"), f.downErr
			}
			_ = os.Remove(path)
			return []byte("down " + name + "\n"), nil
		}
		return nil, nil
	}
	deleteNetdev = func(ifname string) error {
		f.deleteCalls++
		_ = os.Remove(filepath.Join(f.netDir, ifname))
		return nil
	}
	t.Cleanup(func() {
		netClassBase, awgQuickFunc, deleteNetdev = origNet, origQuick, origDel
	})
	return f
}

func staleTestInstance(id int) Instance {
	return Instance{
		Id:         id,
		Tag:        "awg-recover-test",
		Ifname:     ifnameFor(id),
		Port:       50000,
		PrivateKey: "6AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		Address:    "10.77.77.1/24",
		MTU:        1420,
	}
}

// TestEnsureLocked_RecoversStaleInterface: a live interface with no procs
// entry (kernel fallback tick swept the conf while its own down failed) used
// to fail every reconcile tick with "awg interface already up" until a panel
// restart. The manager must bring it down itself and start it, and the next
// tick must converge to the no-op fingerprint branch.
func TestEnsureLocked_RecoversStaleInterface(t *testing.T) {
	f := newRecoveryFake(t)
	if err := os.WriteFile(filepath.Join(f.netDir, "awg1"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := GetManager()
	inst := staleTestInstance(1)
	m.mu.Lock()
	err := m.ensureLocked(inst)
	m.mu.Unlock()
	if err != nil {
		t.Fatalf("ensure must recover a stale interface, got: %v", err)
	}
	if f.deleteCalls != 0 {
		t.Fatalf("graceful down must recover the iface, deleteNetdev calls=%d", f.deleteCalls)
	}
	if _, ok := m.procs[inst.Id]; !ok {
		t.Fatal("procs entry missing after recovery")
	}
	starts := f.startCalls
	m.mu.Lock()
	err = m.ensureLocked(inst)
	m.mu.Unlock()
	if err != nil {
		t.Fatalf("converged tick must be a no-op, got: %v", err)
	}
	if f.startCalls != starts {
		t.Fatalf("converged tick restarted the iface: starts %d -> %d", starts, f.startCalls)
	}
}

// TestEnsureLocked_RecoversWhenQuickDownFails: when awg-quick down fails
// (tools being swapped mid rebuild) the netdev stays behind; the recovery must
// fall back to the hard delete instead of failing forever.
func TestEnsureLocked_RecoversWhenQuickDownFails(t *testing.T) {
	f := newRecoveryFake(t)
	f.downErr = errors.New("test down failure")
	if err := os.WriteFile(filepath.Join(f.netDir, "awg2"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := GetManager()
	inst := staleTestInstance(2)
	m.mu.Lock()
	err := m.ensureLocked(inst)
	m.mu.Unlock()
	if err != nil {
		t.Fatalf("hard delete must recover after a failed quick down, got: %v", err)
	}
	if f.deleteCalls != 1 {
		t.Fatalf("expected exactly one hard delete, got %d", f.deleteCalls)
	}
	if _, ok := m.procs[inst.Id]; !ok {
		t.Fatal("procs entry missing after recovery")
	}
}
