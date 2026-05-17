// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

// =============================================================================
// Core Configuration Types
// =============================================================================

// AWGConfig holds the complete server-side AWG interface configuration.
type AWGConfig struct {
	// Interface section
	PrivateKey string
	Address    string // server IP/CIDR, e.g. "10.0.0.1/24"
	ListenPort int
	MTU        int

	// Obfuscation (all values validated by ValidateAWGParams)
	Jc   int    `json:"jc"`
	Jmin int    `json:"jmin"`
	Jmax int    `json:"jmax"`
	S1   int    `json:"s1"`
	S2   int    `json:"s2"`
	S3   int    `json:"s3"`
	S4   int    `json:"s4"`
	H1   string `json:"h1"`
	H2   string `json:"h2"`
	H3   string `json:"h3"`
	H4   string `json:"h4"`
	I1   string `json:"i1,omitempty"`
	I2   string `json:"i2,omitempty"`
	I3   string `json:"i3,omitempty"`
	I4   string `json:"i4,omitempty"`
	I5   string `json:"i5,omitempty"`

	ObfLevel       int    `json:"obfLevel"`
	MimicryProfile string `json:"mimicryProfile"`
	Region         string `json:"region"`

	// Routing
	PostUp   string
	PostDown string

	// DNS for clients
	DNS string
}

// AWGClient represents a single peer/client on an AWG interface.
type AWGClient struct {
	Email      string `json:"email"`
	ID         string `json:"id"`         // public key
	PrivateKey string `json:"privateKey"` // client private key
	Password   string `json:"password"`   // pre-shared key (PSK)
	Address    string `json:"address"`    // client IP/CIDR
	Enable     bool   `json:"enable"`
	ExpiryTime int64  `json:"expiryTime"`
	TgID       string `json:"tgId"`
	SubID      string `json:"subId"`
	Comment    string `json:"comment"`
}

// AWGInterface describes a network interface for AWG.
type AWGInterface struct {
	Name    string // e.g. "awg1"
	TUNName string // e.g. "awg1t" (child TUN, invisible to user)
	ID      int    // numeric ID extracted from name
}

// =============================================================================
// Routing Types
// =============================================================================

// RoutingConfig holds all parameters for TUN routing setup.
type RoutingConfig struct {
	AWGInterface string
	TUNInterface string
	AWGServerIP  string
	AWGSubnet    string
	RouteTable   string
	RoutePref    int
	MTU          int
}

// =============================================================================
// Repair Types
// =============================================================================

// RepairResult reports the health of an AWG inbound and what was fixed.
type RepairResult struct {
	InboundID     int      `json:"inbound_id"`
	InterfaceOK   bool     `json:"interface_ok"`
	TUNOK         bool     `json:"tun_ok"`
	RoutingOK     bool     `json:"routing_ok"`
	FirewallOK    bool     `json:"firewall_ok"`
	ConfigOK      bool     `json:"config_ok"`
	ClientKeysOK  bool     `json:"client_keys_ok"`
	ObfuscationOK bool     `json:"obfuscation_ok"`
	Fixed         []string `json:"fixed,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

func (r *RepairResult) AllOK() bool {
	return r.InterfaceOK && r.TUNOK && r.RoutingOK &&
		r.FirewallOK && r.ConfigOK && r.ClientKeysOK &&
		r.ObfuscationOK && len(r.Errors) == 0
}

// =============================================================================
// Install Types
// =============================================================================

// InstallResult reports the outcome of an AWG installation attempt.
type InstallResult struct {
	KernelModule bool
	Tools        bool
	RebootNeeded bool
	Log          string
}

// =============================================================================
// Package Constants
// =============================================================================

const (
	awgConfigDir = "/etc/amnezia/amneziawg"
)
