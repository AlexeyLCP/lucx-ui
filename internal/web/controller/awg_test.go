// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
)

// TestAwgCPSBudget_MatchesSaveTimeGuard: the generator and ValidateIFields
// must never disagree, or "generate" hands back a set "save" then refuses.
func TestAwgCPSBudget_MatchesSaveTimeGuard(t *testing.T) {
	for _, withHPK := range []bool{false, true} {
		if got, want := awgCPSBudget(withHPK), awg.WorstCaseIBytesBudget(withHPK); got != want {
			t.Fatalf("awgCPSBudget(%v) = %d, want %d (ValidateIFields' own budget)", withHPK, got, want)
		}
	}
}
