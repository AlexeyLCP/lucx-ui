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

type AWGManager struct {
	InboundService *service.InboundService
	XrayService    *service.XrayService
}

func NewAWGManager(inboundSvc *service.InboundService, xraySvc *service.XrayService) *AWGManager {
	return &AWGManager{InboundService: inboundSvc, XrayService: xraySvc}
}

// Create generates obfuscation ONCE, saves to DB, writes .conf, runs awg-quick up, starts tun2socks.
func (m *AWGManager) Create(awg *model.Inbound) (*model.Inbound, error) {
	pre := CheckPrerequisites()
	if !pre.OK() {
		return nil, fmt.Errorf("prerequisites not met: %v", pre.Errors)
	}

	// 1. Read user params
	obfLevel := getIntFromSettings(awg.Settings, "obfLevel", 2)
	mimicryProfile := getStringFromSettings(awg.Settings, "mimicryProfile", "quic")
	region := getStringFromSettings(awg.Settings, "region", "ru")

	// 2. Generate obfuscation ONCE
	params, err := GenerateAWGParams(obfLevel, mimicryProfile, region)
	if err != nil {
		return nil, fmt.Errorf("generate params: %w", err)
	}
	i1, i2, i3, i4, i5 := GenerateCPS(obfLevel, CPSProfile(params.MimicryProfile))
	if err := ValidateAWGParams(params); err != nil {
		return nil, fmt.Errorf("validate params: %w", err)
	}

	// 3. Merge into inbound.Settings and save to DB
	if err := MergeParamsToSettings(awg, params, i1, i2, i3, i4, i5); err != nil {
		return nil, fmt.Errorf("merge params: %w", err)
	}
	awg, needRestart, err := m.InboundService.AddInbound(awg)
	if err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	awgID := awg.Id

	// 4. Create first client (uses obfuscation from saved Settings)
	if err := m.EnsureFirstClientExists(awg); err != nil {
		logAWG("Create: first client warning for inbound=%d: %v", awgID, err)
	}

	// 5. Write .conf (now has peers from step 4) and scripts
	iface := fmt.Sprintf("awg%d", awgID)
	serverIP := fmt.Sprintf("10.%d.0.1", awgID%255)
	subnet := fmt.Sprintf("10.%d.0.0/24", awgID%255)

	if err := m.writeConfigFiles(awg, iface, serverIP, subnet); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("write config: %w", err)
	}

	logAWG("Create: Jc=%d Jmin=%d Jmax=%d S1=%d S2=%d H1=%s",
		params.Jc, params.Jmin, params.Jmax, params.S1, params.S2, params.H1)

	// 6. Run awg-quick up
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	cmd := exec.Command("awg-quick", "up", confPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		logAWG("Create: awg-quick up warning: %v\n%s", err, string(out))
	}

	// 7. Start tun2socks
	go func() {
		cmd := exec.Command("tun2socks", "-device", "tun0",
			"-proxy", "socks5://127.0.0.1:10808",
			"-loglevel", "silent")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			logAWG("tun2socks start failed: %v", err)
			return
		}
		logAWG("tun2socks started pid=%d", cmd.Process.Pid)
	}()

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
		return nil
	}
	results := make(map[int]*RepairResult)
	for _, ib := range inbounds {
		r := m.RepairAWGInbound(ib.Id)
		results[ib.Id] = r
	}
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
	jc := intVal(parseSettings(awg.Settings), "jc", 0)
	result.ObfuscationOK = jc > 0
	clients, _ := m.InboundService.GetClients(awg)
	result.ClientKeysOK = len(clients) > 0
	return result
}

// writeConfigFiles writes up.sh, down.sh, and the .conf with all peers.
func (m *AWGManager) writeConfigFiles(awg *model.Inbound, iface, serverIP, subnet string) error {
	awgID := awg.Id
	if err := os.MkdirAll(awgConfigDir, 0755); err != nil {
		return err
	}
	data := TemplateData{
		AWGInterface:   iface,
		AWGServerIP:    serverIP,
		AWGSubnet:      subnet,
		AWGPort:        awg.Port,
		RouteTable:     fmt.Sprintf("10%d", awgID),
		RouteTableName: fmt.Sprintf("awg%d", awgID),
		RoutePref:      1000 + awgID,
		MTU:            intVal(parseSettings(awg.Settings), "mtu", 1320),
	}
	up, _ := RenderPostUp(data)
	down, _ := RenderPostDown(data)
	upPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID))
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	os.WriteFile(upPath, []byte(up), 0755)
	os.WriteFile(downPath, []byte(down), 0755)

	conf := BuildServerConfig(awg, upPath, downPath)
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	if err := os.WriteFile(confPath, []byte(conf), 0600); err != nil {
		return err
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
