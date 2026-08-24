// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverPaths is the host layout the importer scans. Tests point these at temp dirs.
type DiscoverPaths struct {
	AmneziaDir  string
	Toolza3Dir  string
	DockerRoots []string
	ClientDirs  []string
}

// DefaultDiscoverPaths is the production layout on a Linux VPS.
func DefaultDiscoverPaths() DiscoverPaths {
	return DiscoverPaths{
		AmneziaDir:  awgConfigDir,
		Toolza3Dir:  "/etc/awg3",
		DockerRoots: []string{"/opt/amnezia"},
		ClientDirs:  []string{"/root", "/etc/awg3/clients"},
	}
}

// ImportPeer is one peer row in the preview table.
type ImportPeer struct {
	Email      string `json:"email"`
	AllowedIPs string `json:"allowedIPs"`
	PublicKey  string `json:"publicKey"`
	HasKey     bool   `json:"hasKey"`
	Suspended  bool   `json:"suspended"`
}

// ImportCandidate is one unmanaged interface the operator can import.
type ImportCandidate struct {
	ID           string                   `json:"id"`
	Source       string                   `json:"source"`
	Ifname       string                   `json:"ifname"`
	ConfPath     string                   `json:"confPath"`
	Live         bool                     `json:"live"`
	Port         int                      `json:"port"`
	Address      string                   `json:"address"`
	AwgVersion   string                   `json:"awgVersion"`
	PeerCount    int                      `json:"peerCount"`
	NamedPeers   int                      `json:"namedPeers"`
	KeysFound    int                      `json:"keysFound"`
	Handshakes   int                      `json:"handshakes"`
	Suspended    int                      `json:"suspended"`
	Backend      string                   `json:"backend"`
	DropOnImport bool                     `json:"dropOnImport"`
	Warning      string                   `json:"warning"`
	Peers        []ImportPeer             `json:"peers"`
	Conf         ServerConf               `json:"-"`
	Keys         map[string]ClientKeyFile `json:"-"`
}

// Discover finds unmanaged AWG server configs. Managed x-ui files are skipped.
func Discover(paths DiscoverPaths) []ImportCandidate {
	keys := indexClientKeys(paths.ClientDirs)
	if paths.Toolza3Dir != "" {
		for k, v := range indexClientKeys([]string{filepath.Join(paths.Toolza3Dir, "clients")}) {
			keys[k] = v
		}
	}
	var out []ImportCandidate
	seen := map[string]struct{}{}
	add := func(c ImportCandidate) {
		if c.Conf.PrivateKey == "" || c.Conf.ListenPort == 0 {
			return
		}
		if _, ok := seen[c.ID]; ok {
			return
		}
		seen[c.ID] = struct{}{}
		out = append(out, finishCandidate(c, keys))
	}
	if paths.AmneziaDir != "" {
		for _, c := range scanConfDir(paths.AmneziaDir, ImportSourceMulti, "kernel", false) {
			add(c)
		}
	}
	if paths.Toolza3Dir != "" {
		for _, c := range scanConfDir(paths.Toolza3Dir, ImportSourceToolza3, "userspace", true) {
			add(c)
		}
	}
	for _, root := range paths.DockerRoots {
		for _, c := range scanDockerRoot(root) {
			add(c)
		}
	}
	return out
}

func scanConfDir(dir, source, backend string, drop bool) []ImportCandidate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []ImportCandidate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		if strings.HasPrefix(e.Name(), "awgo-") || strings.HasPrefix(e.Name(), "client") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if configIsManaged(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		conf := ParseServerConf(string(data))
		ifname := strings.TrimSuffix(e.Name(), ".conf")
		if !isImportIfname(ifname) {
			ifname = guessIfname(ifname)
		}
		out = append(out, ImportCandidate{
			ID:           source + ":" + ifname,
			Source:       source,
			Ifname:       ifname,
			ConfPath:     path,
			Port:         conf.ListenPort,
			Address:      conf.Address,
			AwgVersion:   conf.AwgVersion,
			Backend:      backend,
			DropOnImport: drop,
			Conf:         conf,
		})
	}
	return out
}

func scanDockerRoot(root string) []ImportCandidate {
	var out []ImportCandidate
	for _, sub := range []string{"awg", "awg2", "awg3"} {
		dir := filepath.Join(root, sub)
		for _, name := range []string{"wg0.conf", "awg0.conf", "awg.conf"} {
			path := filepath.Join(dir, name)
			if configIsManaged(path) {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			conf := ParseServerConf(string(data))
			table := filepath.Join(dir, "clientsTable")
			out = append(out, ImportCandidate{
				ID:           ImportSourceDocker + ":" + sub,
				Source:       ImportSourceDocker,
				Ifname:       sub,
				ConfPath:     path,
				Port:         conf.ListenPort,
				Address:      conf.Address,
				AwgVersion:   conf.AwgVersion,
				Backend:      "docker",
				DropOnImport: true,
				Warning:      "Docker Amnezia must be stopped after import; clients reconnect on the kernel interface.",
				Conf:         conf,
				Keys:         indexAmneziaClientsTable(table),
			})
		}
	}
	return out
}

func isImportIfname(name string) bool {
	return isInboundAwgInterface(name) || name == "wg0" || name == "awg"
}

func guessIfname(fileBase string) string {
	if fileBase != "" {
		return fileBase
	}
	return "awg"
}

func finishCandidate(c ImportCandidate, keys map[string]ClientKeyFile) ImportCandidate {
	if c.Keys == nil {
		c.Keys = map[string]ClientKeyFile{}
	}
	for k, v := range keys {
		if _, ok := c.Keys[k]; !ok {
			c.Keys[k] = v
		}
	}
	used := map[string]struct{}{}
	c.Peers = c.Peers[:0]
	c.NamedPeers = 0
	c.KeysFound = 0
	c.Suspended = 0
	for _, p := range c.Conf.Peers {
		email := sanitizeEmail(p.Name, p.AllowedIPs, p.PublicKey, used)
		if p.Name != "" {
			c.NamedPeers++
		}
		if p.Suspended {
			c.Suspended++
		}
		_, has := c.Keys[p.PublicKey]
		if has {
			c.KeysFound++
		}
		c.Peers = append(c.Peers, ImportPeer{
			Email:      email,
			AllowedIPs: p.AllowedIPs,
			PublicKey:  p.PublicKey,
			HasKey:     has,
			Suspended:  p.Suspended,
		})
	}
	c.PeerCount = len(c.Conf.Peers)
	c.Live = interfaceIsUp(c.Ifname)
	if c.Source == ImportSourceToolza3 && c.Warning == "" {
		c.DropOnImport = true
		c.Warning = "toolza3 uses userspace amneziawg-go; stop awg3 after import."
	}
	if dns, _ := firstClientHints(c.Keys); c.Conf.DNS == "" && dns != "" {
		c.Conf.DNS = dns
	}
	applyClientCPS(&c.Conf, c.Keys)
	return c
}

func firstClientHints(keys map[string]ClientKeyFile) (dns string, _ ClientKeyFile) {
	for _, k := range keys {
		if k.DNS != "" {
			return k.DNS, k
		}
	}
	return "", ClientKeyFile{}
}

func applyClientCPS(conf *ServerConf, keys map[string]ClientKeyFile) {
	if conf.I1 != "" {
		return
	}
	for _, k := range keys {
		if k.I1 == "" {
			continue
		}
		conf.I1, conf.I2, conf.I3, conf.I4, conf.I5 = k.I1, k.I2, k.I3, k.I4, k.I5
		if conf.AwgVersion == "1.5" {
			conf.AwgVersion = "2"
		}
		return
	}
}

func indexClientKeys(dirs []string) map[string]ClientKeyFile {
	out := map[string]ClientKeyFile{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			k := ParseClientKeyFile(string(data))
			if k.PrivateKey == "" {
				continue
			}
			if pub := PublicKeyOf(k.PrivateKey); pub != "" {
				out[pub] = k
			}
		}
	}
	return out
}

func indexAmneziaClientsTable(path string) map[string]ClientKeyFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil
	}
	out := map[string]ClientKeyFile{}
	for _, row := range rows {
		priv, _ := row["clientPrivKey"].(string)
		if priv == "" {
			priv, _ = row["privateKey"].(string)
		}
		if priv == "" {
			if cfg, ok := row["config"].(string); ok {
				k := ParseClientKeyFile(cfg)
				if pub := PublicKeyOf(k.PrivateKey); pub != "" {
					out[pub] = k
				}
				continue
			}
		}
		if priv == "" {
			continue
		}
		k := ClientKeyFile{PrivateKey: priv}
		if pub := PublicKeyOf(priv); pub != "" {
			out[pub] = k
		}
	}
	return out
}

func interfaceIsUp(ifname string) bool {
	if ifname == "" {
		return false
	}
	_, err := os.Stat("/sys/class/net/" + ifname)
	return err == nil
}

// isInboundAwgInterface reports whether name is an inbound AWG interface
// (e.g. "awg1") and NOT an outbound one (e.g. "awgo-1").
func isInboundAwgInterface(name string) bool {
	if !strings.HasPrefix(name, "awg") {
		return false
	}
	rest := name[len("awg"):]
	if rest == "" {
		return false
	}
	r := rune(rest[0])
	return r >= '0' && r <= '9'
}
