// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// =============================================================================
// Standalone Repair (called from controllers without AWGManager)
// =============================================================================

// RepairAWGOnGet checks and repairs a single AWG inbound when it is loaded.
// Safe to call on every GetInbound; only modifies if params are missing.
// Returns the (possibly modified) inbound and whether it was repaired.
func RepairAWGOnGet(inbound *model.Inbound) (*model.Inbound, bool) {
	if inbound.Protocol != model.AWG {
		return inbound, false
	}

	jc := getIntFromSettings(inbound.Settings, "jc", 0)
	i1 := getStringFromSettings(inbound.Settings, "i1", "")

	needsRepair := jc == 0 || (jc == 8 && getIntFromSettings(inbound.Settings, "jmin", 0) == 50)
	if !needsRepair && i1 == "" {
		obfLevel := getIntFromSettings(inbound.Settings, "obfLevel", 2)
		if obfLevel >= 2 {
			needsRepair = true // level 2+ should have I1
		}
	}
	if !needsRepair {
		return inbound, false
	}

	logAWG("RepairAWGOnGet: inbound %d needs repair (jc=%d i1=%s)", inbound.Id, jc, i1)

	obfLevel := getIntFromSettings(inbound.Settings, "obfLevel", 2)
	mimicryProfile := getStringFromSettings(inbound.Settings, "mimicryProfile", "quic")
	region := getStringFromSettings(inbound.Settings, "region", "ru")

	params, err := GenerateAWGParams(obfLevel, mimicryProfile, region)
	if err != nil {
		logAWG("RepairAWGOnGet: generate failed: %v", err)
		return inbound, false
	}
	i1v, i2, i3, i4, i5 := GenerateCPS(obfLevel, CPSProfile(params.MimicryProfile))

	if err := ValidateAWGParams(params); err != nil {
		logAWG("RepairAWGOnGet: validate failed: %v", err)
		return inbound, false
	}
	if err := MergeParamsToSettings(inbound, params, i1v, i2, i3, i4, i5); err != nil {
		logAWG("RepairAWGOnGet: merge failed: %v", err)
		return inbound, false
	}
	logAWG("RepairAWGOnGet: inbound %d repaired", inbound.Id)
	return inbound, true
}

// EnsureParams regenerates obfuscation params for an AWG inbound if missing.
// Standalone version — does not require AWGManager.
func EnsureParams(inbound *model.Inbound) (bool, error) {
	if inbound.Protocol != model.AWG {
		return false, nil
	}
	jc := getIntFromSettings(inbound.Settings, "jc", 0)
	jmin := getIntFromSettings(inbound.Settings, "jmin", 0)
	if jc > 0 && jmin > 0 {
		return false, nil
	}

	logAWG("EnsureParams: inbound %d missing obfuscation, regenerating", inbound.Id)
	obfLevel := getIntFromSettings(inbound.Settings, "obfLevel", 2)
	mimicryProfile := getStringFromSettings(inbound.Settings, "mimicryProfile", "quic")
	region := getStringFromSettings(inbound.Settings, "region", "ru")

	params, err := GenerateAWGParams(obfLevel, mimicryProfile, region)
	if err != nil {
		return false, fmt.Errorf("generate params: %w", err)
	}
	i1, i2, i3, i4, i5 := GenerateCPS(obfLevel, CPSProfile(params.MimicryProfile))

	if err := ValidateAWGParams(params); err != nil {
		return false, fmt.Errorf("validate params: %w", err)
	}
	if err := MergeParamsToSettings(inbound, params, i1, i2, i3, i4, i5); err != nil {
		return false, fmt.Errorf("merge params: %w", err)
	}
	return true, nil
}
