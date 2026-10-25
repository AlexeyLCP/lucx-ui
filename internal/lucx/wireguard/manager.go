// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

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
	kind        string
	cleanup     bool
}
type Manager struct {
	mu           sync.Mutex
	dir          string
	command      Command
	current      map[int]managed
	counters     *Counters
	pending      []Usage
	inputWarning bool
	wgBin        string
	wgKind       string
	wgResolved   bool
	wgErr        error
	system       bool
}

var diagnosticKeys = regexp.MustCompile(`[A-Za-z0-9+/]{32,}=?`)

var singleton struct {
	sync.Once
	manager *Manager
}

func GetManager() *Manager {
	singleton.Do(func() { singleton.manager = NewManager(filepath.Join(config.GetDBFolderPath(), "wireguard"), nil) })
	return singleton.manager
}

func NewManager(dir string, command Command) *Manager {
	system := command == nil
	if system {
		command = systemCommand
	}
	return &Manager{dir: dir, command: command, current: map[int]managed{}, counters: NewCounters(), wgBin: "wg", wgKind: "wireguard", system: system}
}

// wgTools resolves once which wg-class binary manages the netdevs: the awg
// binary speaks the "amneziawg" genl family and cannot touch a vanilla
// wireguard netdev (and vice versa), so the binary and interface type must
// change together. Test mode (command != systemCommand) pins wg defaults.
func (m *Manager) wgTools() (bin, kind string, err error) {
	if m.wgResolved {
		return m.wgBin, m.wgKind, m.wgErr
	}
	if !m.system {
		m.wgErr = nil
	} else if bin, e := lookPath("wg"); e == nil {
		m.wgBin, m.wgKind, m.wgErr = bin, "wireguard", nil
	} else if bin, e = lookPath("awg"); e == nil {
		m.wgBin, m.wgKind, m.wgErr = bin, "amneziawg", nil
	} else {
		m.wgErr = fmt.Errorf(`neither "wg" nor "awg" binary found in PATH; install wireguard-tools or amneziawg-tools`)
	}
	m.wgResolved = true
	return m.wgBin, m.wgKind, m.wgErr
}

func (m *Manager) wgRun(args ...string) ([]byte, error) {
	bin, _, err := m.wgTools()
	if err != nil {
		return nil, err
	}
	return m.run(append([]string{bin}, args...)...)
}

type diagnosticBuffer struct{ data []byte }

func (b *diagnosticBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 4096 - len(b.data); remaining > 0 {
		b.data = append(b.data, p[:min(remaining, len(p))]...)
	}
	return n, nil
}

func (b *diagnosticBuffer) String() string { return strings.ToValidUTF8(string(b.data), "�") }

func missingInputRule(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

var lookPath = exec.LookPath

func systemCommand(input string, args ...string) ([]byte, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("kernel WireGuard requires Linux")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(input)
	var stderr diagnosticBuffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		code := -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
		detail := diagnosticKeys.ReplaceAllString(stderr.String(), "[redacted-key]")
		detail = strings.Join(strings.Fields(detail), " ")
		if detail == "" {
			detail = err.Error()
		}
		if len(detail) > 512 {
			detail = detail[:512]
			for !utf8.ValidString(detail) {
				detail = detail[:len(detail)-1]
			}
			detail += "…"
		}
		return nil, fmt.Errorf("kernel WireGuard %s operation failed (exit %d): %s: %w", args[0], code, detail, err)
	}
	return out, nil
}
func (m *Manager) run(args ...string) ([]byte, error) { return m.command("", args...) }
func (m *Manager) path(i Instance) string             { return filepath.Join(m.dir, i.Name()+".conf") }

func (m *Manager) owned(i Instance) (bool, string, error) {
	out, err := m.run("ip", "-j", "-d", "link", "show", "dev", i.Name())
	if err != nil {
		return false, "", nil
	}
	var rows []struct {
		Alias string `json:"ifalias"`
		Info  struct {
			Kind string `json:"info_kind"`
		} `json:"linkinfo"`
	}
	if json.Unmarshal(out, &rows) != nil || len(rows) != 1 ||
		(rows[0].Info.Kind != "wireguard" && rows[0].Info.Kind != "amneziawg") {
		return true, "", fmt.Errorf("refusing unowned interface %s", i.Name())
	}
	if rows[0].Alias != owner {
		return true, rows[0].Info.Kind, fmt.Errorf("refusing unowned interface %s", i.Name())
	}
	return true, rows[0].Info.Kind, nil
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
	bin, _, err := m.wgTools()
	if err != nil {
		return err
	}
	_, err = m.run(bin, operation, i.Name(), f.Name())
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

func inputRuleArgs(i Instance, operation string) []string {
	args := []string{"iptables", "-w", "5", operation, "INPUT"}
	if operation == "-I" {
		args = append(args, "1")
	}
	return append(args, "-i", i.Name(), "-m", "mark", "--mark", i.Mark(), "-m", "comment", "--comment", owner, "-j", "ACCEPT")
}

func (m *Manager) ensureInput(i Instance) error {
	if _, err := m.run(inputRuleArgs(i, "-C")...); err == nil {
		return nil
	} else if errors.Is(err, exec.ErrNotFound) {
		if !m.inputWarning {
			logger.Warning("kernel WireGuard: iptables unavailable; allow marked tunnel input in the host nftables firewall")
			m.inputWarning = true
		}
		return nil
	} else if !missingInputRule(err) {
		return err
	}
	_, err := m.run(inputRuleArgs(i, "-I")...)
	return err
}

func (m *Manager) sample(i Instance) {
	out, err := m.wgRun("show", i.Name(), "dump")
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
	bin, kind, err := m.wgTools()
	if err != nil {
		return err
	}
	exists, liveKind, err := m.owned(i)
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
	fingerprint := i.DeviceFingerprint() + "\n" + kind
	current, known := m.current[i.ID]
	if exists {
		if known {
			m.sample(current.inst)
		}
		if !known {
			m.sample(i)
		}
	}
	if known && (current.fingerprint != fingerprint || current.kind != kind) {
		if err := m.remove(i.ID); err != nil {
			return err
		}
		exists = false
		known = false
	}
	if exists && !known && liveKind != "" && liveKind != kind {
		if _, err = m.run("ip", "link", "delete", "dev", i.Name()); err != nil {
			return err
		}
		exists = false
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
	if err := m.ensureInput(i); err != nil {
		return err
	}
	if err := m.firewall(i); err != nil {
		return err
	}
	if !exists {
		if _, err = m.run("ip", "link", "add", "dev", i.Name(), "type", kind); err != nil {
			return err
		}
		if _, err = m.run("ip", "link", "set", "dev", i.Name(), "alias", owner); err != nil {
			_, _ = m.run("ip", "link", "delete", "dev", i.Name())
			return err
		}
	}
	if !exists {
		m.current[i.ID] = managed{inst: i, kind: kind}
	}
	fail := func(e error) error {
		if !exists {
			_ = m.remove(i.ID)
		}
		return e
	}
	if exists && (!known || current.inst.FwMark != i.FwMark) {
		if _, err = m.run(bin, "set", i.Name(), "fwmark", strconv.FormatUint(uint64(i.FwMark), 10)); err != nil {
			return err
		}
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
	m.current[i.ID] = managed{inst: i, fingerprint: fingerprint, peers: peerFP, kind: kind}
	return nil
}

// RevokeDisallowed only removes access; invalid device settings must never grant peers.
func (m *Manager) RevokeDisallowed(ib *model.Inbound) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ib.Id < 1 || ib.Id > 65535 {
		return fmt.Errorf("invalid kernel WireGuard ID")
	}
	i := Instance{ID: ib.Id}
	exists, _, err := m.owned(i)
	if err != nil || !exists {
		return err
	}
	type accessPeer struct {
		PublicKey string `json:"publicKey"`
		Email     string `json:"email"`
		Enable    *bool  `json:"enable"`
	}
	var access struct {
		Clients []accessPeer `json:"clients"`
		Peers   []accessPeer `json:"peers"`
	}
	decodeErr := json.Unmarshal([]byte(ib.Settings), &access)
	allowed := map[string]bool{}
	disabled := map[string]bool{}
	for _, ct := range ib.ClientStats {
		if !ct.Enable || ct.ExpiryTime > 0 && ct.ExpiryTime <= time.Now().UnixMilli() || ct.Total > 0 && ct.Up+ct.Down >= ct.Total {
			disabled[ct.Email] = true
		}
	}
	if decodeErr == nil && (len(access.Clients) == 0 || len(access.Peers) == 0) {
		for _, p := range append(access.Clients, access.Peers...) {
			if validKey(p.PublicKey) && (p.Enable == nil || *p.Enable) && !disabled[p.Email] {
				allowed[p.PublicKey] = true
			}
		}
	}
	current, known := m.current[i.ID]
	if known {
		m.sample(current.inst)
	}
	out, err := m.wgRun("show", i.Name(), "peers")
	if err != nil {
		return err
	}
	for _, key := range strings.Fields(string(out)) {
		if !validKey(key) {
			return fmt.Errorf("invalid live WireGuard peer key")
		}
		if allowed[key] {
			continue
		}
		if _, err = m.wgRun("set", i.Name(), "peer", key, "remove"); err != nil {
			return err
		}
		if known {
			var peers []Peer
			for _, p := range current.inst.Peers {
				if p.PublicKey != key {
					peers = append(peers, p)
				}
			}
			current.inst.Peers = peers
			current.peers = ""
			m.current[i.ID] = current
		}
	}
	if decodeErr != nil {
		return fmt.Errorf("unreadable kernel WireGuard access list; all live peers revoked")
	}
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
	exists, _, err := m.owned(i)
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
	if _, e := m.run(inputRuleArgs(i, "-C")...); e == nil {
		if _, e = m.run(inputRuleArgs(i, "-D")...); e != nil {
			return e
		}
	} else if !missingInputRule(e) && !errors.Is(e, exec.ErrNotFound) {
		return e
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

func (m *Manager) Reconcile(desired []Instance) []error { return m.ReconcilePreserving(desired, nil) }

func (m *Manager) ReconcilePreserving(desired []Instance, preserve []int) []error {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := map[int]bool{}
	for _, id := range preserve {
		want[id] = true
	}
	var errors []error
	for _, i := range desired {
		want[i.ID] = true
		if err := m.ensure(i); err != nil {
			errors = append(errors, fmt.Errorf("kernel WireGuard inbound %d: %w", i.ID, err))
		}
	}
	links := make([]struct {
		Name  string `json:"ifname"`
		Alias string `json:"ifalias"`
	}, 0, 2)
	for _, scannedKind := range []string{"wireguard", "amneziawg"} {
		rows, scanErr := m.run("ip", "-j", "-d", "link", "show", "type", scannedKind)
		if scanErr != nil {
			continue
		}
		var found []struct {
			Name  string `json:"ifname"`
			Alias string `json:"ifalias"`
		}
		if json.Unmarshal(rows, &found) != nil {
			continue
		}
		links = append(links, found...)
	}
	for _, row := range links {
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
