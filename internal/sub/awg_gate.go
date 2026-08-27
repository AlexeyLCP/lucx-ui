// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import "github.com/mhsanaei/3x-ui/v3/internal/awg"

// awgVersionFieldsAllowed wraps awg.AwgVersionFieldsAllowed so this package's tests need no changes.
func awgVersionFieldsAllowed(versionAsks, localInbound, hostSupports bool) bool {
	return awg.AwgVersionFieldsAllowed(versionAsks, localInbound, hostSupports)
}
