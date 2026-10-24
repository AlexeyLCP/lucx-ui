// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build !linux

package wireguard

// No-op firewall stubs: the kernel WireGuard sidecar (and wgobfs) only run
// on Linux hosts; dev boxes and Windows builds keep the same API surface.

// EnsureWgoFirewall is a no-op off Linux.
func EnsureWgoFirewall(WgoFirewallState) error { return nil }

// SyncWgoFirewall is a no-op off Linux.
func SyncWgoFirewall([]WgoFirewallState) {}
