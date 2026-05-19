// Copyright (c) 2025 LucX-UI Project.

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

const defaultSOCKS5Port = 20000

type AWGManager struct {
	InboundService *service.InboundService
	XrayService    *service.XrayService
}

func NewAWGManager(inboundSvc *service.InboundService, xraySvc *service.XrayService) *AWGManager {
	return &AWGManager{InboundService: inboundSvc, XrayService: xraySvc}
}

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

	// Ensure shared SOCKS5 proxy
	if err := m.ensureDefaultSOCKS5(awg, params); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("socks5 proxy: %w", err)
	}

	// Write config files (with peers)
	tmplData := TemplateData{
		AWGInterface:   iface,
		TUNInterface:   "tun0",
		AWGServerIP:    serverIP,
		AWGSubnet:      subnet,
		AWGPort:        awg.Port,
		SOCKS5Port:     defaultSOCKS5Port,
		RouteTable:     rtTable,
		RouteTableName: fmt.Sprintf("awg%d", awgID),
		RoutePref:      rtPref,
		MTU:            params.MTU,
	}
	if err := m.writeConfigFiles(awg, params, tmplData); err != nil {
		m.rollbackCreate(awg.Id)
		return nil, fmt.Errorf("write config: %w", err)
	}

	logAWG("BuildServerConfig: Jc=%d Jmin=%d Jmax=%d S1=%d S2=%d S3=%d S4=%d H1=%s",
		params.Jc, params.Jmin, params.Jmax, params.S1, params.S2, params.S3, params.S4, params.H1)

	// Register route table before awg-quick up
	appendToFile("/etc/iproute2/rt_tables", fmt.Sprintf("%d %s\n", 100+awgID, tmplData.RouteTableName))

	// Use awg-quick up — correctly handles setconf + interface + PostUp
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	cmd := exec.Command("awg-quick", "up", confPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		// awg-quick might have already created the interface — check
		if !interfaceExists(iface) {
			m.rollbackCreate(awgID)
			return nil, fmt.Errorf("awg-quick up failed: %w\n%s", err, string(out))
		}
		logAWG("Create: awg-quick warning (interface exists): %v", err)
	}

	// Start tun2socks if not running (don't kill existing — avoid SSH disruption)
	if !tun2socksRunning() {
		go exec.Command("tun2socks", "-device", "tun0",
			"-proxy", fmt.Sprintf("socks5://127.0.0.1:%d", defaultSOCKS5Port),
			"-loglevel", "silent").Start()
	}

	// Auto-create first client
	if err := m.EnsureFirstClientExists(awg); err != nil {
		logAWG("Create: first client warning for inbound=%d: %v", awgID, err)
	}

	if needRestart {
		m.XrayService.SetToNeedRestart()
	}

	logAWG("Create: inbound=%d port=%d iface=%s ok", awgID, awg.Port, iface)
	return awg, nil
}

func (m *AWGManager) Delete(awgID int) error {
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	if data, err := os.ReadFile(downPath); err == nil {
		exec.Command("/bin/bash", "-c", string(data)).Run()
	}
	children, _ := m.InboundService.GetByParentId(awgID)
	for _, child := range children {
		m.InboundService.DelInbound(child.Id)
	}
	m.InboundService.DelInbound(awgID)
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID)))
	os.Remove(downPath)
	go func() { m.XrayService.SetToNeedRestart() }()
	logAWG("Delete: inbound=%d ok", awgID)
	return nil
}

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
	result.InterfaceOK = interfaceExists(iface)
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

func (m *AWGManager) ensureDefaultSOCKS5(awg *model.Inbound, params *AWGParams) error {
	db := database.GetDB()
	var count int64
	db.Model(&model.Inbound{}).Where("port = ? AND protocol = ? AND listen = ?",
		defaultSOCKS5Port, "socks", "127.0.0.1").Count(&count)
	if count > 0 {
		return nil
	}

	settings := jsonMarshal(map[string]interface{}{"auth": "noauth", "udp": true})
	socksInbound := &model.Inbound{
		UserId:   awg.UserId,
		Protocol: "socks",
		Tag:      "awg-socks-default",
		Port:     defaultSOCKS5Port,
		Listen:   "127.0.0.1",
		Settings: settings,
		Enable:   true,
	}
	_, _, err := m.InboundService.AddInbound(socksInbound)
	if err != nil {
		return nil // might already exist
	}
	logAWG("ensureDefaultSOCKS5: port=%d", defaultSOCKS5Port)
	return nil
}

func tun2socksRunning() bool {
	return exec.Command("pgrep", "-f", "tun2socks -device tun0").Run() == nil
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
	os.WriteFile(upPath, []byte(postUp), 0755)
	os.WriteFile(downPath, []byte(postDown), 0755)

	conf := BuildServerConfig(awg, params, data, upPath, downPath)
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	if err := os.WriteFile(confPath, []byte(conf), 0600); err != nil {
		return fmt.Errorf("write %s: %w", confPath, err)
	}

	logAWG("writeConfig: inbound=%d conf=%s", awgID, confPath)
	return nil
}

func (m *AWGManager) rollbackCreate(awgID int) {
	m.InboundService.DelInbound(awgID)
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID)))
}
