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

// Create sets up a complete AWG inbound: saves to DB, creates SOCKS5 child,
// writes config + PostUp/PostDown scripts, executes PostUp, auto-creates first client.
func (m *AWGManager) Create(awg *model.Inbound) (*model.Inbound, error) {
	pre := CheckPrerequisites()
	if !pre.OK() {
		return nil, fmt.Errorf("prerequisites not met: %v", pre.Errors)
	}

	obfLevel := getIntFromSettings(awg.Settings, "obfLevel", 2)
	mimicryProfile := getStringFromSettings(awg.Settings, "mimicryProfile", "quic")
	region := getStringFromSettings(awg.Settings, "region", "ru")

	params, err := GenerateAWGParams(obfLevel, mimicryProfile, region)
	if err != nil {
		return nil, fmt.Errorf("generate params: %w", err)
	}
	i1, i2, i3, i4, i5 := GenerateCPS(obfLevel, CPSProfile(params.MimicryProfile))
	if err := ValidateAWGParams(params); err != nil {
		return nil, fmt.Errorf("validate params: %w", err)
	}

	if err := MergeParamsToSettings(awg, params, i1, i2, i3, i4, i5); err != nil {
		return nil, fmt.Errorf("merge params: %w", err)
	}

	awg, needRestart, err := m.InboundService.AddInbound(awg)
	if err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	awgID := awg.Id

	iface := fmt.Sprintf("awg%d", awgID)
	serverIP := fmt.Sprintf("10.%d.0.1", awgID%255)
	subnet := fmt.Sprintf("10.%d.0.0/24", awgID%255)
	rtTable := fmt.Sprintf("10%d", awgID)
	rtPref := 1000 + awgID

	// Create SOCKS5 child inbound (invisible — ParentID set)
	socksPort, err := m.createSOCKS5Child(awg, params)
	if err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("socks5 child: %w", err)
	}

	// Write configs with SOCKS5 port
	tmplData := TemplateData{
		AWGInterface:   iface,
		TUNInterface:   "tun0",
		AWGServerIP:    serverIP,
		AWGSubnet:      subnet,
		AWGPort:        awg.Port,
		SOCKS5Port:     socksPort,
		RouteTable:     rtTable,
		RouteTableName: fmt.Sprintf("awg%d", awgID),
		RoutePref:      rtPref,
		MTU:            params.MTU,
	}
	if err := m.writeConfigFiles(awg, params, tmplData); err != nil {
		m.rollbackCreate(awg.Id)
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Execute PostUp (creates awgN interface, tun0, tun2socks, nftables, routes)
	upPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID))
	cmd := exec.Command("/bin/bash", upPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("postup failed: %w\n%s", err, string(out))
	}

	// Auto-create first client
	if err := m.EnsureFirstClientExists(awg); err != nil {
		logAWG("Create: first client warning for inbound=%d: %v", awgID, err)
	}

	if needRestart {
		m.XrayService.SetToNeedRestart()
	}

	logAWG("Create: inbound=%d port=%d iface=%s socks5=%d ok", awgID, awg.Port, iface, socksPort)
	return awg, nil
}

// Delete tears down an AWG inbound and all its children.
func (m *AWGManager) Delete(awgID int) error {
	// Run PostDown
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	if data, err := os.ReadFile(downPath); err == nil {
		exec.Command("/bin/bash", "-c", string(data)).Run()
	}

	// Delete child inbounds
	children, _ := m.InboundService.GetByParentId(awgID)
	for _, child := range children {
		m.InboundService.DelInbound(child.Id)
	}

	m.InboundService.DelInbound(awgID)

	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID)))
	os.Remove(downPath)

	go func() {
		m.XrayService.SetToNeedRestart()
	}()

	logAWG("Delete: inbound=%d ok", awgID)
	return nil
}

// RepairAllAWGInbounds repairs every AWG inbound at panel startup.
func (m *AWGManager) RepairAllAWGInbounds() map[int]*RepairResult {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		logAWG("RepairAll: db error: %v", err)
		return nil
	}
	results := make(map[int]*RepairResult)
	repaired := 0
	for _, ib := range inbounds {
		r := m.RepairAWGInbound(ib.Id)
		results[ib.Id] = r
		if len(r.Fixed) > 0 {
			repaired++
		}
	}
	logAWG("RepairAll: %d inbounds checked, %d repaired", len(inbounds), repaired)
	return results
}

// RepairAWGInbound performs a full health check and repair.
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
	subnet := fmt.Sprintf("10.%d.0.0/24", awgID%255)
	rtTable := fmt.Sprintf("10%d", awgID)

	result.InterfaceOK = interfaceExists(iface)
	if !result.InterfaceOK {
		// Re-run PostUp to recreate
		upPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID))
		if _, err := os.Stat(upPath); err == nil {
			exec.Command("/bin/bash", upPath).Run()
			result.InterfaceOK = interfaceExists(iface)
			if result.InterfaceOK {
				result.Fixed = append(result.Fixed, "interface recreated")
			}
		}
	}

	result.RoutingOK = routeExists(subnet, rtTable)

	jc := getIntFromSettings(awg.Settings, "jc", 0)
	result.ObfuscationOK = jc > 0
	if !result.ObfuscationOK {
		if updated, _ := EnsureParams(awg); updated {
			m.InboundService.UpdateInbound(awg)
			result.ObfuscationOK = true
			result.Fixed = append(result.Fixed, "obfuscation repaired")
		}
	}

	clients, _ := m.InboundService.GetClients(awg)
	result.ClientKeysOK = len(clients) > 0

	return result
}

func (m *AWGManager) createSOCKS5Child(awg *model.Inbound, params *AWGParams) (int, error) {
	awgID := awg.Id
	socksPort := 20000 + (awgID % 1000)

	settings := jsonMarshal(map[string]interface{}{
		"auth": "noauth",
		"udp":  true,
	})

	socksInbound := &model.Inbound{
		UserId:   awg.UserId,
		NodeID:   awg.NodeID,
		ParentID: &awgID,
		Protocol: "socks",
		Tag:      fmt.Sprintf("awg-socks-%d", awgID),
		Port:     socksPort,
		Listen:   "127.0.0.1",
		Settings: settings,
		Enable:   true,
	}

	_, _, err := m.InboundService.AddInbound(socksInbound)
	if err != nil {
		return 0, err
	}
	logAWG("createSOCKS5Child: parent=%d port=%d", awgID, socksPort)
	return socksPort, nil
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

	logAWG("writeConfig: inbound=%d conf=%s up=%s down=%s socks5=%d", awgID, confPath, upPath, downPath, data.SOCKS5Port)
	return nil
}

func (m *AWGManager) rollbackCreate(awgID int) {
	m.InboundService.DelInbound(awgID)
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID)))
}
