// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
)

// kernelBudget is what the save-time guard charges a generated set on a stock
// kernel interface — the same call the panel makes before writing the .conf.
func kernelBudget() int { return awg.IBytesBudget(awg.BaselineIfname, false) }

// The generator must never hand back a set the save-time guard would reject,
// and never quietly hand back fewer than the five fields that were asked for:
// both turn "generate, then save" into a dead end for the operator.
func TestGenerateCPS_FitsBudgetOrRefuses(t *testing.T) {
	budget := kernelBudget()

	cases := []struct {
		profile MimicryProfile
		browser BrowserProfile
		fits    bool
	}{
		{ProfileDNS, BrowserChrome, true},
		{ProfileTLS, BrowserChrome, true},
		{ProfileTLS, BrowserFirefox, true},
		{ProfileTLS, BrowserSafari, true},
		{ProfileQUIC, BrowserChrome, true},
		{ProfileQUIC, BrowserFirefox, true},
		{ProfileQUIC, BrowserSafari, true},
		{ProfileSIP, BrowserChrome, true},
	}

	for _, c := range cases {
		for _, reg := range []Region{RegionWorld, RegionRU} {
			t.Run(string(c.profile)+"/"+string(c.browser)+"/"+string(reg), func(t *testing.T) {
				draws := 100
				if !c.fits {
					draws = 3 // a refusal costs a full retry sweep
				}
				for i := 0; i < draws; i++ {
					r, err := GenerateCPS(c.profile, reg, "", c.browser, false, budget)
					if !c.fits {
						if !errors.Is(err, ErrCPSBudgetExceeded) {
							t.Fatalf("draw %d: want ErrCPSBudgetExceeded, got err=%v, %d bytes", i, err, r.IBytes())
						}
						continue
					}
					if err != nil {
						t.Fatalf("draw %d: unexpected error: %v", i, err)
					}
					if got := r.IBytes(); got > budget {
						t.Fatalf("draw %d: %d bytes > budget %d — the save guard rejects it", i, got, budget)
					}
					for n, v := range map[string]string{"I1": r.I1, "I2": r.I2, "I3": r.I3, "I4": r.I4, "I5": r.I5} {
						if v == "" {
							t.Fatalf("draw %d: %s empty — a full set was asked for and silently shrunk", i, n)
						}
					}
				}
			})
		}
	}
}

// A header-protection key costs 36 netlink bytes, so an AWG3 host has less room
// for I1-I5. Generating against the wider budget makes the save-time guard
// reject the panel's own output.
func TestGenerateCPS_HonoursCallerBudget(t *testing.T) {
	budget := awg.IBytesBudget(awg.BaselineIfname, true)
	if wide := awg.IBytesBudget(awg.BaselineIfname, false); budget >= wide {
		t.Fatalf("header-protection key must narrow the budget: %d vs %d", budget, wide)
	}
	for i := 0; i < 100; i++ {
		r, err := GenerateCPS(ProfileQUIC, RegionWorld, "", BrowserChrome, false, budget)
		if errors.Is(err, ErrCPSBudgetExceeded) {
			continue
		}
		if err != nil {
			t.Fatalf("draw %d: unexpected error: %v", i, err)
		}
		if got := r.IBytes(); got > budget {
			t.Fatalf("draw %d: %d bytes > caller budget %d", i, got, budget)
		}
	}
}

// The single-field sets behind the Lite and Standard profiles have always fit;
// they must keep fitting, and must not start carrying I2-I5.
func TestGenerateCPS_SingleFieldAlwaysFits(t *testing.T) {
	budget := kernelBudget()
	for _, p := range []MimicryProfile{ProfileTLS, ProfileQUIC, ProfileSIP, ProfileDNS} {
		for _, br := range []BrowserProfile{BrowserChrome, BrowserFirefox, BrowserSafari} {
			t.Run(string(p)+"/"+string(br), func(t *testing.T) {
				for i := 0; i < 50; i++ {
					r, err := GenerateCPS(p, RegionWorld, "", br, true, budget)
					if err != nil {
						t.Fatalf("draw %d: unexpected error: %v", i, err)
					}
					if got := r.IBytes(); got > budget {
						t.Fatalf("draw %d: %d bytes > budget %d", i, got, budget)
					}
					if r.I2 != "" || r.I3 != "" || r.I4 != "" || r.I5 != "" {
						t.Fatalf("draw %d: onlyI1 set carries I2-I5", i)
					}
				}
			})
		}
	}
}

// The refusal path still has to work: no profile busts the budget any more, so
// only a caller with less room can force it.
func TestGenerateCPS_RefusesWhenTheBudgetCannotFitASession(t *testing.T) {
	_, err := GenerateCPS(ProfileQUIC, RegionWorld, "", BrowserChrome, false, 200)
	if !errors.Is(err, ErrCPSBudgetExceeded) {
		t.Fatalf("want ErrCPSBudgetExceeded, got %v", err)
	}
}
