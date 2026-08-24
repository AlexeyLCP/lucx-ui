// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import "testing"

func TestKernelAvailable_RetriesFalseThenCachesTrue(t *testing.T) {
	ResetKernelAvailableForTest()
	t.Cleanup(ResetKernelAvailableForTest)

	calls := 0
	probeKernel = func() bool {
		calls++
		return calls >= 2
	}

	if KernelAvailable() {
		t.Fatal("first probe false must not cache as available")
	}
	if !KernelAvailable() {
		t.Fatal("second probe true must flip to available")
	}
	if !KernelAvailable() {
		t.Fatal("positive result must stick")
	}
	if calls != 2 {
		t.Fatalf("probe calls = %d, want 2 (false retried, true cached)", calls)
	}
}
