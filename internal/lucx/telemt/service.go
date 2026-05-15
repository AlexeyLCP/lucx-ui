// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telemt

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

// TelemtService manages the full lifecycle of Telemt inbounds.
type TelemtService struct {
	Manager        *TelemtManager
	InboundService *service.InboundService
	XrayService    *service.XrayService
}

// NewTelemtService creates a new TelemtService.
func NewTelemtService(ibSvc *service.InboundService, xraySvc *service.XrayService) *TelemtService {
	return &TelemtService{
		Manager:        NewTelemtManager(),
		InboundService: ibSvc,
		XrayService:    xraySvc,
	}
}

// CreateTelemtInbound creates a Telemt inbound with auto-paired SOCKS5 child.
func (s *TelemtService) CreateTelemtInbound(telemt *model.Inbound) (*model.Inbound, error) {
	// 1. Ensure binary available
	if _, err := s.Manager.EnsureBinary(); err != nil {
		return nil, fmt.Errorf("telemt binary: %w", err)
	}

	// 2. Allocate free SOCKS5 port on localhost
	socksPort, err := pickFreePort()
	if err != nil {
		return nil, fmt.Errorf("allocate socks port: %w", err)
	}
	socksPassword := randString(16)

	// 3. Save Telemt inbound to DB
	telemt, needRestart, err := s.InboundService.AddInbound(telemt)
	if err != nil {
		return nil, fmt.Errorf("save telemt inbound: %w", err)
	}

	// 4. Create paired SOCKS5 Xray inbound (child)
	socksSettings := fmt.Sprintf(`{"auth":"password","accounts":[{"user":"telemt","pass":"%s"}],"udp":false}`, socksPassword)
	socksInbound := &model.Inbound{
		UserId:   telemt.UserId,
		NodeID:   telemt.NodeID,
		ParentID: &telemt.Id,
		Protocol: "socks",
		Tag:      fmt.Sprintf("telemt-in-%d", telemt.Id),
		Listen:   "127.0.0.1",
		Port:     socksPort,
		Settings: socksSettings,
		Enable:   true,
	}
	if _, _, err := s.InboundService.AddInbound(socksInbound); err != nil {
		s.InboundService.DelInbound(telemt.Id)
		return nil, fmt.Errorf("create socks child: %w", err)
	}

	// 5. Generate TOML config and write to disk
	configData := ConfigData{
		ID:             telemt.Id,
		Port:           telemt.Port,
		PublicHost:     getPublicIP(),
		SocksPort:      socksPort,
		SocksPassword:  socksPassword,
		APIPort:        9090 + telemt.Id,
		TLSDomain:      "gosuslugi.ru",
		MaxConnections: 10000,
	}
	tomlContent, _ := GenerateConfig(configData)

	os.MkdirAll(telemtConfigDir, 0755)
	configPath := filepath.Join(telemtConfigDir, fmt.Sprintf("telemt-%d.toml", telemt.Id))
	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		s.rollbackCreate(telemt.Id)
		return nil, fmt.Errorf("write config: %w", err)
	}

	// 6. Start Telemt process
	if err := s.Manager.Start(telemt.Id, configPath); err != nil {
		s.rollbackCreate(telemt.Id)
		return nil, fmt.Errorf("start telemt: %w", err)
	}

	// 7. Restart Xray to pick up new SOCKS5 inbound
	if needRestart {
		s.XrayService.RestartXray(false)
	}

	return telemt, nil
}

// DeleteTelemtInbound tears down a Telemt inbound and its SOCKS5 child.
func (s *TelemtService) DeleteTelemtInbound(id int) error {
	// 1. Stop Telemt process
	s.Manager.Stop(id)

	// 2. Delete child SOCKS5 inbounds
	children, _ := s.InboundService.GetByParentId(id)
	for _, child := range children {
		s.InboundService.DelInbound(child.Id)
	}

	// 3. Delete Telemt inbound
	s.InboundService.DelInbound(id)

	// 4. Clean up files
	os.Remove(filepath.Join(telemtConfigDir, fmt.Sprintf("telemt-%d.toml", id)))
	os.Remove(filepath.Join(telemtPIDDir, fmt.Sprintf("telemt-%d.pid", id)))
	os.RemoveAll(filepath.Join(telemtDataDir, fmt.Sprintf("telemt-%d", id)))

	// 5. Restart Xray
	s.XrayService.RestartXray(false)
	return nil
}

func (s *TelemtService) rollbackCreate(id int) {
	s.DeleteTelemtInbound(id)
}

func pickFreePort() (int, error) {
	for port := 20000; port <= 50000; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free ports in 20000-50000")
}

func randString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[idx.Int64()]
	}
	return string(result)
}

func getPublicIP() string {
	return "" // caller should fill from request context or settings
}

// AddClient adds a user to a running Telemt instance's TOML config.
func (s *TelemtService) AddClient(telemtID int, client TelemtClient) error {
	configPath := filepath.Join(telemtConfigDir, fmt.Sprintf("telemt-%d.toml", telemtID))
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// Stop the instance before modifying config
	s.Manager.Stop(telemtID)

	// Append client to config
	line := fmt.Sprintf("\n%s = \"%s\"\n", client.Name, client.Secret)
	if err := os.WriteFile(configPath, append(data, []byte(line)...), 0644); err != nil {
		// Try to restart anyway
		_ = s.Manager.Start(telemtID, configPath)
		return fmt.Errorf("write config: %w", err)
	}

	// Restart
	if err := s.Manager.Start(telemtID, configPath); err != nil {
		return fmt.Errorf("restart telemt: %w", err)
	}

	return nil
}

// DeleteClient removes a user from a running Telemt instance's TOML config.
func (s *TelemtService) DeleteClient(telemtID int, name string) error {
	configPath := filepath.Join(telemtConfigDir, fmt.Sprintf("telemt-%d.toml", telemtID))
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// Stop the instance before modifying config
	s.Manager.Stop(telemtID)

	// Remove the client line
	lines := string(data)
	target := fmt.Sprintf("%s = ", name)
	var newLines []string
	for _, line := range splitLines(lines) {
		if !startsWith(line, target) {
			newLines = append(newLines, line)
		}
	}
	newContent := joinLines(newLines)

	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		_ = s.Manager.Start(telemtID, configPath)
		return fmt.Errorf("write config: %w", err)
	}

	// Restart
	if err := s.Manager.Start(telemtID, configPath); err != nil {
		return fmt.Errorf("restart telemt: %w", err)
	}

	return nil
}

// splitLines splits text into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinLines joins lines into a single string.
func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		result += line
	}
	return result
}

// startsWith checks if s starts with prefix.
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
