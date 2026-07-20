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
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// awgShowIfname runs `awg show <ifname>` and returns the combined stdout+stderr.
// Used both for liveness probing (a non-nil error means the interface is down
// or absent) and for CollectClientTraffic's per-peer counter scrape. Kept here
// rather than in process.go to avoid touching the server-side helper surface.
func awgShowIfname(ifname string) ([]byte, error) {
	return exec.CommandContext(context.Background(), "awg", "show", ifname).CombinedOutput()
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
	m.sweepOrphanClientsOnce()
	clientMu.Lock()
	defer clientMu.Unlock()
	confPath := filepath.Join(awgConfigDir, ci.Ifname+".conf")
	newFP := ci.fingerprint()
	if st, ok := clients[ci.Ifname]; ok && st.fp == newFP {
		if _, err := awgShowIfname(ci.Ifname); err == nil {
			return nil
		}
	}
	if err := os.WriteFile(confPath, []byte(renderClientConf(ci)), 0o600); err != nil {
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
	clients[ci.Ifname] = clientState{fp: newFP}
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

// CollectClientTraffic reads handshake age and rx/tx byte counters for one
// client interface via `awg show <iface>`. Returns ok=false if the interface
// is down or the output is unreadable. Mirrors scrapePeers but for the single
// peer on the client side. The output is parsed one line per peer with
// tab-separated fields: pubkey, psk-status, rx, tx, handshake-epoch.
func (m *Manager) CollectClientTraffic(ifname string) (handshakeAge time.Duration, rx, tx int64, ok bool) {
	out, err := awgShowIfname(ifname)
	if err != nil {
		return 0, 0, 0, false
	}
	now := time.Now()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "interface") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		var h int64
		_, _ = fmt.Sscanf(fields[4], "%d", &h)
		if h > 0 {
			handshakeAge = now.Sub(time.Unix(0, h*int64(time.Second)))
		}
		_, _ = fmt.Sscanf(fields[2], "%d", &rx)
		_, _ = fmt.Sscanf(fields[3], "%d", &tx)
		return handshakeAge, rx, tx, true
	}
	return 0, 0, 0, true
}
