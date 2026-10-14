// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"
)

func TestCollectHostStatus_NonLinux(t *testing.T) {
	// On Windows CI/dev host the probe must not panic and should report empty.
	hs := CollectHostStatus()
	if hs.Interfaces < 0 {
		t.Fatalf("interfaces negative: %+v", hs)
	}
}
