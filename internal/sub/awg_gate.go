// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

// awgVersionFieldsAllowed decides whether a share link may carry AWG 3.x-only
// fields. The server side gates them on the host's tools support, so emitting
// one the server dropped hands the client a config it cannot connect with —
// RandomTrailers in particular is checked against the receiver's own flag.
//
// hostSupports is this process's probe, which only describes a locally-hosted
// inbound. A node's capability is not stored anywhere the master can read, so
// for those the version alone still decides, exactly as before.
func awgVersionFieldsAllowed(versionAsks, localInbound, hostSupports bool) bool {
	if !versionAsks {
		return false
	}
	if !localInbound {
		return true
	}
	return hostSupports
}
