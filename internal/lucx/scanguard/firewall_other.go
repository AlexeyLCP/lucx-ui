// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build !linux

package scanguard

import "net/netip"

// No-op firewall stubs: the scanner guard only manipulates netfilter on
// Linux hosts; dev boxes and Windows builds keep the same API surface.

func applyToFirewall([]netip.Prefix, []netip.Prefix, []int) error { return ErrUnsupported }

func ensureRules([]int) error { return ErrUnsupported }

func removeFirewall() error { return ErrUnsupported }

func probeStatus([]int) ProbedStatus { return ProbedStatus{} }
