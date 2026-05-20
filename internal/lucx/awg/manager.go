// Copyright (c) 2025 LucX-UI Project.

package awg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

const hiddenSOCKS5Port = 10808

// Create generates obfuscation ONCE, saves to DB, creates hidden child SOCKS5,
// writes .conf, runs awg-quick up, starts tun2socks.
func (m *AWGManager) Create(awg *model.Inbound) (*model.Inbound, error) {
	pre := CheckPrerequisites()
	if !pre.OK() {
		return nil, fmt.Errorf("prerequisites not met: %v", pre.Errors)
	}

	obfLevel := getIntFromSettings(awg.Settings, "obfLevel", 2)
	mimicryProfile := getStringFromSettings(awg.Settings, "mimicryProfile", "quic")
	region := getStringFromSettings(awg.Settings, "region", "ru")

	// 1. Generate server keys + obfuscation ONCE
	params, err := GenerateAWGParams(obfLevel, mimicryProfile, region)
	if err != nil {
		return nil, fmt.Errorf("generate params: %w", err)
	}
	i1, i2, i3, i4, i5 := GenerateCPS(obfLevel, CPSProfile(params.MimicryProfile))
	if err := ValidateAWGParams(params); err != nil {
		return nil, fmt.Errorf("validate params: %w", err)
	}

	// 2. Merge obfuscation + serverPublicKey into inbound.Settings (single source of truth)
	if err := MergeParamsToSettings(awg, params, i1, i2, i3, i4, i5); err != nil {
		return nil, fmt.Errorf("merge params: %w", err)
	}
	awg, needRestart, err := m.InboundService.AddInbound(awg)
	if err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	awgID := awg.Id

	// 3. Create hidden child SOCKS5 inbound (Rule 4)
	if err := m.createHiddenChild(awg); err != nil {
		m.rollbackCreate(awgID)
		return nil, fmt.Errorf("hidden child: %w", err)
	}

	// 4. Create first client (uses obfuscation from saved Settings)
	if err := m.EnsureFirstClientExists(awg); err != nil {
		logAWG("Create: first client warning for inbound=%d: %v", awgID, err)
	}

	// 5. Write .conf (with peers from step 4) and scripts
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

	// 7. Start tun2socks if not already running
	m.ensureTun2socks()

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

// RestoreAll rebuilds all AWG interfaces and ensures tun2socks is running.
// Called at panel startup (Rule 6).
func (m *AWGManager) RestoreAll() {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		logAWG("RestoreAll: db error: %v", err)
		return
	}
	restored := 0
	for _, ib := range inbounds {
		iface := fmt.Sprintf("awg%d", ib.Id)
		if !interfaceExists(iface) {
			confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", ib.Id))
			if _, err := os.Stat(confPath); err == nil {
				cmd := exec.Command("awg-quick", "up", confPath)
				if out, err := cmd.CombinedOutput(); err != nil {
					logAWG("RestoreAll: awg%d failed: %v\n%s", ib.Id, err, string(out))
					continue
				}
			}
		}
		// Ensure peers from DB are in kernel
		if err := m.syncPeers(&ib); err != nil {
			logAWG("RestoreAll: sync peers awg%d: %v", ib.Id, err)
		}
		restored++
	}
	m.ensureTun2socks()
	logAWG("RestoreAll: %d inbounds restored", restored)
}

func (m *AWGManager) RepairAllAWGInbounds() map[int]*RepairResult {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		return nil
	}
	results := make(map[int]*RepairResult)
	for _, ib := range inbounds {
		results[ib.Id] = m.RepairAWGInbound(ib.Id)
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

// createHiddenChild creates a SOCKS5 inbound with ParentID set (Rule 4).
func (m *AWGManager) createHiddenChild(awg *model.Inbound) error {
	awgID := awg.Id
	settings := `{"auth":"noauth","udp":true}`

	hidden := &model.Inbound{
		UserId:   awg.UserId,
		ParentID: &awgID,
		Protocol: "socks",
		Tag:      fmt.Sprintf("awg-hidden-%d", awgID),
		Port:     hiddenSOCKS5Port,
		Listen:   "127.0.0.1",
		Settings: settings,
		Enable:   true,
	}
	_, _, err := m.InboundService.AddInbound(hidden)
	if err != nil {
		return err
	}
	logAWG("createHiddenChild: awg=%d port=%d", awgID, hiddenSOCKS5Port)
	return nil
}

// syncPeers reads peers from DB and ensures they're in the AWG kernel.
func (m *AWGManager) syncPeers(awg *model.Inbound) error {
	clients, err := m.InboundService.GetClients(awg)
	if err != nil {
		return err
	}
	iface := fmt.Sprintf("awg%d", awg.Id)
	for _, c := range clients {
		if c.ID == "" || c.Password == "" || !c.Enable {
			continue
		}
		peerConf := fmt.Sprintf("[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = 0.0.0.0/0, ::/0\nPersistentKeepalive = 25\n",
			c.ID, c.Password)
		cmd := exec.Command("awg", "addconf", iface, "/dev/stdin")
		cmd.Stdin = strings.NewReader(peerConf) // assuming strings is imported
		cmd.Run()
	}
	return nil
}

// ensureTun2socks starts tun2socks if not already running.
func (m *AWGManager) ensureTun2socks() {
	if exec.Command("pgrep", "-f", "tun2socks.*10808").Run() == nil {
		return // already running
	}
	go func() {
		cmd := exec.Command("tun2socks", "-device", "tun0",
			"-proxy", fmt.Sprintf("socks5://127.0.0.1:%d", hiddenSOCKS5Port),
			"-loglevel", "silent")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			logAWG("tun2socks start failed: %v", err)
			return
		}
		logAWG("tun2socks started pid=%d", cmd.Process.Pid)
	}()
}

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

// RestoreAllInterfaces is a package-level function that restores all AWG
// interfaces and starts tun2socks. Safe to call without an AWGManager.
func RestoreAllInterfaces() {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Where("protocol = ?", "awg").Find(&inbounds).Error; err != nil {
		logAWG("RestoreAllInterfaces: db error: %v", err)
		return
	}
	for _, ib := range inbounds {
		confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", ib.Id))
		if _, err := os.Stat(confPath); os.IsNotExist(err) {
			logAWG("RestoreAllInterfaces: conf not found for awg%d, skipping", ib.Id)
			continue
		}
		iface := fmt.Sprintf("awg%d", ib.Id)
		if !interfaceExists(iface) {
			cmd := exec.Command("awg-quick", "up", confPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				logAWG("RestoreAllInterfaces: awg%d failed: %v\n%s", ib.Id, err, string(out))
				continue
			}
		}
		logAWG("RestoreAllInterfaces: awg%d restored", ib.Id)
	}
	// Start tun2socks if not running
	if exec.Command("pgrep", "-f", "tun2socks.*10808").Run() != nil {
		cmd := exec.Command("tun2socks", "-device", "tun0",
			"-proxy", "socks5://127.0.0.1:10808",
			"-loglevel", "silent")
		cmd.Start()
		logAWG("RestoreAllInterfaces: tun2socks started")
	}
}
