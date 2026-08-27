// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"fmt"
	"strings"
	"testing"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/tuntest"
)

// A generated .conf is imported by Amnezia clients, which run amneziawg-go,
// while the server runs the kernel module. ipcSet drives the userspace engine's
// own parser, in the config shape internal/amneziawgnet writes.
func ipcSet(t *testing.T, i [5]string) error {
	t.Helper()
	var b strings.Builder
	b.WriteString("private_key=" + strings.Repeat("ab", 32) + "\n")
	b.WriteString("listen_port=51820\n")
	b.WriteString("jc=4\njmin=40\njmax=70\n")
	b.WriteString("s1=15\ns2=30\ns3=20\ns4=25\n")
	for n, v := range i {
		if v != "" {
			fmt.Fprintf(&b, "i%d=%s\n", n+1, v)
		}
	}
	tun := tuntest.NewChannelTUN()
	dev := device.NewDevice(tun.TUN(), awgconn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	t.Cleanup(dev.Close)
	return dev.IpcSet(b.String())
}

// Every tag Descriptor offers must be understood by the userspace engine too.
// A tag only the kernel implements makes the server work and the client fail.
func TestDescriptor_AcceptedByUserspaceEngine(t *testing.T) {
	set := [5]string{
		Descriptor{}.Lit([]byte{0x16, 0x03, 0x01}).Rand(64).String(),
		Descriptor{}.Rand(32).String(),
		Descriptor{}.RandChars(16).String(),
		Descriptor{}.RandDigits(16).String(),
		Descriptor{}.Lit([]byte{0x47, 0x45, 0x54}).Timestamp().Rand(8).Lit([]byte{0x0D, 0x0A}).String(),
	}
	if err := ipcSet(t, set); err != nil {
		t.Fatalf("amneziawg-go rejected a descriptor this generator can emit: %v\nset: %q", err, set)
	}
}

// The canary that gives the test above its teeth: <c> is kernel-only, and the
// userspace engine aborts IpcSetOperation before mergeWithDevice, so a client
// tunnel does not come up at all. If this stops failing, ipcSet stopped
// validating and the test above proves nothing.
func TestUserspaceEngine_RejectsKernelOnlyCounterTag(t *testing.T) {
	err := ipcSet(t, [5]string{"<c>"})
	if err == nil {
		t.Fatal("amneziawg-go accepted <c>; the cross-engine check is no longer validating anything")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "tag") {
		t.Logf("rejected as expected, message not tag-shaped: %v", err)
	}
}
