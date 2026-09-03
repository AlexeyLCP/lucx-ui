// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DiscoverPaths is the host layout the importer scans. Tests point these at temp dirs.
type DiscoverPaths struct {
	AmneziaDir     string
	Toolza3Dir     string
	DockerRoots    []string
	ClientDirs     []string
	ScanLiveDocker bool
	DockerList     func() []string
	DockerRead     func(container, path string) ([]byte, error)
	DockerStop     func(container string) error
}

// DefaultDiscoverPaths is the production layout on a Linux VPS.
func DefaultDiscoverPaths() DiscoverPaths {
	return DiscoverPaths{
		AmneziaDir:     awgConfigDir,
		Toolza3Dir:     "/etc/awg3",
		DockerRoots:    []string{"/opt/amnezia"},
		ClientDirs:     []string{"/root", "/etc/awg3/clients", awgConfigDir},
		ScanLiveDocker: true,
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
	ExtraPaths   []string                 `json:"-"`
	ConfText     string                   `json:"-"`
	TableText    string                   `json:"-"`
	StopTarget   string                   `json:"stopTarget"`
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
		if _, ok := seen["pk:"+c.Conf.PrivateKey]; ok {
			return
		}
		seen[c.ID] = struct{}{}
		seen["pk:"+c.Conf.PrivateKey] = struct{}{}
		out = append(out, finishCandidate(c, keys))
	}
	if paths.AmneziaDir != "" {
		for _, c := range scanConfDir(paths.AmneziaDir, ImportSourceMulti, "kernel", false) {
			add(c)
		}
		for _, c := range scanOutboundConfDir(paths.AmneziaDir) {
			if c.Conf.PrivateKey == "" {
				continue
			}
			if _, ok := seen[c.ID]; ok {
				continue
			}
			if _, ok := seen["pk:"+c.Conf.PrivateKey]; ok {
				continue
			}
			seen[c.ID] = struct{}{}
			seen["pk:"+c.Conf.PrivateKey] = struct{}{}
			out = append(out, finishOutboundCandidate(c))
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
	if paths.ScanLiveDocker {
		for _, c := range scanLiveDocker(paths) {
			add(c)
		}
	}
	if out == nil {
		return []ImportCandidate{}
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
		if isClientConf(conf) {
			continue
		}
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
	for _, sub := range []string{"awg", "awg2", "awg3", "wireguard"} {
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
			keys := indexAmneziaClientsTable(table)
			extra := []string{}
			if _, err := os.Stat(table); err == nil {
				extra = append(extra, table)
			}
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
				StopTarget:   "docker:amnezia-" + sub,
				Warning:      "After import the old Docker container is stopped so the kernel iface can take the port.",
				Conf:         conf,
				Keys:         keys,
				ExtraPaths:   extra,
			})
		}
	}
	return out
}

func isImportIfname(name string) bool {
	return isInboundAwgInterface(name) || name == "wg0" || name == "awg"
}

func guessIfname(fileBase string) string {
	if isImportIfname(fileBase) {
		return fileBase
	}
	return "awg"
}

func finishCandidate(c ImportCandidate, keys map[string]ClientKeyFile) ImportCandidate {
	matched := map[string]ClientKeyFile{}
	if c.Keys != nil {
		for _, p := range c.Conf.Peers {
			if k, ok := c.Keys[p.PublicKey]; ok {
				matched[p.PublicKey] = k
			}
		}
	}
	for _, p := range c.Conf.Peers {
		if _, ok := matched[p.PublicKey]; ok {
			continue
		}
		if k, ok := keys[p.PublicKey]; ok {
			matched[p.PublicKey] = k
		}
	}
	c.Keys = matched
	for i := range c.Conf.Peers {
		k, ok := matched[c.Conf.Peers[i].PublicKey]
		if !ok {
			continue
		}
		if c.Conf.Peers[i].Name == "" {
			c.Conf.Peers[i].Name = k.Name
		}
		if c.Conf.Peers[i].AllowedIPs == "" && k.AllowedIPs != "" {
			c.Conf.Peers[i].AllowedIPs = k.AllowedIPs
		}
	}
	used := map[string]struct{}{}
	c.Peers = make([]ImportPeer, 0, len(c.Conf.Peers))
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
		k, has := c.Keys[p.PublicKey]
		hasKey := has && k.PrivateKey != ""
		if hasKey {
			c.KeysFound++
		}
		c.Peers = append(c.Peers, ImportPeer{
			Email:      email,
			AllowedIPs: p.AllowedIPs,
			PublicKey:  p.PublicKey,
			HasKey:     hasKey,
			Suspended:  p.Suspended,
		})
	}
	c.PeerCount = len(c.Conf.Peers)
	c.Live = interfaceIsUp(c.Ifname)
	if c.Source == ImportSourceToolza3 && c.Warning == "" {
		c.DropOnImport = true
		c.StopTarget = "systemd:awg3"
		c.Warning = "After import the awg3 service is stopped so the kernel iface can take the port."
	}
	if dns := firstPeerDNS(c.Conf.Peers, c.Keys); c.Conf.DNS == "" && dns != "" {
		c.Conf.DNS = dns
	}
	applyClientCPS(&c.Conf, c.Keys)
	return c
}

func firstPeerDNS(peers []ServerPeer, keys map[string]ClientKeyFile) string {
	for _, p := range peers {
		if k, ok := keys[p.PublicKey]; ok && k.DNS != "" {
			return k.DNS
		}
	}
	return ""
}

func applyClientCPS(conf *ServerConf, keys map[string]ClientKeyFile) {
	if conf.I1 != "" {
		return
	}
	for _, p := range conf.Peers {
		k, ok := keys[p.PublicKey]
		if !ok || k.I1 == "" {
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
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			k := ParseClientKeyFile(string(data))
			if k.PrivateKey == "" {
				continue
			}
			k.Path = path
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
	return parseAmneziaClientsTable(data, path)
}

func parseAmneziaClientsTable(data []byte, path string) map[string]ClientKeyFile {
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil
	}
	out := map[string]ClientKeyFile{}
	for _, row := range rows {
		name := amneziaRowName(row)
		priv, _ := row["clientPrivKey"].(string)
		if priv == "" {
			priv, _ = row["privateKey"].(string)
		}
		if priv == "" {
			if cfg, ok := row["config"].(string); ok && cfg != "" {
				k := ParseClientKeyFile(cfg)
				k.Path = path
				k.Name = name
				if pub := PublicKeyOf(k.PrivateKey); pub != "" {
					out[pub] = k
					continue
				}
			}
		}
		pub := strings.TrimSpace(rowString(row, "clientId", "publicKey"))
		if priv != "" {
			if derived := PublicKeyOf(priv); derived != "" {
				pub = derived
			}
		}
		if pub == "" {
			continue
		}
		k := out[pub]
		k.Path = path
		if priv != "" {
			k.PrivateKey = priv
		}
		if name != "" {
			k.Name = name
		}
		if ips := amneziaRowAllowed(row); ips != "" {
			k.AllowedIPs = ips
		}
		out[pub] = k
	}
	return out
}

func amneziaRowName(row map[string]any) string {
	if s := rowString(row, "clientName", "name"); s != "" {
		return s
	}
	ud, ok := row["userData"].(map[string]any)
	if !ok {
		return ""
	}
	return rowString(ud, "clientName", "name")
}

func amneziaRowAllowed(row map[string]any) string {
	if s := rowString(row, "allowedIps", "allowedIPs"); s != "" {
		return s
	}
	ud, ok := row["userData"].(map[string]any)
	if !ok {
		return ""
	}
	return rowString(ud, "allowedIps", "allowedIPs")
}

func rowString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, _ := row[key].(string); strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
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

func isOutboundAwgInterface(name string) bool {
	if !strings.HasPrefix(name, "awgo-") {
		return false
	}
	rest := name[len("awgo-"):]
	if rest == "" {
		return false
	}
	for i := 0; i < len(rest); i++ {
		if rest[i] < '0' || rest[i] > '9' {
			return false
		}
	}
	return true
}

func isClientConf(conf ServerConf) bool {
	for _, p := range conf.Peers {
		if strings.TrimSpace(p.Endpoint) != "" {
			return true
		}
	}
	return false
}

func scanOutboundConfDir(dir string) []ImportCandidate {
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
		if !isClientConf(conf) || conf.PrivateKey == "" || conf.Address == "" {
			continue
		}
		ifname := strings.TrimSuffix(e.Name(), ".conf")
		if isInboundAwgInterface(ifname) {
			continue
		}
		peer := conf.Peers[0]
		for _, p := range conf.Peers {
			if p.Endpoint != "" {
				peer = p
				break
			}
		}
		port := 0
		if _, portStr, err := net.SplitHostPort(peer.Endpoint); err == nil {
			port, _ = strconv.Atoi(portStr)
		}
		out = append(out, ImportCandidate{
			ID:         ImportSourceOutbound + ":" + ifname,
			Source:     ImportSourceOutbound,
			Ifname:     ifname,
			ConfPath:   path,
			Port:       port,
			Address:    conf.Address,
			AwgVersion: conf.AwgVersion,
			Backend:    "kernel",
			Conf:       conf,
		})
	}
	return out
}

func finishOutboundCandidate(c ImportCandidate) ImportCandidate {
	c.PeerCount = len(c.Conf.Peers)
	c.KeysFound = 1
	c.NamedPeers = 1
	c.Live = interfaceIsUp(c.Ifname)
	c.Peers = []ImportPeer{}
	for _, p := range c.Conf.Peers {
		if p.Endpoint == "" {
			continue
		}
		c.Peers = append(c.Peers, ImportPeer{
			Email:      p.Endpoint,
			AllowedIPs: p.AllowedIPs,
			PublicKey:  p.PublicKey,
			HasKey:     true,
		})
	}
	return c
}
