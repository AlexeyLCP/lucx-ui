//go:build !linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

// defaultRouteInterface is a no-op off Linux. AWG is a Linux kernel module;
// other platforms are not a supported deployment target for the AWG sidecar,
// so NAT setup is unnecessary there.
func defaultRouteInterface() string { return "" }

// killStrayAwgInterfaces is a no-op off Linux. AWG is a Linux kernel module;
// other platforms are not a supported deployment target for the AWG sidecar,
// so orphan sweeping is unnecessary there. There is no tun2socks daemon to
// sweep — routing into Xray is via an injected TUN inbound owned by Xray.
func killStrayAwgInterfaces() int { return 0 }

// ModuleSupportsAwg3 is a no-op off Linux (AWG is a Linux kernel module).
// Dev/build hosts return false so renderers degrade AWG3 fields gracefully.
// Tests override via SetModuleSupportsAwg3 to simulate a v3-capable module.
var moduleSupportsAwg3Override *bool

func ModuleSupportsAwg3() bool {
	if moduleSupportsAwg3Override != nil {
		return *moduleSupportsAwg3Override
	}
	return false
}

// SetModuleSupportsAwg3 overrides the module-support probe for tests.
// Pass nil to clear the override (restore real probing). Test-only helper.
func SetModuleSupportsAwg3(supported *bool) {
	moduleSupportsAwg3Override = supported
}

var moduleSupportsAwg31Override *bool

func ModuleSupportsAwg31() bool {
	if moduleSupportsAwg31Override != nil {
		return *moduleSupportsAwg31Override
	}
	return false
}

func SetModuleSupportsAwg31(supported *bool) {
	moduleSupportsAwg31Override = supported
}

// awg3CapabilityCheck is a stub off Linux: AWG is a Linux kernel module, so
// AWG3 (HeaderProtectionKey) support is never present on dev hosts.
func awg3CapabilityCheck(prober) DiagCheck {
	return DiagCheck{awg3SupportCheckName, false, "AWG is a Linux-only kernel module"}
}

// awg31CapabilityCheck is a stub off Linux, same as awg3CapabilityCheck.
func awg31CapabilityCheck(prober) DiagCheck {
	return DiagCheck{awg31SupportCheckName, false, "AWG is a Linux-only kernel module"}
}
