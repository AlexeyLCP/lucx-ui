// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package telemt

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	telemtBinaryPath = "/usr/local/bin/telemt"
	telemtConfigDir  = "/etc/telemt"
	telemtPIDDir     = "/var/run/telemt"
	telemtDataDir    = "/var/lib/telemt"
	releaseAPIURL    = "https://api.github.com/repos/telemt/telemt/releases/latest"
	downloadURLTmpl  = "https://github.com/telemt/telemt/releases/download/%s/telemt-x86_64-linux-gnu.tar.gz"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

type TelemtManager struct {
	mu        sync.Mutex
	instances map[int]*Instance
}

type Instance struct {
	ID         int
	ConfigPath string
	PID        int
	SocksPort  int
	APIPort    int
	Status     string
}

func NewTelemtManager() *TelemtManager {
	return &TelemtManager{instances: make(map[int]*Instance)}
}

func (m *TelemtManager) EnsureBinary() (string, error) {
	if _, err := os.Stat(telemtBinaryPath); err == nil {
		out, err := exec.Command(telemtBinaryPath, "--version").Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	resp, err := http.Get(releaseAPIURL)
	if err != nil {
		return "", fmt.Errorf("fetch latest release: %w (manual install: curl -fsSL https://raw.githubusercontent.com/telemt/telemt/main/install.sh | sh)", err)
	}
	defer resp.Body.Close()
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decode release: %w", err)
	}
	downloadURL := fmt.Sprintf(downloadURLTmpl, release.TagName)
	tarball, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("download tarball: %w", err)
	}
	defer tarball.Body.Close()
	if err := extractTelemtBinary(tarball.Body); err != nil {
		return "", fmt.Errorf("extract binary: %w", err)
	}
	if err := os.Chmod(telemtBinaryPath, 0755); err != nil {
		return "", fmt.Errorf("chmod: %w", err)
	}
	return release.TagName, nil
}

func extractTelemtBinary(r io.Reader) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == "telemt" || strings.HasSuffix(hdr.Name, "/telemt") {
			f, err := os.Create(telemtBinaryPath)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, tr)
			return err
		}
	}
	return fmt.Errorf("telemt binary not found in tarball")
}

func (m *TelemtManager) Start(id int, configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	dataDir := filepath.Join(telemtDataDir, fmt.Sprintf("telemt-%d", id))
	pidPath := filepath.Join(telemtPIDDir, fmt.Sprintf("telemt-%d.pid", id))
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(telemtPIDDir, 0755)
	cmd := exec.Command(telemtBinaryPath, "start", configPath,
		"--pid-file", pidPath, "--working-dir", dataDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start telemt: %w", err)
	}
	pidData, _ := os.ReadFile(pidPath)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(pidData)))
	m.instances[id] = &Instance{ID: id, ConfigPath: configPath, PID: pid, Status: "running"}
	return nil
}

func (m *TelemtManager) Stop(id int) error {
	m.mu.Lock()
	inst, ok := m.instances[id]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	pidPath := filepath.Join(telemtPIDDir, fmt.Sprintf("telemt-%d.pid", id))
	// Graceful stop via telemt CLI — Run() blocks until process exits
	stopCmd := exec.Command(telemtBinaryPath, "stop", "--pid-file", pidPath)
	if err := stopCmd.Run(); err != nil {
		// Graceful stop failed — force kill via process handle
		if proc, ferr := os.FindProcess(inst.PID); ferr == nil {
			proc.Kill()
			proc.Wait()
		}
	}
	m.mu.Lock()
	delete(m.instances, id)
	m.mu.Unlock()
	return nil
}

func (m *TelemtManager) Healthcheck(apiPort int) error {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", apiPort))
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("healthcheck returned %d", resp.StatusCode)
	}
	return nil
}

func (m *TelemtManager) RestoreAll() error {
	entries, err := os.ReadDir(telemtConfigDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "telemt-") || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		var id int
		fmt.Sscanf(entry.Name(), "telemt-%d.toml", &id)
		configPath := filepath.Join(telemtConfigDir, entry.Name())
		if err := m.Start(id, configPath); err != nil {
			return fmt.Errorf("restore %s: %w", entry.Name(), err)
		}
	}
	return nil
}
