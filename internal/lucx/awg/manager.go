// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

// AWGManager manages the full lifecycle of AWG inbounds.
type AWGManager struct {
	InboundService *service.InboundService
	XrayService    *service.XrayService
}

// NewAWGManager creates a new AWGManager.
func NewAWGManager(inboundSvc *service.InboundService, xraySvc *service.XrayService) *AWGManager {
	return &AWGManager{
		InboundService: inboundSvc,
		XrayService:    xraySvc,
	}
}

// =============================================================================
// Lifecycle: Create / Delete
// =============================================================================

// CreateAWGInbound sets up a complete AWG inbound: saves to DB, creates TUN
// child, writes config, sets up routing, and auto-creates the first client.
// Follows pumbaX convention: server + first peer in one step.
func (m *AWGManager) Create(awg *model.Inbound) (*model.Inbound, error) {
	// 1. Check prerequisites
	pre := CheckPrerequisites()
	if !pre.OK() {
		return nil, fmt.Errorf("prerequisites not met: %v", pre.Errors)
	}

	// 2. Read user parameters from inbound settings
	obfLevel := getIntFromSettings(awg.Settings, "obfLevel", 2)
	mimicryProfile := getStringFromSettings(awg.Settings, "mimicryProfile", "quic")
	region := getStringFromSettings(awg.Settings, "region", "ru")

	// 3. Generate AWG obfuscation params + CPS
	params, err := GenerateAWGParams(obfLevel, mimicryProfile, region)
	if err != nil {
		return nil, fmt.Errorf("generate params: %w", err)
	}
	i1, i2, i3, i4, i5 := GenerateCPS(obfLevel, CPSProfile(params.MimicryProfile))
	if err := ValidateAWGParams(params); err != nil {
		return nil, fmt.Errorf("validate params: %w", err)
	}

	// 4. Merge obfuscation into settings BEFORE AddInbound
	if err := MergeParamsToSettings(awg, params, i1, i2, i3, i4, i5); err != nil {
		return nil, fmt.Errorf("merge params: %w", err)
	}

	// 5. Save to DB
	awg, needRestart, err := m.InboundService.AddInbound(awg)
	if err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	awgID := awg.Id

	// 6. Compute network parameters
	iface := fmt.Sprintf("awg%d", awgID)
	tunIface := fmt.Sprintf("awg%dt", awgID)
	serverIP := fmt.Sprintf("10.%d.0.1", awgID%255)
	subnet := fmt.Sprintf("10.%d.0.0/24", awgID%255)
	rtTable := fmt.Sprintf("10%d", awgID)
	rtPref := 1000 + awgID

	// 7. Create child TUN inbound (invisible — ParentID set)
	if _, err := m.createTUNChild(awg, params, tunIface); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("tun child: %w", err)
	}

	// 8. Write PostUp/PostDown scripts + AWG config
	tmplData := TemplateData{
		AWGInterface:   iface,
		TUNInterface:   tunIface,
		AWGServerIP:    serverIP,
		AWGSubnet:      subnet,
		AWGPort:        awg.Port,
		RouteTable:     rtTable,
		RouteTableName: fmt.Sprintf("awg%d", awgID),
		RoutePref:      rtPref,
		MTU:            params.MTU,
	}
	if err := m.writeConfigFiles(awg, params, tmplData); err != nil {
		m.rollbackCreate(awg.Id)
		return nil, fmt.Errorf("write config: %w", err)
	}

	// 9. Setup routing (creates interface, iptables, routes) — idempotent
	routingCfg := RoutingConfig{
		AWGInterface: iface,
		TUNInterface: tunIface,
		AWGServerIP:  serverIP,
		AWGSubnet:    subnet,
		RouteTable:   rtTable,
		RoutePref:    rtPref,
		MTU:          params.MTU,
	}
	if err := SetupTUNRouting(routingCfg); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("routing: %w", err)
	}

	// 10. Auto-create first client (pumbaX: server + first peer in one step)
	if err := m.EnsureFirstClientExists(awg); err != nil {
		logAWG("Create: first client warning for inbound=%d: %v", awgID, err)
		// Non-fatal — inbound still works, user can add clients manually
	}

	// 11. Restart Xray if needed
	if needRestart {
		m.XrayService.SetToNeedRestart()
	}

	logAWG("Create: inbound=%d port=%d iface=%s tun=%s ok", awgID, awg.Port, iface, tunIface)
	return awg, nil
}

// DeleteAWGInbound tears down an AWG inbound and all its children.
func (m *AWGManager) Delete(awgID int) error {
	// 1. Run PostDown (best-effort)
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	if data, err := os.ReadFile(downPath); err == nil {
		runCmd("/bin/bash", "-c", string(data))
	}

	// 2. Cleanup routing
	cfg := RoutingConfig{
		AWGInterface: fmt.Sprintf("awg%d", awgID),
		TUNInterface: fmt.Sprintf("awg%dt", awgID),
		AWGSubnet:    fmt.Sprintf("10.%d.0.0/24", awgID%255),
		RouteTable:   fmt.Sprintf("10%d", awgID),
		RoutePref:    1000 + awgID,
	}
	CleanupTUNRouting(cfg)

	// 3. Delete child inbounds (TUN, SOCKS5)
	children, _ := m.InboundService.GetByParentId(awgID)
	for _, child := range children {
		m.InboundService.DelInbound(child.Id)
	}

	// 4. Delete from DB
	m.InboundService.DelInbound(awgID)

	// 5. Clean up files
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID)))
	os.Remove(downPath)

	// 6. Non-blocking Xray restart
	go func() {
		m.XrayService.SetToNeedRestart()
	}()

	logAWG("Delete: inbound=%d ok", awgID)
	return nil
}

// =============================================================================
// Repair
// =============================================================================

// RepairAWGInbound performs a full health check and repair on a single AWG inbound.
func (m *AWGManager) RepairAWGInbound(awgID int) *RepairResult {
	result := &RepairResult{InboundID: awgID}

	awg, err := m.InboundService.GetInbound(awgID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("get inbound: %v", err))
		return result
	}
	if awg.Protocol != model.AWG {
		result.Errors = append(result.Errors, "not an AWG inbound")
		return result
	}

	iface := fmt.Sprintf("awg%d", awgID)
	tunIface := fmt.Sprintf("awg%dt", awgID)
	subnet := fmt.Sprintf("10.%d.0.0/24", awgID%255)
	rtTable := fmt.Sprintf("10%d", awgID)

	// 1. Interface
	result.InterfaceOK = interfaceExists(iface)
	if !result.InterfaceOK {
		SetupAWGInterface(iface)
		result.InterfaceOK = interfaceExists(iface)
		if result.InterfaceOK {
			result.Fixed = append(result.Fixed, "interface created")
		}
	}

	// 2. TUN
	result.TUNOK = interfaceExists(tunIface)
	if !result.TUNOK {
		result.Fixed = append(result.Fixed, "tun missing (needs Xray restart)")
	}

	// 3. Routing
	result.RoutingOK = routeExists(subnet, rtTable)
	if !result.RoutingOK && result.InterfaceOK {
		mtu := getIntFromSettings(awg.Settings, "mtu", 1320)
		cfg := RoutingConfig{
			AWGInterface: iface, TUNInterface: tunIface,
			AWGServerIP: fmt.Sprintf("10.%d.0.1", awgID%255),
			AWGSubnet: subnet, RouteTable: rtTable,
			RoutePref: 1000 + awgID, MTU: mtu,
		}
		SetupTUNRouting(cfg)
		result.RoutingOK = routeExists(subnet, rtTable)
		if result.RoutingOK {
			result.Fixed = append(result.Fixed, "routing repaired")
		}
	}

	// 4. Firewall
	result.FirewallOK = iptablesRuleExists("FORWARD", iface, tunIface)
	if !result.FirewallOK && result.InterfaceOK {
		cfg := RoutingConfig{AWGInterface: iface, TUNInterface: tunIface, AWGSubnet: subnet}
		SetupTUNRouting(cfg)
		result.FirewallOK = iptablesRuleExists("FORWARD", iface, tunIface)
		if result.FirewallOK {
			result.Fixed = append(result.Fixed, "firewall repaired")
		}
	}

	// 5. Config file
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	_, err = os.Stat(confPath)
	result.ConfigOK = (err == nil)

	// 6. Clients
	clients, _ := m.InboundService.GetClients(awg)
	result.ClientKeysOK = len(clients) > 0
	if !result.ClientKeysOK {
		if err := m.EnsureFirstClientExists(awg); err == nil {
			result.ClientKeysOK = true
			result.Fixed = append(result.Fixed, "default client created")
		}
	}

	// 7. Obfuscation
	jc := getIntFromSettings(awg.Settings, "jc", 0)
	result.ObfuscationOK = jc > 0
	if !result.ObfuscationOK {
		if updated, _ := EnsureParams(awg); updated {
			m.InboundService.UpdateInbound(awg)
			result.ObfuscationOK = true
			result.Fixed = append(result.Fixed, "obfuscation repaired")
		}
	}

	logAWG("Repair: inbound=%d interface=%v tun=%v routing=%v fw=%v config=%v clients=%v obfs=%v fixed=%v",
		awgID, result.InterfaceOK, result.TUNOK, result.RoutingOK,
		result.FirewallOK, result.ConfigOK, result.ClientKeysOK,
		result.ObfuscationOK, result.Fixed)
	return result
}

// RepairAllAWGInbounds repairs every AWG inbound and returns results keyed by ID.
// Called at panel startup (web.go).
func (m *AWGManager) RepairAllAWGInbounds() map[int]*RepairResult {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		logAWG("RepairAll: db error: %v", err)
		return nil
	}

	results := make(map[int]*RepairResult)
	for _, ib := range inbounds {
		results[ib.Id] = m.RepairAWGInbound(ib.Id)
	}
	repaired := 0
	for _, r := range results {
		if len(r.Fixed) > 0 {
			repaired++
		}
	}
	logAWG("RepairAll: %d inbounds checked, %d repaired", len(inbounds), repaired)
	return results
}

// =============================================================================
// Helpers
// =============================================================================

func (m *AWGManager) createTUNChild(awg *model.Inbound, params *AWGParams, tunName string) (*model.Inbound, error) {
	awgID := awg.Id
	tunSubnet := pickFreeTunSubnet(awgID)

	settings := jsonMarshal(map[string]interface{}{
		"name":    tunName,
		"address": []string{tunSubnet},
		"stack":   "system",
		"mtu":     params.MTU,
	})

	tunInbound := &model.Inbound{
		UserId:   awg.UserId,
		NodeID:   awg.NodeID,
		ParentID: &awgID, // makes it invisible — filtered by frontend
		Protocol: model.TUN,
		Tag:      fmt.Sprintf("awg-tun-%d", awgID),
		Port:     0,
		Settings: settings,
		Enable:   true,
	}

	_, _, err := m.InboundService.AddInbound(tunInbound)
	if err != nil {
		return nil, err
	}
	logAWG("createTUNChild: parent=%d tun=%s subnet=%s", awgID, tunName, tunSubnet)
	return tunInbound, nil
}

func (m *AWGManager) writeConfigFiles(awg *model.Inbound, params *AWGParams, data TemplateData) error {
	awgID := awg.Id

	if err := os.MkdirAll(awgConfigDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", awgConfigDir, err)
	}

	postUp, err := RenderPostUp(data)
	if err != nil {
		return fmt.Errorf("render PostUp: %w", err)
	}
	postDown, err := RenderPostDown(data)
	if err != nil {
		return fmt.Errorf("render PostDown: %w", err)
	}

	upPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID))
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	if err := os.WriteFile(upPath, []byte(postUp), 0755); err != nil {
		return fmt.Errorf("write %s: %w", upPath, err)
	}
	if err := os.WriteFile(downPath, []byte(postDown), 0755); err != nil {
		return fmt.Errorf("write %s: %w", downPath, err)
	}

	logAWG("BuildServerConfig: Jc=%d Jmin=%d Jmax=%d S1=%d S2=%d S3=%d S4=%d H1=%s",
		params.Jc, params.Jmin, params.Jmax, params.S1, params.S2, params.S3, params.S4, params.H1)

	conf := BuildServerConfig(awg, params, data, upPath, downPath)
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	if err := os.WriteFile(confPath, []byte(conf), 0600); err != nil {
		return fmt.Errorf("write %s: %w", confPath, err)
	}

	appendToFile("/etc/iproute2/rt_tables", fmt.Sprintf("%d %s\n", 100+awgID, data.RouteTableName))

	logAWG("writeConfig: inbound=%d conf=%s up=%s down=%s", awgID, confPath, upPath, downPath)
	return nil
}

func (m *AWGManager) rollbackCreate(awgID int) {
	m.InboundService.DelInbound(awgID)
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID)))
}
