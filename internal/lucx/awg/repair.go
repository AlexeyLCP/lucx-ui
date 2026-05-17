// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// RepairResult reports the health of an AWG inbound and what was fixed.
type RepairResult struct {
	InterfaceOK   bool     `json:"interface_ok"`
	TUNOK         bool     `json:"tun_ok"`
	RoutingOK     bool     `json:"routing_ok"`
	FirewallOK    bool     `json:"firewall_ok"`
	ConfigOK      bool     `json:"config_ok"`
	ClientKeysOK  bool     `json:"client_keys_ok"`
	ObfuscationOK bool     `json:"obfuscation_ok"`
	Fixed         []string `json:"fixed"`
}

// Repair performs a full health check and repair on an AWG inbound.
func (m *AWGManager) Repair(awgID int) (*RepairResult, error) {
	result := &RepairResult{Fixed: []string{}}

	awg, err := m.InboundService.GetInbound(awgID)
	if err != nil {
		return nil, fmt.Errorf("get inbound: %w", err)
	}
	if awg.Protocol != model.AWG {
		return nil, fmt.Errorf("not an AWG inbound: %s", awg.Protocol)
	}

	iface := fmt.Sprintf("awg%d", awgID)
	tunIface := fmt.Sprintf("awg%dt", awgID)

	// 1. Check AWG interface
	result.InterfaceOK = interfaceExists(iface)
	if !result.InterfaceOK {
		// Try to create it
		exec.Command("ip", "link", "add", iface, "type", "amneziawg").Run()
		result.InterfaceOK = interfaceExists(iface)
		if result.InterfaceOK {
			result.Fixed = append(result.Fixed, "interface created")
		}
	}

	// 2. Check TUN interface
	result.TUNOK = interfaceExists(tunIface)
	if !result.TUNOK {
		result.Fixed = append(result.Fixed, "tun interface missing (needs Xray restart)")
	}

	// 3. Check routing
	subnet := fmt.Sprintf("10.%d.0.0/24", awgID%255)
	rtTable := fmt.Sprintf("10%d", awgID)
	result.RoutingOK = routeExists(subnet, rtTable)
	if !result.RoutingOK && result.InterfaceOK {
		cfg := RoutingConfig{
			AWGInterface: iface,
			TUNInterface: tunIface,
			AWGServerIP:  fmt.Sprintf("10.%d.0.1", awgID%255),
			AWGSubnet:    subnet,
			RouteTable:   rtTable,
			RoutePref:    1000 + awgID,
			MTU:          getIntFromSettings(awg.Settings, "mtu", 1320),
		}
		SetupTUNRouting(cfg)
		result.RoutingOK = routeExists(subnet, rtTable)
		if result.RoutingOK {
			result.Fixed = append(result.Fixed, "routing repaired")
		}
	}

	// 4. Check firewall
	result.FirewallOK = iptablesRuleExists("FORWARD", iface, tunIface)
	if !result.FirewallOK && result.InterfaceOK {
		cfg := RoutingConfig{
			AWGInterface: iface,
			TUNInterface: tunIface,
			AWGSubnet:    subnet,
		}
		SetupTUNRouting(cfg)
		result.FirewallOK = iptablesRuleExists("FORWARD", iface, tunIface)
		if result.FirewallOK {
			result.Fixed = append(result.Fixed, "firewall repaired")
		}
	}

	// 5. Check config file
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	_, err = os.Stat(confPath)
	result.ConfigOK = (err == nil)

	// 6. Check clients
	clients, _ := m.InboundService.GetClients(awg)
	result.ClientKeysOK = len(clients) > 0
	if !result.ClientKeysOK {
		_ = m.EnsureFirstClient(awg)
		result.ClientKeysOK = true
		result.Fixed = append(result.Fixed, "default client created")
	}

	// 7. Check obfuscation
	jc := getIntFromSettings(awg.Settings, "jc", 0)
	result.ObfuscationOK = jc > 0
	if !result.ObfuscationOK {
		if updated, _ := m.EnsureParams(awg); updated {
			m.InboundService.UpdateInbound(awg)
			result.ObfuscationOK = true
			result.Fixed = append(result.Fixed, "obfuscation repaired")
		}
	}

	return result, nil
}

// RepairAllInbounds performs Repair on every AWG inbound.
func (m *AWGManager) RepairAllInbounds() (map[int]*RepairResult, error) {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		return nil, err
	}

	results := make(map[int]*RepairResult)
	for _, ib := range inbounds {
		result, err := m.Repair(ib.Id)

		if err != nil {
			result = &RepairResult{Fixed: []string{fmt.Sprintf("error: %v", err)}}
		}
		results[ib.Id] = result
	}
	return results, nil
}

// --- Repair helpers ---

func routeExists(subnet, table string) bool {
	out, err := exec.Command("ip", "route", "show", "table", table).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), subnet)
}

func iptablesRuleExists(chain, iface1, iface2 string) bool {
	err := exec.Command("iptables", "-C", chain, "-i", iface1, "-o", iface2, "-j", "ACCEPT").Run()
	return err == nil
}
