// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ImportedAWGConfig holds the parsed contents of an AWG server/client config.
type ImportedAWGConfig struct {
	Interface  ImportedInterface   `json:"interface"`
	Peers      []ImportedPeer      `json:"peers"`
	PostUp     string              `json:"postUp"`
	PostDown   string              `json:"postDown"`
	FilePath   string              `json:"filePath"`
}

// ImportedInterface holds the [Interface] section of an AWG config.
type ImportedInterface struct {
	PrivateKey string `json:"privateKey"`
	Address    string `json:"address"`
	ListenPort int    `json:"listenPort"`
	MTU        int    `json:"mtu"`
	Jc         int    `json:"jc"`
	Jmin       int    `json:"jmin"`
	Jmax       int    `json:"jmax"`
	S1         int    `json:"s1"`
	S2         int    `json:"s2"`
	S3         int    `json:"s3"`
	S4         int    `json:"s4"`
	H1         string `json:"h1"`
	H2         string `json:"h2"`
	H3         string `json:"h3"`
	H4         string `json:"h4"`
	I1         string `json:"i1"`
	I2         string `json:"i2"`
	I3         string `json:"i3"`
	I4         string `json:"i4"`
	I5         string `json:"i5"`
	DNS        string `json:"dns"`
}

// ImportedPeer holds a [Peer] section of an AWG config.
type ImportedPeer struct {
	Comment           string `json:"comment"`
	PublicKey         string `json:"publicKey"`
	PresharedKey      string `json:"presharedKey"`
	AllowedIPs        string `json:"allowedIPs"`
	Endpoint          string `json:"endpoint"`
	PersistentKeepalive int  `json:"persistentKeepalive"`
}

// ScanAWGConfigs scans /etc/amnezia/amneziawg/ for .conf files and returns their paths.
func ScanAWGConfigs() ([]string, error) {
	var paths []string
	entries, err := os.ReadDir(awgConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config dir %s: %w", awgConfigDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".conf") &&
			!strings.HasSuffix(entry.Name(), "-up.sh") && !strings.HasSuffix(entry.Name(), "-down.sh") {
			paths = append(paths, filepath.Join(awgConfigDir, entry.Name()))
		}
	}
	return paths, nil
}

// ParseAWGConfig parses a single AWG .conf file and returns its structured contents.
func ParseAWGConfig(path string) (*ImportedAWGConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	cfg := &ImportedAWGConfig{FilePath: path}
	var currentSection string
	var currentPeer *ImportedPeer
	scanner := bufio.NewScanner(f)
	var pendingPeerComment string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#!") {
			continue
		}

		// Comment line before [Peer] - treat as peer name
		if strings.HasPrefix(line, "#") && currentSection == "peer" && currentPeer != nil {
			continue // skip inline comments
		}

		if strings.HasPrefix(line, "[Interface]") {
			currentSection = "interface"
			continue
		}
		if strings.HasPrefix(line, "[Peer]") {
			currentSection = "peer"
			currentPeer = &ImportedPeer{Comment: pendingPeerComment}
			cfg.Peers = append(cfg.Peers, *currentPeer)
			currentPeer = &cfg.Peers[len(cfg.Peers)-1]
			pendingPeerComment = ""
			continue
		}

		// Comment line before [Peer] captures the peer name
		if strings.HasPrefix(line, "#") && currentSection != "peer" {
			comment := strings.TrimPrefix(line, "#")
			comment = strings.TrimSpace(comment)
			pendingPeerComment = comment
			continue
		}

		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])

		switch currentSection {
		case "interface":
			switch strings.ToLower(key) {
			case "privatekey":
				cfg.Interface.PrivateKey = val
			case "address":
				cfg.Interface.Address = val
			case "listenport":
				cfg.Interface.ListenPort, _ = strconv.Atoi(val)
			case "mtu":
				cfg.Interface.MTU, _ = strconv.Atoi(val)
			case "jc":
				cfg.Interface.Jc, _ = strconv.Atoi(val)
			case "jmin":
				cfg.Interface.Jmin, _ = strconv.Atoi(val)
			case "jmax":
				cfg.Interface.Jmax, _ = strconv.Atoi(val)
			case "s1":
				cfg.Interface.S1, _ = strconv.Atoi(val)
			case "s2":
				cfg.Interface.S2, _ = strconv.Atoi(val)
			case "s3":
				cfg.Interface.S3, _ = strconv.Atoi(val)
			case "s4":
				cfg.Interface.S4, _ = strconv.Atoi(val)
			case "h1":
				cfg.Interface.H1 = val
			case "h2":
				cfg.Interface.H2 = val
			case "h3":
				cfg.Interface.H3 = val
			case "h4":
				cfg.Interface.H4 = val
			case "i1":
				cfg.Interface.I1 = val
			case "i2":
				cfg.Interface.I2 = val
			case "i3":
				cfg.Interface.I3 = val
			case "i4":
				cfg.Interface.I4 = val
			case "i5":
				cfg.Interface.I5 = val
			case "dns":
				cfg.Interface.DNS = val
			case "postup":
				cfg.PostUp = val
			case "postdown":
				cfg.PostDown = val
			}
		case "peer":
			if currentPeer == nil {
				continue
			}
			switch strings.ToLower(key) {
			case "publickey":
				currentPeer.PublicKey = val
			case "presharedkey":
				currentPeer.PresharedKey = val
			case "allowedips":
				currentPeer.AllowedIPs = val
			case "endpoint":
				currentPeer.Endpoint = val
			case "persistentkeepalive":
				currentPeer.PersistentKeepalive, _ = strconv.Atoi(val)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan config %s: %w", path, err)
	}

	return cfg, nil
}

// ImportAllAWGConfigs scans and parses all existing AWG configs.
func ImportAllAWGConfigs() ([]*ImportedAWGConfig, error) {
	paths, err := ScanAWGConfigs()
	if err != nil {
		return nil, err
	}
	var configs []*ImportedAWGConfig
	for _, path := range paths {
		cfg, err := ParseAWGConfig(path)
		if err != nil {
			fmt.Printf("[LUCX-AWG] ImportAll: skipping broken config %s: %v\n", path, err); continue
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

// HasObfuscation returns true if the imported config has non-default obfuscation params.
func (c *ImportedAWGConfig) HasObfuscation() bool {
	iface := c.Interface
	return iface.Jc > 0 || iface.Jmin > 0 || iface.Jmax > 0 ||
		iface.S1 > 0 || iface.S2 > 0 || iface.S3 > 0 || iface.S4 > 0 ||
		iface.H1 != "" || iface.H2 != "" || iface.H3 != "" || iface.H4 != "" ||
		iface.I1 != "" || iface.I2 != "" || iface.I3 != "" || iface.I4 != "" || iface.I5 != ""
}

// AWGIDFromPath extracts the AWG interface ID from a config path (e.g., "awg0.conf" → 0).
func AWGIDFromPath(path string) int {
	base := filepath.Base(path)
	idStr := strings.TrimPrefix(base, "awg")
	idStr = strings.TrimSuffix(idStr, ".conf")
	id, _ := strconv.Atoi(idStr)
	return id
}
