// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os/exec"
	"strings"
)

// FirewallType represents the detected firewall backend.
type FirewallType string

const (
	FirewallNftables       FirewallType = "nft"
	FirewallIptablesLegacy FirewallType = "iptables-legacy"
)

// DetectFirewall determines whether to use nftables or iptables-legacy.
func DetectFirewall() FirewallType {
	if _, err := exec.LookPath("nft"); err == nil {
		out, err := exec.Command("nft", "list", "ruleset").Output()
		if err == nil && len(out) > 0 && strings.Contains(string(out), "table") {
			return FirewallNftables
		}
	}
	return FirewallIptablesLegacy
}

// Prerequisites holds the result of an environment check for AWG operation.
type Prerequisites struct {
	HasAWGModule bool         `json:"hasAWGModule"`
	HasIPRoute2  bool         `json:"hasIPRoute2"`
	HasIptables  bool         `json:"hasIptables"`
	IsRoot       bool         `json:"isRoot"`
	FirewallType FirewallType `json:"firewallType"`
	Errors       []string     `json:"errors"`
}

// CheckPrerequisites verifies all requirements for AWG operation.
// Tries modprobe if the amneziawg module is not loaded.
func CheckPrerequisites() *Prerequisites {
	p := &Prerequisites{FirewallType: DetectFirewall()}

	// Check/load amneziawg kernel module
	if out, err := exec.Command("lsmod").Output(); err == nil {
		if strings.Contains(string(out), "amneziawg") {
			p.HasAWGModule = true
		}
	}
	if !p.HasAWGModule {
		if err := exec.Command("modprobe", "amneziawg").Run(); err == nil {
			p.HasAWGModule = true
		} else {
			p.Errors = append(p.Errors, "amneziawg kernel module not available")
		}
	}

	// Check iproute2
	if _, err := exec.LookPath("ip"); err != nil {
		p.Errors = append(p.Errors, "iproute2 not found (required for ip rule/route)")
	} else {
		p.HasIPRoute2 = true
	}

	// Check firewall
	if p.FirewallType == FirewallIptablesLegacy {
		if _, err := exec.LookPath("iptables"); err != nil {
			p.Errors = append(p.Errors, "iptables not found")
		} else {
			p.HasIptables = true
		}
	} else {
		p.HasIptables = true // nft provides equivalent functionality
	}

	// Check root
	if out, err := exec.Command("id", "-u").Output(); err == nil && strings.TrimSpace(string(out)) == "0" {
		p.IsRoot = true
	} else {
		p.Errors = append(p.Errors, "root privileges required for AWG interface management")
	}

	return p
}

// OK returns true if all prerequisites are met.
func (p *Prerequisites) OK() bool {
	return len(p.Errors) == 0
}
