// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// awgShowIfname runs `awg show <ifname> dump` and returns the combined
// stdout+stderr. The `dump` subcommand emits one tab-delimited line per peer
// (machine-readable) instead of the default multi-section human-readable
// output — that is what CollectClientTraffic parses. Used for liveness
// probing too: a non-nil error means the interface is down or absent, and
// `awg show <iface> dump` fails identically for a missing interface, so the
// liveness semantics are unchanged. Kept here rather than in process.go to
// avoid touching the server-side helper surface (scrapePeers runs its own
// `awg show <iface> dump` directly and is unaffected).
func awgShowIfname(ifname string) ([]byte, error) {
	return exec.CommandContext(context.Background(), awgBin("awg"), "show", ifname, "dump").CombinedOutput()
}

// clientState tracks one running client interface so EnsureClient can detect
// fingerprint changes and restart awg-quick when the operator edits settings.
type clientState struct {
	fp string
}

var (
	clientMu    sync.Mutex
	clients     = map[string]clientState{} // ifname -> state
	clientSwept sync.Once
)

// EnsureClient reconciles a single client AWG interface to desired state.
// Writes the awg-quick .conf (mode 0600 — awg-quick rejects world-readable
// configs because they contain the private key), runs `awg-quick up` if the
// interface is down, and restarts it when the fingerprint changed. Idempotent
// and safe to call every 10s from awg_job. Mirrors Manager.Ensure but for the
// client (egress) side. Lives on the same *Manager singleton as the server
// methods but keeps its own state map and mutex so the two sides never block
// each other.
func (m *Manager) EnsureClient(ci ClientInstance) error {
	if rebuildPaused.Load() {
		return fmt.Errorf("awg: module rebuild in progress")
	}
	m.sweepOrphanClientsOnce()
	clientMu.Lock()
	defer clientMu.Unlock()
	confPath := filepath.Join(awgConfigDir, ci.Ifname+".conf")
	// The rendered file IS the restart trigger here: an outbound has no
	// syncconf path, so every change goes in by taking the interface down and up.
	conf := renderClientConf(ci)
	if st, ok := clients[ci.Ifname]; ok && st.fp == conf {
		if _, err := awgShowIfname(ci.Ifname); err == nil {
			return nil
		}
	}
	if err := os.WriteFile(confPath, []byte(conf), 0o600); err != nil {
		return err
	}
	if _, err := awgShowIfname(ci.Ifname); err == nil {
		if out, err := awgQuick("down", confPath); err != nil {
			logger.Warningf("awg: awg-quick down failed before restart: %s %v", string(out), err)
		}
	}
	if out, err := awgQuick("up", confPath); err != nil {
		return fmt.Errorf("awg-quick up %s: %w (%s)", confPath, err, string(out))
	}
	clients[ci.Ifname] = clientState{fp: conf}
	return nil
}

// RemoveClient tears down a client interface (awg-quick down + rm conf) and
// drops it from the in-memory state map. Safe to call when the interface is
// already gone (idempotent).
func (m *Manager) RemoveClient(ifname string) error {
	clientMu.Lock()
	defer clientMu.Unlock()
	confPath := filepath.Join(awgConfigDir, ifname+".conf")
	if _, err := awgShowIfname(ifname); err == nil {
		if out, err := awgQuick("down", confPath); err != nil {
			return fmt.Errorf("awg-quick down %s: %w (%s)", confPath, err, string(out))
		}
	}
	_ = os.Remove(confPath)
	delete(clients, ifname)
	return nil
}

// StopAllClients tears down every tracked awgo-N client interface and clears
// the in-memory state map. Called before a module rebuild (RebuildAwgModule)
// so the script's rmmod can unload amneziawg — a live client interface keeps
// the module busy and the swap would silently leave the old module loaded.
// RestartAwg re-creates the outbounds from the database after the script.
func (m *Manager) StopAllClients() {
	clientMu.Lock()
	defer clientMu.Unlock()
	for ifname := range clients {
		confPath := filepath.Join(awgConfigDir, ifname+".conf")
		if _, err := awgShowIfname(ifname); err == nil {
			if out, err := awgQuick("down", confPath); err != nil {
				logger.Warningf("awg: stop client %s failed: %s %v", ifname, string(out), err)
			}
		}
	}
	clients = map[string]clientState{}
}

// SweepOrphanClients removes awgo-* interfaces and .conf files left over from
// a previous x-ui run that have no matching awg_outbounds row (or whose row is
// disabled). Runs once on first EnsureClient call (sync.Once) — not every tick.
// Uses a sync.Once separate from the server-side m.swept flag so the two
// sweeps stay independent.
func (m *Manager) sweepOrphanClientsOnce() {
	clientSwept.Do(func() {
		clientMu.Lock()
		defer clientMu.Unlock()
		entries, err := os.ReadDir(awgConfigDir)
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "awgo-") || !strings.HasSuffix(name, ".conf") {
				continue
			}
			ifname := strings.TrimSuffix(name, ".conf")
			if _, ok := clients[ifname]; ok {
				continue
			}
			if _, err := awgShowIfname(ifname); err != nil {
				_ = os.Remove(filepath.Join(awgConfigDir, name))
				continue
			}
			if out, err := awgQuick("down", filepath.Join(awgConfigDir, name)); err != nil {
				logger.Warningf("awg: orphan sweep down failed for %s: %s %v", ifname, string(out), err)
				continue
			}
			_ = os.Remove(filepath.Join(awgConfigDir, name))
		}
	})
}

// RunningClientIfnames returns sorted names of UP outbound awgo-N interfaces
// currently tracked by EnsureClient. Used by CollectHostStatus so the
// dashboard / Settings → Cores card counts both inbound awgN and outbound
// awgo-N (previously only inbounds — a host with only outbounds showed
// "Interfaces UP: 0" even when tunnels carried traffic).
func (m *Manager) RunningClientIfnames() []string {
	clientMu.Lock()
	defer clientMu.Unlock()
	names := make([]string, 0, len(clients))
	for ifname := range clients {
		if _, err := awgShowIfname(ifname); err != nil {
			continue
		}
		names = append(names, ifname)
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}

// CollectClientTraffic reads handshake age and rx/tx byte counters for one
// client interface via `awg show <iface> dump`. Returns ok=false if the
// interface is down or `awg` is unavailable; ok=true with zero counters when
// the interface is up but has no peers yet, or the peer has never completed a
// handshake. Mirrors scrapePeers but for the single peer on the client side.
//
// `awg show <iface> dump` output is one interface line followed by one
// tab-delimited line per peer, fields (matching scrapePeers / parseAwgDump):
//
//	[0]=pubkey  [1]=preshared-key  [2]=endpoint  [3]=allowed-ips
//	[4]=latest-handshake-epoch  [5]=rx  [6]=tx  [7]=keepalive
//
// The previous implementation parsed the plain (non-dump) `awg show` output
// as if it were tab-delimited, but the plain format is multi-section
// human-readable — the parser never matched, fell through to `return
// ..., true`, and so rx/tx/handshakeAge were always 0. That broke the UI
// status badge and the test endpoint's down-detection. The dump format
// fixes this; the field indices mirror parseAwgDump (handshake=4, rx=5,
// tx=6), which is the proven server-side parser.
func (m *Manager) CollectClientTraffic(ifname string) (handshakeAge time.Duration, rx, tx int64, ok bool) {
	out, err := awgShowIfname(ifname)
	if err != nil {
		return 0, 0, 0, false
	}
	return parseClientDump(string(out), time.Now())
}

// parseClientDump parses `awg show <iface> dump` output for a client
// interface. The first line is the interface row (private key, public key,
// listen port, jc/jmin/jmax/s1-s4/…); subsequent lines are peer rows in the
// same 7+ tab-delimited format as the server-side dump (parseAwgDump):
// fields = pubkey, psk, endpoint, allowed-ips, handshake-epoch, rx, tx, keepalive.
// A client interface has at most one peer (the upstream server), so the first
// parseable peer row wins. ok=true with zero counters when the interface
// exists but produced no parseable peer row (peer added but never connected).
// Extracted from CollectClientTraffic so the field-index mapping can be unit
// tested without shelling out to awg.
func parseClientDump(out string, now time.Time) (handshakeAge time.Duration, rx, tx int64, ok bool) {
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		return 0, 0, 0, true
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		hsEpoch, errHs := strconv.ParseInt(fields[4], 10, 64)
		rxVal, errRx := strconv.ParseInt(fields[5], 10, 64)
		txVal, errTx := strconv.ParseInt(fields[6], 10, 64)
		if errHs != nil || errRx != nil || errTx != nil {
			continue
		}
		if hsEpoch > 0 {
			handshakeAge = now.Sub(time.Unix(hsEpoch, 0))
		}
		return handshakeAge, rxVal, txVal, true
	}
	// Interface exists but produced no parseable peer line: treat as up
	// with zero counters (peer added but never connected).
	return 0, 0, 0, true
}
