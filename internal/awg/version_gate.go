// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

// AwgVersionFieldsAllowed decides whether an AWG artifact may carry 3.x fields;
// hostSupports only speaks for a locally-hosted inbound.
func AwgVersionFieldsAllowed(versionAsks, localInbound, hostSupports bool) bool {
	if !versionAsks {
		return false
	}
	if !localInbound {
		return true
	}
	return hostSupports
}
