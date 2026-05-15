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
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

const (
	awgConfigDir    = "/etc/amnezia/amneziawg"
	clientConfigDir = "/root"
)

// AWGService manages the full lifecycle of AWG inbounds.
type AWGService struct {
	InboundService *service.InboundService
	XrayService    *service.XrayService
}

// NewAWGService creates a new AWGService.
func NewAWGService(inboundSvc *service.InboundService, xraySvc *service.XrayService) *AWGService {
	return &AWGService{
		InboundService: inboundSvc,
		XrayService:    xraySvc,
	}
}

// CreateAWGInbound creates an AWG inbound with auto-paired TUN child.
func (s *AWGService) CreateAWGInbound(awg *model.Inbound) (*model.Inbound, error) {
	// 1. Check prerequisites
	pre := CheckPrerequisites()
	if !pre.OK() {
		return nil, fmt.Errorf("prerequisites not met: %v", pre.Errors)
	}

	// 2. Generate AWG parameters
	params, err := GenerateAWGParams(1, "quic", "ru")
	if err != nil {
		return nil, fmt.Errorf("generate params: %w", err)
	}

	// 3. Save AWG inbound (AddInbound returns (inbound, needRestart, error))
	awg, needRestart, err := s.InboundService.AddInbound(awg)
	if err != nil {
		return nil, fmt.Errorf("save awg inbound: %w", err)
	}

	// 4. Allocate resources
	awgId := awg.Id
	serverIP := fmt.Sprintf("10.%d.0.1", awgId%255)
	subnet := fmt.Sprintf("10.%d.0.0/24", awgId%255)
	tunSubnet := pickFreeTunSubnet(awgId)
	tunName := fmt.Sprintf("awg%dt", awgId)
	iface := fmt.Sprintf("awg%d", awgId)

	// 5. Create paired TUN child inbound
	tunSettings, _ := json.Marshal(map[string]interface{}{
		"name":    tunName,
		"address": []string{tunSubnet},
		"stack":   "system",
		"mtu":     params.MTU,
	})

	tunInbound := &model.Inbound{
		UserId:   awg.UserId,
		NodeID:   awg.NodeID,
		ParentID: &awgId,
		Protocol: model.TUN,
		Tag:      fmt.Sprintf("awg-tun-%d", awgId),
		Port:     0,
		Settings: string(tunSettings),
		Enable:   true,
	}
	_, _, err = s.InboundService.AddInbound(tunInbound)
	if err != nil {
		s.InboundService.DelInbound(awgId)
		return nil, fmt.Errorf("create tun child: %w", err)
	}

	// 6. Generate PostUp/PostDown scripts
	tmplData := TemplateData{
		AWGInterface:   iface,
		TUNInterface:   tunName,
		AWGServerIP:    serverIP,
		AWGSubnet:      subnet,
		AWGPort:        awg.Port,
		RouteTable:     fmt.Sprintf("10%d", awgId),
		RouteTableName: fmt.Sprintf("awg%d", awgId),
		RoutePref:      1000 + awgId,
		MTU:            params.MTU,
	}

	postUp, _ := RenderPostUp(tmplData)
	postDown, _ := RenderPostDown(tmplData)

	os.MkdirAll(awgConfigDir, 0755)

	upPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgId))
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgId))
	os.WriteFile(upPath, []byte(postUp), 0755)
	os.WriteFile(downPath, []byte(postDown), 0755)

	// 7. Write AWG config
	awgConf := buildAWGConfig(awg, params, tmplData, upPath, downPath)
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgId))
	os.WriteFile(confPath, []byte(awgConf), 0600)

	// 8. Register route table
	rtLine := fmt.Sprintf("%d %s\n", 100+awgId, tmplData.RouteTableName)
	appendToFile("/etc/iproute2/rt_tables", rtLine)

	// 9. Execute PostUp
	cmd := exec.Command("/bin/bash", upPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		s.rollbackCreate(awgId)
		return nil, fmt.Errorf("postup failed: %w\n%s", err, string(out))
	}

	// 10. Restart Xray
	if needRestart {
		s.XrayService.RestartXray(false)
	}

	return awg, nil
}

// DeleteAWGInbound tears down an AWG inbound and all its children.
func (s *AWGService) DeleteAWGInbound(awgId int) error {
	// 1. Run PostDown
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgId))
	if data, err := os.ReadFile(downPath); err == nil {
		exec.Command("/bin/bash", "-c", string(data)).Run()
	}

	// 2. Delete child TUN inbounds
	children, _ := s.InboundService.GetByParentId(awgId)
	for _, child := range children {
		s.InboundService.DelInbound(child.Id)
	}

	// 3. Delete AWG inbound
	s.InboundService.DelInbound(awgId)

	// 4. Clean up files
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgId)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgId)))
	os.Remove(filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgId)))

	// 5. Restart Xray
	s.XrayService.RestartXray(false)
	return nil
}

// AddClient adds a peer to an existing AWG interface.
func (s *AWGService) AddClient(awgId int, client *model.Client) error {
	iface := fmt.Sprintf("awg%d", awgId)

	// Count existing clients to determine next IP
	awg, err := s.InboundService.GetInbound(awgId)
	if err != nil {
		return fmt.Errorf("get awg inbound: %w", err)
	}

	var settings map[string]interface{}
	json.Unmarshal([]byte(awg.Settings), &settings)
	clients, _ := settings["clients"].([]interface{})
	nextOctet := 2 + len(clients)

	clientIP := fmt.Sprintf("10.%d.0.%d/32", awgId%255, nextOctet)

	// Add peer to running interface
	cmd := exec.Command("awg", "set", iface,
		"peer", client.ID,           // ID = client public key
		"preshared-key", client.Password, // Password = PSK
		"allowed-ips", clientIP,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("awg set peer: %w\n%s", err, string(out))
	}

	// Append to config file
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgId))
	peerBlock := fmt.Sprintf("\n[Peer]\n# %s\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = %s\n",
		client.Email, client.ID, client.Password, clientIP)
	appendToFile(confPath, peerBlock)

	return nil
}

// DeleteClient removes a peer from an AWG interface by public key.
func (s *AWGService) DeleteClient(awgId int, publicKey string) error {
	iface := fmt.Sprintf("awg%d", awgId)

	// Remove from running interface (best-effort, may already be gone)
	exec.Command("awg", "set", iface, "peer", publicKey, "remove").Run()

	// Remove [Peer] block from config file
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgId))
	data, err := os.ReadFile(confPath)
	if err != nil {
		return nil // config already gone, nothing to clean
	}

	lines := strings.Split(string(data), "\n")
	var filtered []string
	skip := false
	for _, line := range lines {
		if strings.HasPrefix(line, "[Peer]") {
			skip = false
		}
		if skip {
			if strings.TrimSpace(line) == "" {
				skip = false
			}
			continue
		}
		if strings.HasPrefix(line, "PublicKey = "+publicKey) {
			// Remove the previous comment line and this [Peer] block
			if len(filtered) > 0 && strings.HasPrefix(filtered[len(filtered)-1], "#") {
				filtered = filtered[:len(filtered)-1]
			}
			if len(filtered) > 0 && filtered[len(filtered)-1] == "[Peer]" {
				filtered = filtered[:len(filtered)-1]
			}
			skip = true
			continue
		}
		filtered = append(filtered, line)
	}
	os.WriteFile(confPath, []byte(strings.Join(filtered, "\n")), 0600)
	return nil
}

// RestoreAll re-creates AWG interfaces from saved configs after panel restart.
func (s *AWGService) RestoreAll() error {
	entries, err := os.ReadDir(awgConfigDir)
	if err != nil {
		return nil // no configs to restore
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), "-up.sh") {
			continue
		}
		scriptPath := filepath.Join(awgConfigDir, entry.Name())
		cmd := exec.Command("/bin/bash", scriptPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("restore %s: %w\n%s", entry.Name(), err, string(out))
		}
	}
	return nil
}

func (s *AWGService) rollbackCreate(awgId int) {
	s.DeleteAWGInbound(awgId)
}

func buildAWGConfig(awg *model.Inbound, params *AWGParams, data TemplateData, upPath, downPath string) string {
	return fmt.Sprintf(`[Interface]
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
PostUp = %s
PostDown = %s
`, params.PrivateKey, data.AWGServerIP, awg.Port, params.MTU,
		params.Jc, params.Jmin, params.Jmax,
		params.S1, params.S2, params.S3, params.S4,
		params.H1, params.H2, params.H3, params.H4,
		upPath, downPath)
}

func pickFreeTunSubnet(awgId int) string {
	return fmt.Sprintf("172.19.%d.%d/30", awgId/64, (awgId%64)*4)
}

func appendToFile(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}
