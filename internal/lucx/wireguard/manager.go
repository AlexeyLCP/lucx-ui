// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type (
	Command func(input string, args ...string) ([]byte, error)
	Usage   struct {
		Delta
		Tag   string
		Email string
	}
)

type managed struct {
	inst        Instance
	fingerprint string
	peers       string
	cleanup     bool
}
type Manager struct {
	mu       sync.Mutex
	dir      string
	command  Command
	current  map[int]managed
	counters *Counters
	pending  []Usage
}

var singleton struct {
	sync.Once
	manager *Manager
}

func GetManager() *Manager {
	singleton.Do(func() { singleton.manager = NewManager(filepath.Join(config.GetDBFolderPath(), "wireguard"), nil) })
	return singleton.manager
}

func NewManager(dir string, command Command) *Manager {
	if command == nil {
		command = systemCommand
	}
	return &Manager{dir: dir, command: command, current: map[int]managed{}, counters: NewCounters()}
}

func systemCommand(input string, args ...string) ([]byte, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("kernel WireGuard requires Linux")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kernel WireGuard %s operation failed", args[0])
	}
	return out, nil
}
func (m *Manager) run(args ...string) ([]byte, error) { return m.command("", args...) }
func (m *Manager) path(i Instance) string             { return filepath.Join(m.dir, i.Name()+".conf") }

func (m *Manager) owned(i Instance) (bool, error) {
	out, err := m.run("ip", "-j", "-d", "link", "show", "dev", i.Name())
	if err != nil {
		return false, nil
	}
	var rows []struct {
		Alias string `json:"ifalias"`
		Info  struct {
			Kind string `json:"info_kind"`
		} `json:"linkinfo"`
	}
	if json.Unmarshal(out, &rows) != nil || len(rows) != 1 || rows[0].Alias != owner || rows[0].Info.Kind != "wireguard" {
		return true, fmt.Errorf("refusing unowned interface %s", i.Name())
	}
	return true, nil
}

func writePrivate(path, text string) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular WireGuard config")
		}
		old, e := os.ReadFile(path)
		if e != nil || !strings.HasPrefix(string(old), "# "+owner+"\n") {
			return fmt.Errorf("refusing unowned WireGuard config")
		}
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".wg-*.conf")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func stripQuick(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Address =") || strings.HasPrefix(line, "MTU =") || strings.HasPrefix(line, "Table =") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m *Manager) configure(i Instance, operation string) error {
	f, err := os.CreateTemp(m.dir, ".sync-*.conf")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(stripQuick(RenderConfig(i))); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	_, err = m.run("wg", operation, i.Name(), f.Name())
	return err
}

func ownedFirewall(data []byte, i Instance) bool {
	var parsed struct {
		Entries []struct {
			Table struct {
				Comment string `json:"comment"`
			} `json:"table"`
			Rule struct {
				Family  string `json:"family"`
				Table   string `json:"table"`
				Comment string `json:"comment"`
			} `json:"rule"`
		} `json:"nftables"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return false
	}
	for _, entry := range parsed.Entries {
		if entry.Table.Comment == owner || entry.Rule.Family == "inet" && entry.Rule.Table == i.FirewallTable() && entry.Rule.Comment == owner {
			return true
		}
	}
	return false
}

func (m *Manager) firewall(i Instance) error {
	prefix := ""
	out, err := m.run("nft", "-j", "list", "table", "inet", i.FirewallTable())
	if err == nil {
		if !ownedFirewall(out, i) {
			return fmt.Errorf("refusing unowned nft table %s", i.FirewallTable())
		}
		prefix = "delete table inet " + i.FirewallTable() + "\n"
	}
	_, err = m.command(prefix+RenderFirewall(i), "nft", "-f", "-")
	return err
}

func (m *Manager) sample(i Instance) {
	out, err := m.run("wg", "show", i.Name(), "dump")
	if err != nil {
		return
	}
	peers, err := ParseDump(string(out))
	if err != nil {
		return
	}
	byKey := map[string]string{}
	for _, p := range i.Peers {
		byKey[p.PublicKey] = p.Email
	}
	for _, d := range m.counters.Observe(i.ID, peers, time.Now()) {
		m.pending = append(m.pending, Usage{Delta: d, Tag: i.Tag, Email: byKey[d.PublicKey]})
	}
}
func (m *Manager) Ensure(i Instance) error { m.mu.Lock(); defer m.mu.Unlock(); return m.ensure(i) }
func (m *Manager) ensure(i Instance) error {
	if old, ok := m.current[i.ID]; ok && old.cleanup {
		if err := m.remove(i.ID); err != nil {
			return err
		}
	}
	exists, err := m.owned(i)
	if err != nil {
		return err
	}
	if info, e := os.Lstat(m.dir); e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return fmt.Errorf("refusing non-directory WireGuard store")
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	if !exists {
		if _, known := m.current[i.ID]; !known {
			if text, e := os.ReadFile(m.path(i)); e == nil && strings.HasPrefix(string(text), "# "+owner+"\n") {
				m.current[i.ID] = managed{inst: Instance{ID: i.ID}, cleanup: true}
				if err := m.remove(i.ID); err != nil {
					return err
				}
			}
		}
	}

	conf := RenderConfig(i)
	peers := strings.SplitN(conf, "[Peer]", 2)
	peerFP := ""
	if len(peers) == 2 {
		peerFP = peers[1]
	}
	current, known := m.current[i.ID]
	if exists {
		if known {
			m.sample(current.inst)
		}
		if !known {
			m.sample(i)
		}
	}
	if known && current.fingerprint != i.DeviceFingerprint() {
		if err := m.remove(i.ID); err != nil {
			return err
		}
		exists = false
		known = false
	}
	if !exists {
		routes, _ := m.run("ip", "-j", "route", "show", "table", strconv.Itoa(i.Table()))
		if strings.TrimSpace(string(routes)) != "" && strings.TrimSpace(string(routes)) != "[]" {
			return fmt.Errorf("kernel WireGuard route table is already in use")
		}
		rules, err := m.run("ip", "-j", "rule", "show")
		if err != nil {
			return err
		}
		var entries []map[string]any
		if json.Unmarshal(rules, &entries) != nil {
			return fmt.Errorf("invalid routing rule response")
		}
		for _, entry := range entries {
			if fmt.Sprint(entry["fwmark"]) == i.Mark() || fmt.Sprint(entry["table"]) == strconv.Itoa(i.Table()) {
				return fmt.Errorf("kernel WireGuard routing resources are already in use")
			}
		}
		sockets, err := m.run("ss", "-H", "-lun", "sport", "=", strconv.Itoa(i.Port))
		if err != nil {
			return err
		}
		if len(strings.TrimSpace(string(sockets))) > 0 {
			return fmt.Errorf("kernel WireGuard UDP port is already in use")
		}
	}
	if err := writePrivate(m.path(i), conf); err != nil {
		return err
	}
	if err := m.firewall(i); err != nil {
		return err
	}
	if !exists {
		if _, err = m.run("ip", "link", "add", "dev", i.Name(), "type", "wireguard"); err != nil {
			return err
		}
		if _, err = m.run("ip", "link", "set", "dev", i.Name(), "alias", owner); err != nil {
			m.run("ip", "link", "delete", "dev", i.Name())
			return err
		}
	}
	if !exists {
		m.current[i.ID] = managed{inst: i}
	}
	fail := func(e error) error {
		if !exists {
			_ = m.remove(i.ID)
		}
		return e
	}
	if !exists || !known || current.peers != peerFP {
		op := "syncconf"
		if !exists {
			op = "setconf"
		}
		if err = m.configure(i, op); err != nil {
			return fail(err)
		}
	}
	if !exists || known {
		m.counters.SeedNew(i.ID, i.Peers)
	}

	for _, cmd := range [][]string{{"ip", "address", "replace", i.Address, "dev", i.Name()}, {"sysctl", "-qw", "net.ipv4.conf." + i.Name() + ".rp_filter=2"}, {"ip", "link", "set", "dev", i.Name(), "mtu", strconv.Itoa(i.MTU), "up"}} {
		if _, err = m.run(cmd...); err != nil {
			return fail(err)
		}
	}
	if _, err = m.run("ip", "-4", "route", "replace", "local", "default", "dev", "lo", "table", strconv.Itoa(i.Table()), "proto", "242"); err != nil {
		return fail(err)
	}
	rule, err := m.run("ip", "-4", "rule", "show", "iif", i.Name(), "fwmark", i.Mark(), "lookup", strconv.Itoa(i.Table()))
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(string(rule)) == "" {
		if _, err = m.run("ip", "-4", "rule", "add", "pref", "10001", "iif", i.Name(), "fwmark", i.Mark(), "lookup", strconv.Itoa(i.Table())); err != nil {
			return fail(err)
		}
	}
	m.current[i.ID] = managed{inst: i, fingerprint: i.DeviceFingerprint(), peers: peerFP}
	return nil
}
func (m *Manager) Remove(id int) error { m.mu.Lock(); defer m.mu.Unlock(); return m.remove(id) }
func (m *Manager) remove(id int) error {
	current, ok := m.current[id]
	if !ok {
		return nil
	}
	i := current.inst
	current.cleanup = true
	m.current[id] = current
	exists, err := m.owned(i)
	if err != nil {
		return err
	}
	if exists {
		m.sample(i)
		if _, err = m.run("ip", "link", "delete", "dev", i.Name()); err != nil {
			return err
		}
	}
	rule, err := m.run("ip", "-4", "rule", "show", "iif", i.Name(), "fwmark", i.Mark(), "lookup", strconv.Itoa(i.Table()))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(rule)) != "" {
		if _, err = m.run("ip", "-4", "rule", "del", "pref", "10001", "iif", i.Name(), "fwmark", i.Mark(), "lookup", strconv.Itoa(i.Table())); err != nil {
			return err
		}
	}
	routes, err := m.run("ip", "-j", "-4", "route", "show", "table", "all")
	if err != nil {
		return err
	}
	var routeRows []map[string]any
	if len(routes) > 0 && json.Unmarshal(routes, &routeRows) != nil {
		return fmt.Errorf("invalid route cleanup response")
	}
	for _, r := range routeRows {
		if fmt.Sprint(r["table"]) == strconv.Itoa(i.Table()) && fmt.Sprint(r["protocol"]) == "242" && r["type"] == "local" && r["dst"] == "default" && r["dev"] == "lo" {
			if _, err = m.run("ip", "-4", "route", "del", "local", "default", "dev", "lo", "table", strconv.Itoa(i.Table()), "proto", "242"); err != nil {
				return err
			}
		}
	}
	data, err := m.run("nft", "-j", "list", "table", "inet", i.FirewallTable())
	if err == nil {
		if !ownedFirewall(data, i) {
			return fmt.Errorf("refusing to remove unowned nft table")
		}
		if _, err = m.run("nft", "delete", "table", "inet", i.FirewallTable()); err != nil {
			return err
		}
	} else {
		tables, e := m.run("nft", "-j", "list", "tables")
		if e != nil {
			return e
		}
		if strings.Contains(string(tables), `"`+i.FirewallTable()+`"`) {
			return fmt.Errorf("unable to confirm nft table ownership")
		}
	}
	path := m.path(i)
	text, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if readErr == nil {
		info, e := os.Lstat(path)
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 || !strings.HasPrefix(string(text), "# "+owner+"\n") {
			return fmt.Errorf("refusing to remove unowned WireGuard config")
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	delete(m.current, id)
	m.counters.Observe(id, nil, time.Now())
	return nil
}

func (m *Manager) Reconcile(desired []Instance) []error {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := map[int]bool{}
	var errors []error
	for _, i := range desired {
		want[i.ID] = true
		if err := m.ensure(i); err != nil {
			errors = append(errors, fmt.Errorf("kernel WireGuard inbound %d: %w", i.ID, err))
		}
	}
	links, scanErr := m.run("ip", "-j", "-d", "link", "show", "type", "wireguard")
	if scanErr == nil {
		var rows []struct {
			Name  string `json:"ifname"`
			Alias string `json:"ifalias"`
		}
		if json.Unmarshal(links, &rows) == nil {
			for _, row := range rows {
				if row.Alias != owner || !strings.HasPrefix(row.Name, "lxwg") {
					continue
				}
				id, e := strconv.Atoi(strings.TrimPrefix(row.Name, "lxwg"))
				if e != nil || id < 1 || id > 65535 || want[id] {
					continue
				}
				if _, known := m.current[id]; !known {
					m.current[id] = managed{inst: Instance{ID: id}}
				}
			}
		}
	}
	if files, e := os.ReadDir(m.dir); e == nil {
		for _, f := range files {
			if !strings.HasPrefix(f.Name(), "lxwg") || !strings.HasSuffix(f.Name(), ".conf") || !f.Type().IsRegular() {
				continue
			}
			id, e := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(f.Name(), "lxwg"), ".conf"))
			if e != nil || id < 1 || id > 65535 || want[id] {
				continue
			}
			text, e := os.ReadFile(filepath.Join(m.dir, f.Name()))
			if e != nil || !strings.HasPrefix(string(text), "# "+owner+"\n") {
				continue
			}
			if _, known := m.current[id]; !known {
				m.current[id] = managed{inst: Instance{ID: id}, cleanup: true}
			}
		}
	}
	for id := range m.current {
		if !want[id] {
			if err := m.remove(id); err != nil {
				errors = append(errors, err)
			}
		}
	}
	return errors
}

func (m *Manager) CollectTraffic() []Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.current {
		m.sample(v.inst)
	}
	out := m.pending
	m.pending = nil
	return out
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.current {
		_ = m.remove(id)
	}
}
