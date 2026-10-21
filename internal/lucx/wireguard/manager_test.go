// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"fmt"
	"strings"
	"testing"
)

func TestManagerPeerEditSyncsWithoutRecreatingDevice(t *testing.T) {
	var calls []string
	exists := false
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		calls = append(calls, cmd)
		switch {
		case strings.HasPrefix(cmd, "ip -j -d link show"):
			if exists {
				return []byte(`[{"ifalias":"lucx-kernel-wireguard","linkinfo":{"info_kind":"wireguard"}}]`), nil
			}
			return nil, fmt.Errorf("absent")
		case strings.HasPrefix(cmd, "ip link add"):
			exists = true
		case strings.HasPrefix(cmd, "ip -j rule"):
			return []byte(`[]`), nil
		case strings.HasPrefix(cmd, "ip -j route"):
			return []byte(`[]`), nil
		case strings.HasPrefix(cmd, "nft -j list table "):
			return nil, fmt.Errorf("absent")
		case strings.HasPrefix(cmd, "wg show"):
			return []byte("private public 51820 off\n"), nil
		}
		return nil, nil
	}
	m := NewManager(t.TempDir(), command)
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	calls = nil
	inst.Peers = nil
	if err := m.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(calls, "\n")
	if !strings.Contains(text, "wg syncconf lxwg7") || strings.Contains(text, "ip link add") || strings.Contains(text, "ip link delete") {
		t.Fatalf("peer update recreated interface: %s", text)
	}
}

func TestManagerRefusesForeignInterface(t *testing.T) {
	var mutated bool
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		if strings.HasPrefix(cmd, "ip -j -d link show") {
			return []byte(`[{"ifalias":"foreign","linkinfo":{"info_kind":"wireguard"}}]`), nil
		}
		mutated = true
		return nil, nil
	}
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := NewManager(t.TempDir(), command).Ensure(inst); err == nil {
		t.Fatal("foreign interface accepted")
	}
	if mutated {
		t.Fatal("foreign interface was modified")
	}
}

func TestManagerInstallsGuardBeforeInterfaceActivation(t *testing.T) {
	var calls []string
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		calls = append(calls, cmd)
		if strings.HasPrefix(cmd, "ip -j -d link show") || strings.HasPrefix(cmd, "nft -j list table ") {
			return nil, fmt.Errorf("absent")
		}
		if strings.HasPrefix(cmd, "ip -j") {
			return []byte(`[]`), nil
		}
		if strings.HasPrefix(cmd, "wg show") {
			return []byte("private public 51820 off\n"), nil
		}
		return nil, nil
	}
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := NewManager(t.TempDir(), command).Ensure(inst); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(calls, "\n")
	guard := strings.Index(text, "nft -f -")
	activation := strings.Index(text, "ip link set dev lxwg7 mtu 1280 up")
	if guard < 0 || activation < 0 || guard >= activation {
		t.Fatalf("device activated before guard: %s", text)
	}
}

func TestManagerAccountsNewDeviceFromZero(t *testing.T) {
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		if strings.HasPrefix(cmd, "ip -j -d link show") || strings.HasPrefix(cmd, "nft -j list table ") {
			return nil, fmt.Errorf("absent")
		}
		if strings.HasPrefix(cmd, "ip -j") {
			return []byte(`[]`), nil
		}
		if strings.HasPrefix(cmd, "wg show") {
			return []byte("private public 51820 off\n" + testKey(2) + " PSK endpoint 10.90.0.2/32 990 123 456 25\n"), nil
		}
		return nil, nil
	}
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(t.TempDir(), command)
	if err := m.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	got := m.CollectTraffic()
	if len(got) != 1 || got[0].Email != "alice" || got[0].Up != 123 || got[0].Down != 456 {
		t.Fatalf("new device usage was lost: %+v", got)
	}
}

func TestReconcileRemovesOnlyOwnedOrphanInterfaces(t *testing.T) {
	var calls []string
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		if strings.HasPrefix(cmd, "nft -j list table ") {
			return nil, fmt.Errorf("absent")
		}
		calls = append(calls, cmd)
		if cmd == "ip -j -d link show type wireguard" {
			return []byte(`[{"ifname":"lxwg7","ifalias":"lucx-kernel-wireguard"},{"ifname":"wg0","ifalias":"foreign"}]`), nil
		}
		if strings.HasPrefix(cmd, "ip -j -d link show dev lxwg7") {
			return []byte(`[{"ifalias":"lucx-kernel-wireguard","linkinfo":{"info_kind":"wireguard"}}]`), nil
		}
		if strings.HasPrefix(cmd, "wg show") {
			return []byte("private public 51820 off\n"), nil
		}
		return nil, nil
	}
	if errors := NewManager(t.TempDir(), command).Reconcile(nil); len(errors) != 0 {
		t.Fatalf("cleanup failed: %v", errors)
	}
	text := strings.Join(calls, "\n")
	if !strings.Contains(text, "ip link delete dev lxwg7") || strings.Contains(text, "delete dev wg0") {
		t.Fatalf("orphan cleanup violated ownership: %s", text)
	}
}

func TestFailedPeerRevocationIsRetried(t *testing.T) {
	exists, failSync := false, false
	syncs := 0
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "ip -j -d link show"):
			if exists {
				return []byte(`[{"ifalias":"lucx-kernel-wireguard","linkinfo":{"info_kind":"wireguard"}}]`), nil
			}
			return nil, fmt.Errorf("absent")
		case strings.HasPrefix(cmd, "ip link add"):
			exists = true
		case strings.HasPrefix(cmd, "ip -j"):
			return []byte(`[]`), nil
		case strings.HasPrefix(cmd, "nft -j list table "):
			return nil, fmt.Errorf("absent")
		case strings.HasPrefix(cmd, "wg show"):
			return []byte("private public 51820 off\n"), nil
		case strings.HasPrefix(cmd, "wg syncconf"):
			syncs++
			if failSync {
				failSync = false
				return nil, fmt.Errorf("transient sync failure")
			}
		}
		return nil, nil
	}
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(t.TempDir(), command)
	if err = m.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	inst.Peers = nil
	failSync = true
	if err = m.Ensure(inst); err == nil {
		t.Fatal("failed revocation was reported as success")
	}
	if err = m.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	if syncs != 2 {
		t.Fatalf("failed revocation was not retried: syncs=%d", syncs)
	}
}

func TestAdoptedInterfaceRepairsAddressAndLinkState(t *testing.T) {
	var calls []string
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		calls = append(calls, cmd)
		if strings.HasPrefix(cmd, "ip -j -d link show") {
			return []byte(`[{"ifalias":"lucx-kernel-wireguard","linkinfo":{"info_kind":"wireguard"}}]`), nil
		}
		if strings.HasPrefix(cmd, "nft -j list table ") {
			return nil, fmt.Errorf("absent")
		}
		if strings.HasPrefix(cmd, "wg show") {
			return []byte("private public 51820 off\n"), nil
		}
		return nil, nil
	}
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	if err = NewManager(t.TempDir(), command).Ensure(inst); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(calls, "\n")
	if !strings.Contains(text, "ip address replace 10.90.0.1/24 dev lxwg7") || !strings.Contains(text, "ip link set dev lxwg7 mtu 1280 up") {
		t.Fatalf("incomplete owned device was not repaired: %s", text)
	}
}

func TestFailedCleanupIsRetriedWithoutAnInterface(t *testing.T) {
	exists, routePresent, failDelete := false, false, true
	command := func(input string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "ip -j -d link show"):
			if exists {
				return []byte(`[{"ifalias":"lucx-kernel-wireguard","linkinfo":{"info_kind":"wireguard"}}]`), nil
			}
			return nil, fmt.Errorf("absent")
		case cmd == "ip -j -4 route show table all":
			if routePresent {
				return []byte(`[{"table":51007,"protocol":"242","type":"local","dst":"default","dev":"lo"}]`), nil
			}
			return []byte(`[]`), nil
		case strings.HasPrefix(cmd, "ip -j"):
			return []byte(`[]`), nil
		case strings.HasPrefix(cmd, "ip link add"):
			exists = true
		case strings.HasPrefix(cmd, "ip link delete"):
			exists = false
		case strings.HasPrefix(cmd, "ip -4 route replace"):
			routePresent = true
		case strings.HasPrefix(cmd, "ip -4 route del"):
			if failDelete {
				failDelete = false
				return nil, fmt.Errorf("transient cleanup failure")
			}
			routePresent = false
		case strings.HasPrefix(cmd, "nft -j list table "):
			return nil, fmt.Errorf("absent")
		case strings.HasPrefix(cmd, "wg show"):
			return []byte("private public 51820 off\n"), nil
		}
		return nil, nil
	}
	inst, err := Parse(testInbound(t))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(t.TempDir(), command)
	if err = m.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	if err = m.Remove(inst.ID); err == nil {
		t.Fatal("cleanup failure was discarded")
	}
	if err = m.Remove(inst.ID); err != nil {
		t.Fatal(err)
	}
	if routePresent {
		t.Fatal("cleanup did not retry the orphan route")
	}
}

func TestFirewallOwnershipWorksWithoutTableComments(t *testing.T) {
	i := Instance{ID: 7}
	oldNFT := []byte(`{"nftables":[{"table":{"family":"inet","name":"lxwg_7"}},{"rule":{"family":"inet","table":"lxwg_7","chain":"forward","comment":"lucx-kernel-wireguard"}}]}`)
	if !ownedFirewall(oldNFT, i) {
		t.Fatal("rule ownership marker was ignored")
	}
	foreign := []byte(`{"nftables":[{"table":{"family":"inet","name":"lxwg_7"}},{"rule":{"family":"inet","table":"foreign","comment":"lucx-kernel-wireguard"}}]}`)
	if ownedFirewall(foreign, i) {
		t.Fatal("foreign table accepted")
	}
}
