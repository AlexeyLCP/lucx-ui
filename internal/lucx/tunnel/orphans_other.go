// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build !linux

package tunnel

// killStrayTunnelProcesses is a no-op off Linux. On Windows the kill-on-exit
// job object already terminates every core together with the panel (see
// attachChildLifetime), so orphans do not arise there; other platforms are not
// a supported deployment target for the tunnel sidecars.
func killStrayTunnelProcesses(_ []string) int { return 0 }
