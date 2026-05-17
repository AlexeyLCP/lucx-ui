// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// BuildServerConfig generates the complete AWG server .conf file content.
// Includes all obfuscation params in [Interface] section.
func BuildServerConfig(awg *model.Inbound, params *AWGParams, data TemplateData, upPath, downPath string) string {
	var b strings.Builder

	// Obfuscation CPS probes — separate from AWGParams, stored in inbound settings
	i1 := getStringFromSettings(awg.Settings, "i1", "")
	i2 := getStringFromSettings(awg.Settings, "i2", "")
	i3 := getStringFromSettings(awg.Settings, "i3", "")
	i4 := getStringFromSettings(awg.Settings, "i4", "")
	i5 := getStringFromSettings(awg.Settings, "i5", "")

	i1Line, i2Line, i3Line, i4Line, i5Line := "", "", "", "", ""
	if i1 != "" {
		i1Line = fmt.Sprintf("I1 = <b 0x%s>\n", i1)
		i2Line = fmt.Sprintf("I2 = <b 0x%s>\n", i2)
	}
	if i3 != "" {
		i3Line = fmt.Sprintf("I3 = <b 0x%s>\n", i3)
		i4Line = fmt.Sprintf("I4 = <b 0x%s>\n", i4)
		i5Line = fmt.Sprintf("I5 = <b 0x%s>\n", i5)
	}

	fmt.Fprintf(&b, `[Interface]
PrivateKey = %s
Address = %s/24
ListenPort = %d
MTU = %d
Jc = %d
Jmin = %d
Jmax = %d
S1 = %d
S2 = %d
S3 = %d
S4 = %d
H1 = %s
H2 = %s
H3 = %s
H4 = %s
%s%s%s%s%sPostUp = %s
PostDown = %s
`,
		params.PrivateKey, data.AWGServerIP, awg.Port, params.MTU,
		params.Jc, params.Jmin, params.Jmax,
		params.S1, params.S2, params.S3, params.S4,
		params.H1, params.H2, params.H3, params.H4,
		i1Line, i2Line, i3Line, i4Line, i5Line,
		upPath, downPath,
	)

	return b.String()
}

// BuildClientConfig generates a client .conf file for a single peer.
// Includes ALL obfuscation params — client needs them for DPI bypass.
func BuildClientConfig(cfg AWGConfig, client AWGClient) string {
	var b strings.Builder

	if !client.Enable {
		b.WriteString("# DISABLED CLIENT\n")
	}

	fmt.Fprintf(&b, "# %s\n", client.Email)
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", client.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", client.Address)
	if cfg.DNS != "" {
		fmt.Fprintf(&b, "DNS = %s\n", cfg.DNS)
	} else {
		b.WriteString("DNS = 1.1.1.1, 1.0.0.1\n")
	}
	fmt.Fprintf(&b, "MTU = %d\n", cfg.MTU)

	// All obfuscation params — critical for DPI bypass
	fmt.Fprintf(&b, "Jc = %d\n", cfg.Jc)
	fmt.Fprintf(&b, "Jmin = %d\n", cfg.Jmin)
	fmt.Fprintf(&b, "Jmax = %d\n", cfg.Jmax)
	fmt.Fprintf(&b, "S1 = %d\n", cfg.S1)
	fmt.Fprintf(&b, "S2 = %d\n", cfg.S2)
	fmt.Fprintf(&b, "S3 = %d\n", cfg.S3)
	fmt.Fprintf(&b, "S4 = %d\n", cfg.S4)
	fmt.Fprintf(&b, "H1 = %s\n", cfg.H1)
	fmt.Fprintf(&b, "H2 = %s\n", cfg.H2)
	fmt.Fprintf(&b, "H3 = %s\n", cfg.H3)
	fmt.Fprintf(&b, "H4 = %s\n", cfg.H4)
	if cfg.I1 != "" {
		fmt.Fprintf(&b, "I1 = <b 0x%s>\n", cfg.I1)
		fmt.Fprintf(&b, "I2 = <b 0x%s>\n", cfg.I2)
	}
	if cfg.I3 != "" {
		fmt.Fprintf(&b, "I3 = <b 0x%s>\n", cfg.I3)
		fmt.Fprintf(&b, "I4 = <b 0x%s>\n", cfg.I4)
		fmt.Fprintf(&b, "I5 = <b 0x%s>\n", cfg.I5)
	}

	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", client.ID)
	fmt.Fprintf(&b, "PresharedKey = %s\n", client.Password)
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", cfg.Address, cfg.ListenPort)
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	b.WriteString("PersistentKeepalive = 25\n")

	return b.String()
}

// ValidateAWGConfig checks that all obfuscation parameters are present and valid.
// Returns nil if the config is complete, or an error describing what's missing.
func ValidateAWGConfig(cfg AWGConfig) error {
	var missing []string
	if cfg.Jc == 0 {
		missing = append(missing, "Jc")
	}
	if cfg.Jmin == 0 {
		missing = append(missing, "Jmin")
	}
	if cfg.Jmax == 0 {
		missing = append(missing, "Jmax")
	}
	if cfg.S1 == 0 {
		missing = append(missing, "S1")
	}
	if cfg.H1 == "" {
		missing = append(missing, "H1")
	}
	if cfg.PrivateKey == "" {
		missing = append(missing, "PrivateKey")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing AWG config fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// jsonMarshal is a small helper for inline JSON serialization.
func jsonMarshal(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
