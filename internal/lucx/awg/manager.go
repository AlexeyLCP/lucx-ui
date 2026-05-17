// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

const (
	awgConfigDir = "/etc/amnezia/amneziawg"
)

// AWGManager manages the full lifecycle of AWG inbounds.
// Replaces the older AWGService with a cleaner, more complete API.
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

// Create sets up a complete AWG inbound: saves to DB, creates TUN child,
// writes config, sets up routing, and auto-creates the first client.
func (m *AWGManager) Create(awg *model.Inbound) (*model.Inbound, error) {
	// 1. Check prerequisites
	pre := CheckPrerequisites()
	if !pre.OK() {
		return nil, fmt.Errorf("prerequisites not met: %v", pre.Errors)
	}

	// 2. Read user-chosen parameters
	obfLevel := getIntFromSettings(awg.Settings, "obfLevel", 2)
	mimicryProfile := getStringFromSettings(awg.Settings, "mimicryProfile", "quic")
	region := getStringFromSettings(awg.Settings, "region", "ru")

	// 3. Generate AWG parameters
	params, err := GenerateAWGParams(obfLevel, mimicryProfile, region)
	if err != nil {
		return nil, fmt.Errorf("generate params: %w", err)
	}

	// 4. Generate CPS (I1-I5)
	i1, i2, i3, i4, i5 := GenerateCPS(obfLevel, CPSProfile(params.MimicryProfile))

	// 5. Validate
	if err := ValidateAWGParams(params); err != nil {
		return nil, fmt.Errorf("validate params: %w", err)
	}

	// 6. Merge obfuscation into settings BEFORE AddInbound
	if err := MergeParamsToSettings(awg, params, i1, i2, i3, i4, i5); err != nil {
		return nil, fmt.Errorf("merge params: %w", err)
	}

	// 7. Save to DB
	awg, needRestart, err := m.InboundService.AddInbound(awg)
	if err != nil {
		return nil, fmt.Errorf("save awg inbound: %w", err)
	}

	awgID := awg.Id

	// 8. Auto-create first client (pumbaX-style: server + first client in one step)
	if err := m.EnsureFirstClient(awg); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("first client: %w", err)
	}

	// 9. Create paired TUN child inbound
	tunInbound, err := m.createTUNChild(awg, params)
	if err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("tun child: %w", err)
	}

	// 10. Generate and write PostUp/PostDown scripts
	tmplData := TemplateData{
		AWGInterface:   fmt.Sprintf("awg%d", awgID),
		TUNInterface:   fmt.Sprintf("awg%dt", awgID),
		AWGServerIP:    fmt.Sprintf("10.%d.0.1", awgID%255),
		AWGSubnet:      fmt.Sprintf("10.%d.0.0/24", awgID%255),
		AWGPort:        awg.Port,
		RouteTable:     fmt.Sprintf("10%d", awgID),
		RouteTableName: fmt.Sprintf("awg%d", awgID),
		RoutePref:      1000 + awgID,
		MTU:            params.MTU,
	}

	postUp, _ := RenderPostUp(tmplData)
	postDown, _ := RenderPostDown(tmplData)

	os.MkdirAll(awgConfigDir, 0755)
	upPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID))
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	os.WriteFile(upPath, []byte(postUp), 0755)
	os.WriteFile(downPath, []byte(postDown), 0755)

	// 11. Write AWG config
	awgConf := buildAWGConfig(awg, params, tmplData, upPath, downPath)
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	os.WriteFile(confPath, []byte(awgConf), 0600)

	// 12. Register route table
	appendToFile("/etc/iproute2/rt_tables", fmt.Sprintf("%d %s\n", 100+awgID, tmplData.RouteTableName))

	// 13. Setup routing (creates interface, iptables, routes)
	routingCfg := RoutingConfig{
		AWGInterface: tmplData.AWGInterface,
		TUNInterface: tmplData.TUNInterface,
		AWGServerIP:  tmplData.AWGServerIP,
		AWGSubnet:    tmplData.AWGSubnet,
		RouteTable:   tmplData.RouteTable,
		RoutePref:    tmplData.RoutePref,
		MTU:          tmplData.MTU,
	}
	if err := SetupTUNRouting(routingCfg); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("routing setup: %w", err)
	}

	// 14. Restart Xray
	if needRestart {
		m.XrayService.SetToNeedRestart()
	}

	logAWG("Create: inbound=%d port=%d iface=%s tun=%s", awgID, awg.Port, tmplData.AWGInterface, tmplData.TUNInterface)
	_ = tunInbound // created, tracked via parentId
	return awg, nil
}

// Delete tears down an AWG inbound: PostDown, children, routing, config.
func (m *AWGManager) Delete(awgID int) error {
	// 1. Run PostDown
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	if data, err := os.ReadFile(downPath); err == nil {
		exec.Command("/bin/bash", "-c", string(data)).Run()
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

	// 3. Delete child inbounds
	children, _ := m.InboundService.GetByParentId(awgID)
	for _, child := range children {
		m.InboundService.DelInbound(child.Id)
	}

	// 4. Delete from DB
	m.InboundService.DelInbound(awgID)

	// 5. Clean up files
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(upPath(awgID))
	os.Remove(downPath)

	// 6. Non-blocking Xray restart
	go func() {
		done := make(chan struct{})
		go func() {
			m.XrayService.SetToNeedRestart()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	}()

	logAWG("Delete: inbound=%d", awgID)
	return nil
}

// EnsureFirstClient creates a default client if none exists.
// Follows pumbaX convention: server + first client in one step.
func (m *AWGManager) EnsureFirstClient(awg *model.Inbound) error {
	clients, _ := m.InboundService.GetClients(awg)
	if len(clients) > 0 {
		return nil // already has clients
	}

	defaultClient := model.Client{
		ID:         GenKey(),
		Password:   GenPSK(),
		PrivateKey: GenKey(),
		Email:      fmt.Sprintf("default-%d", awg.Id),
		Enable:     true,
		ExpiryTime: 0,
	}

	clientSettings := fmt.Sprintf(
		`{"clients":[{"id":"%s","password":"%s","privateKey":"%s","email":"%s","enable":true,"expiryTime":0,"tgId":"","subId":"","comment":""}]}`,
		defaultClient.ID, defaultClient.Password, defaultClient.PrivateKey, defaultClient.Email,
	)

	clientInbound := &model.Inbound{Id: awg.Id, Settings: clientSettings}
	if _, err := m.InboundService.AddInboundClient(clientInbound); err != nil {
		return fmt.Errorf("add default client: %w", err)
	}

	logAWG("EnsureFirstClient: inbound=%d email=%s", awg.Id, defaultClient.Email)
	return nil
}

// AddClient adds a peer to an existing AWG interface.
func (m *AWGManager) AddClient(awgID int, client *model.Client) error {
	iface := fmt.Sprintf("awg%d", awgID)
	awg, err := m.InboundService.GetInbound(awgID)
	if err != nil {
		return fmt.Errorf("get awg inbound: %w", err)
	}

	var settings map[string]interface{}
	json.Unmarshal([]byte(awg.Settings), &settings)
	clients, _ := settings["clients"].([]interface{})
	nextOctet := 2 + len(clients)

	clientIP := fmt.Sprintf("10.%d.0.%d/32", awgID%255, nextOctet)

	// Add to kernel interface
	pubKey := client.ID
	psk := client.Password
	exec.Command("awg", "set", iface,
		"peer", pubKey,
		"preshared-key", psk,
		"allowed-ips", clientIP,
		"persistent-keepalive", "25",
	).Run()

	logAWG("AddClient: inbound=%d email=%s", awgID, client.Email)
	return nil
}

// DeleteClient removes a peer from an AWG interface.
func (m *AWGManager) DeleteClient(awgID int, publicKey string) error {
	iface := fmt.Sprintf("awg%d", awgID)
	cmd := exec.Command("awg", "set", iface, "peer", publicKey, "remove")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("awg remove peer: %w\n%s", err, string(out))
	}
	logAWG("DeleteClient: inbound=%d key=%s", awgID, publicKey[:16]+"...")
	return nil
}

// EnsureParams regenerates obfuscation params if missing.
func (m *AWGManager) EnsureParams(inbound *model.Inbound) (bool, error) {
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

// RepairAll iterates all AWG inbounds and fills missing obfuscation params.
func (m *AWGManager) RepairAll() (int, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		return 0, err
	}

	repaired := 0
	for _, ib := range inbounds {
		updated, err := m.EnsureParams(ib)
		if err != nil {
			continue
		}
		if updated {
			if err := db.Save(ib).Error; err == nil {
				repaired++
			}
		}
	}
	logAWG("RepairAll: repaired %d inbounds", repaired)
	return repaired, nil
}

// --- Helpers ---

func (m *AWGManager) createTUNChild(awg *model.Inbound, params *AWGParams) (*model.Inbound, error) {
	awgID := awg.Id
	tunName := fmt.Sprintf("awg%dt", awgID)
	tunSubnet := pickFreeTunSubnet(awgID)

	tunSettings, _ := json.Marshal(map[string]interface{}{
		"name":    tunName,
		"address": []string{tunSubnet},
		"stack":   "system",
		"mtu":     params.MTU,
	})

	tunInbound := &model.Inbound{
		UserId:   awg.UserId,
		NodeID:   awg.NodeID,
		ParentID: &awgID,
		Protocol: model.TUN,
		Tag:      fmt.Sprintf("awg-tun-%d", awgID),
		Port:     0,
		Settings: string(tunSettings),
		Enable:   true,
	}

	_, _, err := m.InboundService.AddInbound(tunInbound)
	if err != nil {
		return nil, err
	}
	return tunInbound, nil
}

func (m *AWGManager) rollbackCreate(awgID int) {
	m.InboundService.DelInbound(awgID)
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(upPath(awgID))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID)))
}

func upPath(awgID int) string {
	return filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID))
}

// --- Standalone helpers (used by controller for backward compat) ---

// RepairAWGOnGet checks and repairs a single AWG inbound when loaded.
// Standalone function for backward compatibility — no AWGManager needed.
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
			needsRepair = true
		}
	}
	if !needsRepair {
		return inbound, false
	}

	logAWG("RepairAWGOnGet: inbound %d needs repair (jc=%d)", inbound.Id, jc)
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
	return inbound, true
}
