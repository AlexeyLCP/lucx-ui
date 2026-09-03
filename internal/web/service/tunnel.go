// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// tunnelNaiveSettingKey / tunnelOlcrtcSettingKey are settings-table keys
// holding each tunnel core config as a JSON blob. The lucxTunnel_ prefix
// keeps them clear of upstream setting keys.
const (
	tunnelNaiveSettingKey  = "lucxTunnel_naive"
	tunnelOlcrtcSettingKey = "lucxTunnel_olcrtc"
	tunnelQwdttSettingKey  = "lucxTunnel_qwdtt"
)

// maxTunnelBinaryDownload caps the binary download (a caddy-naive build is
// ~50 MB; the cap is headroom, not a target).
const maxTunnelBinaryDownload = 200 << 20

// maxTunnelDownloadRedirects bounds the redirect chain. Release assets on
// GitHub redirect once or twice to object storage; anything longer is either
// a loop or someone walking the fetcher somewhere it should not go.
const maxTunnelDownloadRedirects = 5

// TunnelService manages the external tunnel-server sidecars (NaiveProxy,
// olcRTC): config persistence in the settings table, validation,
// port-collision checks against Xray inbounds, per-client credentials, and
// lifecycle via tunnel.Manager. Zero-value usable, like the other services.
type TunnelService struct {
	inboundService InboundService
	clientService  ClientService
	settingService SettingService
}

// NaiveStatus is the full status payload for the NaiveProxy core: the live
// three-level probe, binary presence, the stored config and the generated
// client URL.
type NaiveStatus struct {
	Core         string             `json:"core"`
	DisplayName  string             `json:"displayName"`
	BinaryExists bool               `json:"binaryExists"`
	BinaryPath   string             `json:"binaryPath"`
	ClientURL    string             `json:"clientUrl"`
	Config       tunnel.NaiveConfig `json:"config"`
	Probe        tunnel.Status      `json:"probe"`
	LastLog      string             `json:"lastLog"`
}

// LoadNaiveConfig reads the stored NaiveProxy config, falling back to
// defaults when no row exists yet or the stored JSON is corrupt.
func (s *TunnelService) LoadNaiveConfig() (tunnel.NaiveConfig, error) {
	cfg := tunnel.DefaultNaiveConfig()
	setting := &model.Setting{}
	err := database.GetDB().Model(model.Setting{}).
		Where("key = ?", tunnelNaiveSettingKey).First(setting).Error
	if database.IsNotFound(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if strings.TrimSpace(setting.Value) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		logger.Warning("tunnel: corrupt naive config, using defaults:", err)
		return tunnel.DefaultNaiveConfig(), nil
	}
	return cfg.Merge(), nil
}

// SaveNaiveConfig validates, persists and applies the config. A running core
// whose rendered config changed is restarted by Manager.Ensure. needRestart
// is true when the hidden Xray SOCKS bridge must be added/moved/dropped
// (routeThroughXray toggle, port/outbound change, or enable flip while
// routed) — the bridge lives only in the generated Xray config.
func (s *TunnelService) SaveNaiveConfig(cfg tunnel.NaiveConfig) (needRestart bool, err error) {
	if err := s.legacyLifecycleBlocked(model.Naive, tunnelNaiveSettingKey); err != nil {
		return false, err
	}
	old, _ := s.LoadNaiveConfig()
	cfg = cfg.Merge()
	cfg, err = normalizeNaiveXrayPort(cfg, old)
	if err != nil {
		return false, err
	}
	if err := cfg.Validate(); err != nil {
		return false, err
	}
	if !cfg.UseRawConfig {
		if err := s.checkNaivePortConflict(cfg); err != nil {
			return false, err
		}
	}
	if err := s.persistNaive(cfg); err != nil {
		return false, err
	}
	if err := s.applyNaive(cfg); err != nil {
		return false, err
	}
	return naiveBridgeChanged(old, cfg), nil
}

// StartNaive marks the core enabled, persists, and starts it.
func (s *TunnelService) StartNaive() (needRestart bool, err error) {
	if err := s.legacyLifecycleBlocked(model.Naive, tunnelNaiveSettingKey); err != nil {
		return false, err
	}
	cfg, err := s.LoadNaiveConfig()
	if err != nil {
		return false, err
	}
	old := cfg
	cfg, err = normalizeNaiveXrayPort(cfg, old)
	if err != nil {
		return false, err
	}
	if err := cfg.Validate(); err != nil {
		return false, err
	}
	if !cfg.UseRawConfig {
		if err := s.checkNaivePortConflict(cfg); err != nil {
			return false, err
		}
	}
	cfg.Enabled = true
	if err := s.persistNaive(cfg); err != nil {
		return false, err
	}
	if err := s.applyNaive(cfg); err != nil {
		return false, err
	}
	return naiveBridgeChanged(old, cfg), nil
}

// StopNaive marks the core disabled, persists, and stops the process.
func (s *TunnelService) StopNaive() (needRestart bool, err error) {
	cfg, err := s.LoadNaiveConfig()
	if err != nil {
		return false, err
	}
	old := cfg
	cfg.Enabled = false
	if err := s.persistNaive(cfg); err != nil {
		return false, err
	}
	if err := tunnel.GetManager().Stop(tunnel.Naive); err != nil {
		return false, err
	}
	return naiveBridgeChanged(old, cfg), nil
}

// RestartNaive forces a fresh start with the stored config (and enables the
// core — a restart expresses the intent to run).
func (s *TunnelService) RestartNaive() (needRestart bool, err error) {
	if err := s.legacyLifecycleBlocked(model.Naive, tunnelNaiveSettingKey); err != nil {
		return false, err
	}
	cfg, err := s.LoadNaiveConfig()
	if err != nil {
		return false, err
	}
	old := cfg
	cfg, err = normalizeNaiveXrayPort(cfg, old)
	if err != nil {
		return false, err
	}
	if err := cfg.Validate(); err != nil {
		return false, err
	}
	cfg.Enabled = true
	if err := s.persistNaive(cfg); err != nil {
		return false, err
	}
	if err := tunnel.GetManager().Stop(tunnel.Naive); err != nil {
		logger.Warning("tunnel: restart stop failed:", err)
	}
	if err := s.applyNaive(cfg); err != nil {
		return false, err
	}
	return naiveBridgeChanged(old, cfg), nil
}

// normalizeNaiveXrayPort keeps the SOCKS bridge port stable across saves
// (mirrors normalizeMtprotoXrayPort). Raw mode and routing-off clear the
// port and outbound so a stale value never leaks into injectTunnelEgress.
func normalizeNaiveXrayPort(cfg, old tunnel.NaiveConfig) (tunnel.NaiveConfig, error) {
	if cfg.UseRawConfig {
		cfg.RouteThroughXray = false
		cfg.RouteXrayPort = 0
		cfg.OutboundTag = ""
		return cfg, nil
	}
	if !cfg.RouteThroughXray {
		cfg.RouteXrayPort = 0
		cfg.OutboundTag = ""
		return cfg, nil
	}
	if old.RouteXrayPort > 0 {
		cfg.RouteXrayPort = old.RouteXrayPort
		return cfg, nil
	}
	if cfg.RouteXrayPort > 0 {
		return cfg, nil
	}
	port, err := mtproto.FreeLocalPort()
	if err != nil {
		return cfg, common.NewError("tunnel: allocate SOCKS bridge port: ", err)
	}
	cfg.RouteXrayPort = port
	return cfg, nil
}

// naiveBridgeChanged reports whether the generated Xray SOCKS bridge must
// be regenerated: any change to (enabled∧routed), port, or outbound tag.
func naiveBridgeChanged(old, neo tunnel.NaiveConfig) bool {
	oldOn := old.Enabled && old.RouteThroughXray && old.RouteXrayPort > 0
	newOn := neo.Enabled && neo.RouteThroughXray && neo.RouteXrayPort > 0
	if oldOn != newOn {
		return true
	}
	if !newOn {
		return false
	}
	return old.RouteXrayPort != neo.RouteXrayPort ||
		strings.TrimSpace(old.OutboundTag) != strings.TrimSpace(neo.OutboundTag)
}

// NaiveStatus assembles the status payload from the stored config and the
// live manager state.
func (s *TunnelService) NaiveStatus() (NaiveStatus, error) {
	cfg, err := s.LoadNaiveConfig()
	if err != nil {
		return NaiveStatus{}, err
	}
	mgr := tunnel.GetManager()
	bin := tunnel.Naive.BinaryPath()
	info, statErr := os.Stat(bin)
	return NaiveStatus{
		Core:         string(tunnel.Naive),
		DisplayName:  tunnel.Naive.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		ClientURL:    cfg.ClientURL(),
		Config:       cfg,
		Probe:        tunnel.Status{Running: mgr.AnyRunning("naive-")},
		LastLog:      mgr.LastLogPrefixed("naive-"),
	}, nil
}

// NaiveLogs returns the most recent output lines of the core process.
func (s *TunnelService) NaiveLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().LogsPrefixed("naive-", lines)
}

// PreviewNaive renders the Caddyfile the given form state would produce,
// without persisting anything. Per-client credential lines are omitted from
// the preview (they depend on the live client list, not the form).
func (s *TunnelService) PreviewNaive(cfg tunnel.NaiveConfig) (string, error) {
	cfg = cfg.Merge()
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	return cfg.RenderCaddyfile(nil, ""), nil
}

// ValidateCaddyfile runs `caddy adapt` against the raw text using the
// installed core binary, returning the parser output on failure. It adapts the
// hardened form, which is what the server runs — the editor's text never is.
func (s *TunnelService) ValidateCaddyfile(text string) error {
	if strings.TrimSpace(text) == "" {
		return common.NewError("tunnel: Caddyfile is empty")
	}
	bin := tunnel.Naive.BinaryPath()
	if info, err := os.Stat(bin); err != nil || info.IsDir() {
		return common.NewError("tunnel: caddy binary not installed — upload or download it first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "adapt", "--adapter", "caddyfile", "--config", "-")
	cmd.Stdin = strings.NewReader(tunnel.HardenRawCaddyfile(text))
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return common.NewError("tunnel: caddy adapt: " + msg)
	}
	return nil
}

// DeleteBinary stops the core and removes its binary from disk.
func (s *TunnelService) DeleteBinary() error {
	if err := tunnel.GetManager().Stop(tunnel.Naive); err != nil {
		logger.Warning("tunnel: stop before binary delete failed:", err)
	}
	if err := os.Remove(tunnel.Naive.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DownloadBinary fetches the core binary from a URL into a temp file and
// swaps it into place (an interrupted download never leaves a half-written
// binary where the manager would exec it).
func (s *TunnelService) DownloadBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.Naive.BinaryPath(), downloadURL, wantSHA256)
}

// Reconcile converges every core toward its stored config; called by the
// cron job and after panel boot. A crashed core is revived; a disabled one
// stays down.
func (s *TunnelService) Reconcile() {
	s.reconcileNaiveInbounds()
	s.reconcileOlcrtcInbounds()
	s.reconcileQwdttInbound()
	s.reconcileMieruInbounds()
	s.reconcileTrustTunnelInbounds()
	s.reconcileAnytlsInbounds()
	s.reconcileTproxyInbounds()
}

// tunnelBlobMigrated reports whether the legacy settings blob carries the
// migratedToInbound marker written by the DB migrations
// (migrate_naive_inbound / migrate_tunnel_inbounds). A marked blob is a
// historical artifact: its config was promoted to an inbound, so inbounds
// are the source of truth for the core and the blob must never resurrect
// the legacy process (deleting the last inbound means "off", not "fall
// back to the old config" — VladufQa zombie-spam report).
func (s *TunnelService) tunnelBlobMigrated(settingKey string) bool {
	setting := &model.Setting{}
	err := database.GetDB().Model(model.Setting{}).
		Where("key = ?", settingKey).First(setting).Error
	if err != nil || strings.TrimSpace(setting.Value) == "" {
		return false
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(setting.Value), &raw) != nil {
		return false
	}
	v, ok := raw["migratedToInbound"]
	if !ok {
		return false
	}
	var b bool
	return json.Unmarshal(v, &b) == nil && b
}

// legacyLifecycleBlocked refuses the legacy Tunnels-page start/restart/save
// once inbounds took over the core: the blob was migrated or any inbound of
// the protocol exists. While an inbound exists the reconcile loop kills the
// legacy key every tick (Start looks broken), and after the last inbound is
// deleted an enabled legacy blob is resurrected as a zombie process that
// keeps running after the operator believes everything is gone.
func (s *TunnelService) legacyLifecycleBlocked(proto model.Protocol, settingKey string) error {
	if s.tunnelBlobMigrated(settingKey) {
		return common.NewErrorf("tunnel: %s config was migrated to an inbound — manage %s on the Inbounds page", proto, proto)
	}
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return err
	}
	for _, ib := range inbounds {
		if ib != nil && ib.Protocol == proto {
			return common.NewErrorf("tunnel: %s is managed by inbound %q — manage it on the Inbounds page", proto, ib.Remark)
		}
	}
	return nil
}

// reconcileNaiveInbounds Ensures every Naive inbound sidecar and stops orphans.
// When no protocol=naive inbound exists yet, falls back to the legacy global
// lucxTunnel_naive settings core (pre-migration hosts).
func (s *TunnelService) reconcileNaiveInbounds() {
	secret, _ := s.settingService.GetSecret()
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel: naive inbound list failed:", err)
		return
	}
	var want []tunnel.Instance
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Naive || ib.NodeID != nil {
			continue
		}
		inst, ok := tunnel.InstanceFromInbound(ib, secret)
		if !ok {
			continue
		}
		want = append(want, inst)
	}
	if len(want) > 0 {
		tunnel.GetManager().ReconcileNaive(want)
		// Stop legacy global key if inbounds took over.
		_ = tunnel.GetManager().Stop(tunnel.Naive)
		return
	}
	cfg, err := s.LoadNaiveConfig()
	if err != nil {
		logger.Warning("tunnel: naive reconcile load failed:", err)
		return
	}
	if s.tunnelBlobMigrated(tunnelNaiveSettingKey) {
		tunnel.GetManager().ReconcileNaive(nil)
		_ = tunnel.GetManager().Stop(tunnel.Naive)
		return
	}
	inst, err := s.naiveInstance(cfg)
	if err != nil {
		return
	}
	tunnel.GetManager().ReconcileNaive([]tunnel.Instance{inst})
}

func (s *TunnelService) reconcileOlcrtcInbounds() {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel: olcrtc inbound list failed:", err)
		return
	}
	var want []tunnel.Instance
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Olcrtc || ib.NodeID != nil {
			continue
		}
		inst, ok := tunnel.OlcrtcInstanceFromInbound(ib)
		if !ok {
			continue
		}
		want = append(want, inst)
	}
	if len(want) > 0 {
		tunnel.GetManager().ReconcileOlcrtc(want)
		_ = tunnel.GetManager().Stop(tunnel.Olcrtc)
		return
	}
	cfg, err := s.LoadOlcrtcConfig()
	if err != nil {
		logger.Warning("tunnel: olcrtc reconcile load failed:", err)
		return
	}
	if s.tunnelBlobMigrated(tunnelOlcrtcSettingKey) {
		tunnel.GetManager().ReconcileOlcrtc(nil)
		_ = tunnel.GetManager().Stop(tunnel.Olcrtc)
		return
	}
	inst, err := s.olcrtcInstance(cfg)
	if err != nil {
		return
	}
	tunnel.GetManager().ReconcileOlcrtc([]tunnel.Instance{inst})
}

func (s *TunnelService) reconcileQwdttInbound() {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel: qwdtt inbound list failed:", err)
		return
	}
	var one *model.Inbound
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Qwdtt || ib.NodeID != nil {
			continue
		}
		// Single-instance: prefer enabled; else first row.
		if one == nil || (ib.Enable && !one.Enable) {
			one = ib
		}
	}
	if one != nil {
		inst, ok := tunnel.QwdttInstanceFromInbound(one)
		if ok {
			if err := tunnel.GetManager().Ensure(inst); err != nil {
				logger.Warning("tunnel: qwdtt inbound reconcile failed:", err)
			} else if inst.RouteThroughXray {
				// Re-pin wdtt0→tunN after Xray restart / rule drop (same role as AWG ensureXrayRouting).
				tunnel.GetManager().EnsureQwdttRouting(inst)
			}
		}
		return
	}
	cfg, err := s.LoadQwdttConfig()
	if err != nil {
		logger.Warning("tunnel: qwdtt reconcile load failed:", err)
		return
	}
	if s.tunnelBlobMigrated(tunnelQwdttSettingKey) {
		_ = tunnel.GetManager().Stop(tunnel.Qwdtt)
		return
	}
	inst, err := s.qwdttInstance(cfg)
	if err != nil {
		return
	}
	if err := tunnel.GetManager().Ensure(inst); err != nil {
		logger.Warning("tunnel: qwdtt reconcile failed:", err)
	}
}

// reconcileMieruInbounds Ensures every mieru inbound sidecar and stops
// orphans. mieru is inbound-only (lucx.117+): there is no legacy settings
// blob, so with zero inbounds the wanted set is empty and every mieru-* key
// is swept.
func (s *TunnelService) reconcileMieruInbounds() {
	secret, _ := s.settingService.GetSecret()
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel: mieru inbound list failed:", err)
		return
	}
	var want []tunnel.Instance
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Mieru || ib.NodeID != nil {
			continue
		}
		inst, ok := tunnel.MieruInstanceFromInbound(ib, secret)
		if !ok {
			continue
		}
		want = append(want, inst)
	}
	tunnel.GetManager().ReconcileMieru(want)
}

// reconcileAnytlsInbounds Ensures every AnyTls inbound sidecar and stops
// orphans. Inbound-only like mieru: zero inbounds sweeps every anytls-* key.
func (s *TunnelService) reconcileAnytlsInbounds() {
	panelCert, panelKey := panelCertFiles()
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel: anytls inbound list failed:", err)
		return
	}
	var want []tunnel.Instance
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Anytls || ib.NodeID != nil {
			continue
		}
		inst, ok := tunnel.AnytlsInstanceFromInbound(ib, panelCert, panelKey)
		if !ok {
			continue
		}
		want = append(want, inst)
	}
	tunnel.GetManager().ReconcileAnytls(want)
}

func (s *TunnelService) reconcileTproxyInbounds() {
	panelCert, panelKey := panelCertFiles()
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel: tproxy inbound list failed:", err)
		return
	}
	var relays, mtps, caddies []tunnel.Instance
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Tproxy || ib.NodeID != nil {
			continue
		}
		insts, ok := tunnel.TproxyInstancesFromInbound(ib, panelCert, panelKey)
		if !ok {
			continue
		}
		for _, inst := range insts {
			switch inst.Core {
			case tunnel.Tproxy:
				relays = append(relays, inst)
			case tunnel.Mtproxy:
				mtps = append(mtps, inst)
			case tunnel.TproxyCaddy:
				caddies = append(caddies, inst)
			}
		}
		if ib.Enable {
			tunnel.EnsureMtproxyLocalOnly(ib.Id)
		} else {
			tunnel.ClearMtproxyLocalOnly(ib.Id)
		}
	}
	mgr := tunnel.GetManager()
	mgr.ReconcileMtproxy(mtps)
	mgr.ReconcileTproxy(relays)
	mgr.ReconcileTproxyCaddy(caddies)
}

// panelCertFiles reads the panel ACME certificate paths from settings (the
// TrustTunnel default cert source).
func panelCertFiles() (cert, key string) {
	db := database.GetDB()
	var rows []model.Setting
	if err := db.Where("key IN ?", []string{"webCertFile", "webKeyFile"}).Find(&rows).Error; err != nil {
		return "", ""
	}
	for _, r := range rows {
		switch r.Key {
		case "webCertFile":
			cert = strings.TrimSpace(r.Value)
		case "webKeyFile":
			key = strings.TrimSpace(r.Value)
		}
	}
	return cert, key
}

// reconcileTrustTunnelInbounds Ensures every TrustTunnel inbound sidecar and
// stops orphans. Inbound-only like mieru.
func (s *TunnelService) reconcileTrustTunnelInbounds() {
	secret, _ := s.settingService.GetSecret()
	panelCert, panelKey := panelCertFiles()
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel: trusttunnel inbound list failed:", err)
		return
	}
	var want []tunnel.Instance
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.TrustTunnel || ib.NodeID != nil {
			continue
		}
		inst, ok := tunnel.TrustTunnelInstanceFromInbound(ib, secret, panelCert, panelKey)
		if !ok {
			continue
		}
		want = append(want, inst)
	}
	tunnel.GetManager().ReconcileTrustTunnel(want)
}

// --- olcRTC core -----------------------------------------------------------

// OlcrtcStatus is the full status payload for the olcRTC core.
type OlcrtcStatus struct {
	Core         string              `json:"core"`
	DisplayName  string              `json:"displayName"`
	BinaryExists bool                `json:"binaryExists"`
	BinaryPath   string              `json:"binaryPath"`
	ClientURI    string              `json:"clientUri"`
	Config       tunnel.OlcrtcConfig `json:"config"`
	Probe        tunnel.Status       `json:"probe"`
	LastLog      string              `json:"lastLog"`
}

// LoadOlcrtcConfig reads the stored olcRTC config, falling back to defaults.
func (s *TunnelService) LoadOlcrtcConfig() (tunnel.OlcrtcConfig, error) {
	cfg := tunnel.DefaultOlcrtcConfig()
	setting := &model.Setting{}
	err := database.GetDB().Model(model.Setting{}).
		Where("key = ?", tunnelOlcrtcSettingKey).First(setting).Error
	if database.IsNotFound(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if strings.TrimSpace(setting.Value) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		logger.Warning("tunnel: corrupt olcrtc config, using defaults:", err)
		return tunnel.DefaultOlcrtcConfig(), nil
	}
	return cfg.Merge(), nil
}

// SaveOlcrtcConfig validates, persists and applies the config.
func (s *TunnelService) SaveOlcrtcConfig(cfg tunnel.OlcrtcConfig) error {
	if err := s.legacyLifecycleBlocked(model.Olcrtc, tunnelOlcrtcSettingKey); err != nil {
		return err
	}
	cfg = cfg.Merge().ClampVP8()
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg, err := cfg.EnsureCryptoKey()
	if err != nil {
		return err
	}
	if err := s.persistOlcrtc(cfg); err != nil {
		return err
	}
	return s.applyOlcrtc(cfg)
}

// StartOlcrtc marks the core enabled, persists, and starts it.
func (s *TunnelService) StartOlcrtc() error {
	if err := s.legacyLifecycleBlocked(model.Olcrtc, tunnelOlcrtcSettingKey); err != nil {
		return err
	}
	cfg, err := s.LoadOlcrtcConfig()
	if err != nil {
		return err
	}
	cfg = cfg.Merge().ClampVP8()
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg, err = cfg.EnsureCryptoKey()
	if err != nil {
		return err
	}
	cfg.Enabled = true
	if err := s.persistOlcrtc(cfg); err != nil {
		return err
	}
	return s.applyOlcrtc(cfg)
}

// StopOlcrtc marks the core disabled, persists, and stops the process.
func (s *TunnelService) StopOlcrtc() error {
	cfg, err := s.LoadOlcrtcConfig()
	if err != nil {
		return err
	}
	cfg.Enabled = false
	if err := s.persistOlcrtc(cfg); err != nil {
		return err
	}
	return tunnel.GetManager().Stop(tunnel.Olcrtc)
}

// RestartOlcrtc forces a fresh start with the stored config.
func (s *TunnelService) RestartOlcrtc() error {
	if err := s.legacyLifecycleBlocked(model.Olcrtc, tunnelOlcrtcSettingKey); err != nil {
		return err
	}
	cfg, err := s.LoadOlcrtcConfig()
	if err != nil {
		return err
	}
	cfg = cfg.Merge().ClampVP8()
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg, err = cfg.EnsureCryptoKey()
	if err != nil {
		return err
	}
	cfg.Enabled = true
	if err := s.persistOlcrtc(cfg); err != nil {
		return err
	}
	if err := tunnel.GetManager().Stop(tunnel.Olcrtc); err != nil {
		logger.Warning("tunnel: olcrtc restart stop failed:", err)
	}
	return s.applyOlcrtc(cfg)
}

// OlcrtcStatus assembles the status payload from the stored config and the
// live manager state.
func (s *TunnelService) OlcrtcStatus() (OlcrtcStatus, error) {
	cfg, err := s.LoadOlcrtcConfig()
	if err != nil {
		return OlcrtcStatus{}, err
	}
	mgr := tunnel.GetManager()
	bin := tunnel.Olcrtc.BinaryPath()
	info, statErr := os.Stat(bin)
	return OlcrtcStatus{
		Core:         string(tunnel.Olcrtc),
		DisplayName:  tunnel.Olcrtc.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		ClientURI:    cfg.ClientURI(),
		Config:       cfg,
		Probe:        tunnel.Status{Running: mgr.AnyRunning("olcrtc-")},
		LastLog:      mgr.LastLogPrefixed("olcrtc-"),
	}, nil
}

// OlcrtcLogs returns the most recent output lines of the core process.
func (s *TunnelService) OlcrtcLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().LogsPrefixed("olcrtc-", lines)
}

// PreviewOlcrtc renders the YAML the given form state would produce.
func (s *TunnelService) PreviewOlcrtc(cfg tunnel.OlcrtcConfig) (string, error) {
	cfg = cfg.Merge().ClampVP8()
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	// Preview may lack a key — render with a placeholder so the operator
	// sees the shape; the real key is generated on save.
	if strings.TrimSpace(cfg.CryptoKey) == "" {
		cfg.CryptoKey = strings.Repeat("0", 64)
	}
	return cfg.RenderYAML(tunnel.DataDir(tunnel.Olcrtc)), nil
}

// DeleteOlcrtcBinary stops the core and removes its binary from disk.
func (s *TunnelService) DeleteOlcrtcBinary() error {
	if err := tunnel.GetManager().Stop(tunnel.Olcrtc); err != nil {
		logger.Warning("tunnel: stop before olcrtc binary delete failed:", err)
	}
	if err := os.Remove(tunnel.Olcrtc.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DownloadOlcrtcBinary fetches the core binary from a URL into place.
func (s *TunnelService) DownloadOlcrtcBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.Olcrtc.BinaryPath(), downloadURL, wantSHA256)
}

func (s *TunnelService) persistOlcrtc(cfg tunnel.OlcrtcConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.saveSetting(tunnelOlcrtcSettingKey, string(raw))
}

func (s *TunnelService) applyOlcrtc(cfg tunnel.OlcrtcConfig) error {
	inst, err := s.olcrtcInstance(cfg)
	if err != nil {
		return err
	}
	return tunnel.GetManager().Ensure(inst)
}

func (s *TunnelService) olcrtcInstance(cfg tunnel.OlcrtcConfig) (tunnel.Instance, error) {
	// ProbePort 0: olcrtc has no local listen port (outbound WebRTC only).
	return tunnel.Instance{
		Core:       tunnel.Olcrtc,
		Enabled:    cfg.Enabled,
		ConfigText: cfg.RenderYAML(tunnel.DataDir(tunnel.Olcrtc)),
		ProbePort:  0,
	}, nil
}

// downloadBinaryTo fetches a core binary into dst. It is the single download
// path for all five cores.
//
// The fetched bytes become an executable the panel runs as root, and the URL
// comes straight from an operator-supplied form field, so the fetch is
// constrained rather than trusted:
//
//   - https only — plaintext http would let anyone on the path swap the binary;
//   - every hop (the URL and each redirect target) must resolve to a public
//     unicast address, so the panel cannot be pointed at 127.0.0.1, the LAN or
//     a cloud metadata endpoint and used as a request proxy;
//   - the redirect chain is bounded;
//   - the response is size-capped and hashed, and when the caller supplied an
//     expected SHA-256 a mismatch discards the download before it can replace
//     the running binary.
//
// The digest of what landed is logged either way, so an operator who did not
// pass one can still compare against the release checksums after the fact.
func (s *TunnelService) downloadBinaryTo(dst, downloadURL, wantSHA256 string) error {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return common.NewError("tunnel: empty download url")
	}
	wantSHA256 = strings.ToLower(strings.TrimSpace(wantSHA256))
	if wantSHA256 == "" || !isSHA256Hex(wantSHA256) {
		return common.NewError("tunnel: sha256 must be 64 hex characters")
	}
	if err := checkDownloadURL(downloadURL); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: publicOnlyDialContext,
		},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= maxTunnelDownloadRedirects {
				return common.NewErrorf("tunnel: too many redirects (>%d)", maxTunnelDownloadRedirects)
			}
			return checkDownloadURL(r.URL.String())
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return common.NewErrorf("tunnel: download failed: HTTP %d", resp.StatusCode)
	}

	tmp := dst + ".download"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	digest := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, digest), io.LimitReader(resp.Body, maxTunnelBinaryDownload+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if n > maxTunnelBinaryDownload {
		_ = os.Remove(tmp)
		return common.NewError("tunnel: download exceeds the size limit")
	}
	got := hex.EncodeToString(digest.Sum(nil))
	if wantSHA256 != "" && got != wantSHA256 {
		_ = os.Remove(tmp)
		return common.NewErrorf("tunnel: sha256 mismatch: expected %s, got %s", wantSHA256, got)
	}

	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	logger.Infof("tunnel: binary downloaded to %s (%d bytes, sha256 %s)", dst, n, got)
	return nil
}

// isSHA256Hex reports whether v is exactly 64 lowercase hex characters.
func isSHA256Hex(v string) bool {
	if len(v) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(v)
	return err == nil && len(decoded) == sha256.Size
}

// checkDownloadURL rejects a binary-download target that is not plain https to
// a public host. Called for the operator's URL and again for every redirect
// hop, because a permissive first hop is worthless if hop two lands on
// 169.254.169.254.
func checkDownloadURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return common.NewErrorf("tunnel: invalid download url: %v", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return common.NewError("tunnel: download url must use https")
	}
	host := parsed.Hostname()
	if host == "" {
		return common.NewError("tunnel: download url has no host")
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return common.NewErrorf("tunnel: cannot resolve %s: %v", host, err)
	}
	if len(addrs) == 0 {
		return common.NewErrorf("tunnel: %s resolves to no address", host)
	}
	for _, addr := range addrs {
		if !isPublicUnicast(addr.IP) {
			return common.NewErrorf("tunnel: %s resolves to a non-public address (%s)", host, addr.IP)
		}
	}
	return nil
}

// isPublicUnicast reports whether ip is a routable public address — anything
// loopback, link-local (which covers the 169.254.169.254 metadata service),
// private, multicast or unspecified is refused.
func publicOnlyDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicUnicast(ip) {
			return nil, common.NewErrorf("tunnel: refused non-public address %s", host)
		}
		return dialer.DialContext(ctx, network, addr)
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var last error
	for _, a := range addrs {
		if !isPublicUnicast(a.IP) {
			last = common.NewErrorf("tunnel: %s resolved to a non-public address", host)
			continue
		}
		c, err := dialer.DialContext(ctx, network, net.JoinHostPort(a.IP.String(), port))
		if err == nil {
			return c, nil
		}
		last = err
	}
	if last == nil {
		last = common.NewErrorf("tunnel: no public address for %s", host)
	}
	return nil, last
}

func isPublicUnicast(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	// Carrier-grade NAT (100.64.0.0/10) is not covered by IsPrivate but is
	// just as much "someone else's network" from the panel's point of view.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return false
	}
	return true
}

func (s *TunnelService) persistNaive(cfg tunnel.NaiveConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.saveSetting(tunnelNaiveSettingKey, string(raw))
}

func (s *TunnelService) applyNaive(cfg tunnel.NaiveConfig) error {
	inst, err := s.naiveInstance(cfg)
	if err != nil {
		return err
	}
	return tunnel.GetManager().Ensure(inst)
}

func (s *TunnelService) saveSetting(key, value string) error {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	setting.Value = value
	return db.Save(setting).Error
}

// naiveInstance builds the sidecar desired state from the operator config.
// Raw mode keeps whatever port the form carries for probing (0 disables the
// TCP/TLS probes — the panel cannot parse an arbitrary Caddyfile) and skips
// per-client credentials (the operator owns the whole Caddyfile there).
func (s *TunnelService) naiveInstance(cfg tunnel.NaiveConfig) (tunnel.Instance, error) {
	var extra []tunnel.AuthPair
	if !cfg.UseRawConfig {
		extra = s.naiveClientAuths()
	}
	return tunnel.Instance{
		Core:       tunnel.Naive,
		Enabled:    cfg.Enabled,
		ConfigText: cfg.RenderCaddyfile(extra, tunnel.AccessLogPath(string(tunnel.Naive))),
		ExtraArgs:  cfg.ExtraArgs,
		ProbePort:  cfg.Port,
	}, nil
}

// naiveClientAuths derives one basic_auth pair per enabled panel client so
// each client gets personal NaiveProxy credentials (mirrored by the
// subscription links). Deterministic from the panel secret — no storage.
// A client add/remove/enable flip changes the rendered Caddyfile, so the
// reconcile loop restarts caddy to apply it.
func (s *TunnelService) naiveClientAuths() []tunnel.AuthPair {
	secret, err := s.settingService.GetSecret()
	if err != nil || len(secret) == 0 {
		return nil
	}
	clients, err := s.clientService.List()
	if err != nil {
		logger.Warning("tunnel: list clients for naive auth failed:", err)
		return nil
	}
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		if c.Enable && strings.TrimSpace(c.Email) != "" {
			emails = append(emails, c.Email)
		}
	}
	sort.Strings(emails)
	pairs := make([]tunnel.AuthPair, 0, len(emails))
	for _, email := range emails {
		pairs = append(pairs, tunnel.ClientAuth(secret, email))
	}
	return pairs
}

// NaiveSubLinks returns the personal naive+https share link for each of the
// given client emails, or nil when the core is disabled / in raw mode / has
// no domain. Consumed by the subscription builder so naive joins the client's
// other protocol links. Standard `naive+https://user:pass@host:port#remark`
// format understood by NekoBox / husi / Exclave.
func (s *TunnelService) NaiveSubLinks(emails []string) []string {
	cfg, err := s.LoadNaiveConfig()
	if err != nil || !cfg.Enabled || cfg.UseRawConfig {
		return nil
	}
	domain := strings.TrimSpace(cfg.Domain)
	if domain == "" {
		return nil
	}
	secret, err := s.settingService.GetSecret()
	if err != nil || len(secret) == 0 {
		return nil
	}
	host := domain
	if cfg.Port != 443 {
		host = net.JoinHostPort(domain, strconv.Itoa(cfg.Port))
	}
	out := make([]string, 0, len(emails))
	for _, email := range emails {
		if strings.TrimSpace(email) == "" {
			continue
		}
		pair := tunnel.ClientAuth(secret, email)
		u := url.URL{
			Scheme:   "https",
			User:     url.UserPassword(pair.User, pair.Pass),
			Host:     host,
			Fragment: email,
		}
		out = append(out, "naive+"+u.String())
	}
	return out
}

// checkNaivePortConflict refuses a port already bound by a TCP Xray inbound
// on an overlapping interface. UDP-only protocols (wireguard, AWG) never
// collide with the naive TCP listener. Mirrors the two-direction philosophy
// of elector1337's cross-check; the inbound side needs no hook because
// Xray restarts loudly on a bind failure.
func (s *TunnelService) checkNaivePortConflict(cfg tunnel.NaiveConfig) error {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return err
	}
	for _, ib := range inbounds {
		if ib == nil || ib.Port != cfg.Port {
			continue
		}
		switch ib.Protocol {
		case model.WireGuard, model.AWG, model.AmneziaWG, model.Hysteria:
			continue // UDP-only listeners never collide with naive's TCP port
		}
		if tunnelListenOverlap(cfg.Listen, ib.Listen) {
			return common.NewErrorf("tunnel: port %d is already used by inbound %q",
				cfg.Port, ib.Remark)
		}
	}
	return nil
}

func tunnelListenOverlap(a, b string) bool {
	wild := func(s string) bool {
		s = strings.TrimSpace(s)
		return s == "" || s == "0.0.0.0" || s == "::"
	}
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	return wild(a) || wild(b) || a == b
}

// --- qWDTT core ------------------------------------------------------------

// QwdttStatus is the full status payload for the qWDTT core.
type QwdttStatus struct {
	Core         string             `json:"core"`
	DisplayName  string             `json:"displayName"`
	BinaryExists bool               `json:"binaryExists"`
	BinaryPath   string             `json:"binaryPath"`
	ClientURI    string             `json:"clientUri"`
	LegacyURI    string             `json:"legacyUri"`
	SubJSON      string             `json:"subJson"`
	Config       tunnel.QwdttConfig `json:"config"`
	Probe        tunnel.Status      `json:"probe"`
	LastLog      string             `json:"lastLog"`
}

// LoadQwdttConfig reads the stored qWDTT config, falling back to defaults.
func (s *TunnelService) LoadQwdttConfig() (tunnel.QwdttConfig, error) {
	cfg := tunnel.DefaultQwdttConfig()
	setting := &model.Setting{}
	err := database.GetDB().Model(model.Setting{}).
		Where("key = ?", tunnelQwdttSettingKey).First(setting).Error
	if database.IsNotFound(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if strings.TrimSpace(setting.Value) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(setting.Value), &cfg); err != nil {
		logger.Warning("tunnel: corrupt qwdtt config, using defaults:", err)
		return tunnel.DefaultQwdttConfig(), nil
	}
	return cfg.Merge(), nil
}

// SaveQwdttConfig validates, persists and applies the config.
func (s *TunnelService) SaveQwdttConfig(cfg tunnel.QwdttConfig) error {
	if err := s.legacyLifecycleBlocked(model.Qwdtt, tunnelQwdttSettingKey); err != nil {
		return err
	}
	cfg = cfg.Merge()
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg, err := cfg.EnsurePassword()
	if err != nil {
		return err
	}
	if err := s.persistQwdtt(cfg); err != nil {
		return err
	}
	return s.applyQwdtt(cfg)
}

// StartQwdtt marks the core enabled, persists, and starts it.
func (s *TunnelService) StartQwdtt() error {
	if err := s.legacyLifecycleBlocked(model.Qwdtt, tunnelQwdttSettingKey); err != nil {
		return err
	}
	cfg, err := s.LoadQwdttConfig()
	if err != nil {
		return err
	}
	cfg = cfg.Merge()
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg, err = cfg.EnsurePassword()
	if err != nil {
		return err
	}
	cfg.Enabled = true
	if err := s.persistQwdtt(cfg); err != nil {
		return err
	}
	return s.applyQwdtt(cfg)
}

// StopQwdtt marks the core disabled, persists, and stops the process.
func (s *TunnelService) StopQwdtt() error {
	cfg, err := s.LoadQwdttConfig()
	if err != nil {
		return err
	}
	cfg.Enabled = false
	if err := s.persistQwdtt(cfg); err != nil {
		return err
	}
	return tunnel.GetManager().Stop(tunnel.Qwdtt)
}

// RestartQwdtt forces a fresh start with the stored config.
func (s *TunnelService) RestartQwdtt() error {
	if err := s.legacyLifecycleBlocked(model.Qwdtt, tunnelQwdttSettingKey); err != nil {
		return err
	}
	cfg, err := s.LoadQwdttConfig()
	if err != nil {
		return err
	}
	cfg = cfg.Merge()
	if err := cfg.Validate(); err != nil {
		return err
	}
	cfg, err = cfg.EnsurePassword()
	if err != nil {
		return err
	}
	cfg.Enabled = true
	if err := s.persistQwdtt(cfg); err != nil {
		return err
	}
	if err := tunnel.GetManager().Stop(tunnel.Qwdtt); err != nil {
		logger.Warning("tunnel: qwdtt restart stop failed:", err)
	}
	return s.applyQwdtt(cfg)
}

// QwdttStatus assembles the status payload.
func (s *TunnelService) QwdttStatus() (QwdttStatus, error) {
	cfg, err := s.LoadQwdttConfig()
	if err != nil {
		return QwdttStatus{}, err
	}
	inst, err := s.qwdttInstance(cfg)
	if err != nil {
		return QwdttStatus{}, err
	}
	mgr := tunnel.GetManager()
	bin := tunnel.Qwdtt.BinaryPath()
	info, statErr := os.Stat(bin)
	subJSON, _ := cfg.SubscriptionJSON()
	return QwdttStatus{
		Core:         string(tunnel.Qwdtt),
		DisplayName:  tunnel.Qwdtt.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		ClientURI:    cfg.ClientURI(),
		LegacyURI:    cfg.LegacyURI(),
		SubJSON:      subJSON,
		Config:       cfg,
		Probe:        mgr.StatusOf(inst),
		LastLog:      mgr.LastLog(tunnel.Qwdtt),
	}, nil
}

// QwdttLogs returns the most recent output lines of the core process.
func (s *TunnelService) QwdttLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().Logs(tunnel.Qwdtt, lines)
}

// DeleteQwdttBinary stops the core and removes its binary from disk.
func (s *TunnelService) DeleteQwdttBinary() error {
	if err := tunnel.GetManager().Stop(tunnel.Qwdtt); err != nil {
		logger.Warning("tunnel: stop before qwdtt binary delete failed:", err)
	}
	if err := os.Remove(tunnel.Qwdtt.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DownloadQwdttBinary fetches the core binary from a URL into place.
func (s *TunnelService) DownloadQwdttBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.Qwdtt.BinaryPath(), downloadURL, wantSHA256)
}

func (s *TunnelService) persistQwdtt(cfg tunnel.QwdttConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.saveSetting(tunnelQwdttSettingKey, string(raw))
}

func (s *TunnelService) applyQwdtt(cfg tunnel.QwdttConfig) error {
	inst, err := s.qwdttInstance(cfg)
	if err != nil {
		return err
	}
	return tunnel.GetManager().Ensure(inst)
}

func (s *TunnelService) qwdttInstance(cfg tunnel.QwdttConfig) (tunnel.Instance, error) {
	// ProbePort 0: DTLS is UDP; process-alive is the only reliable signal.
	return tunnel.Instance{
		Core:    tunnel.Qwdtt,
		Enabled: cfg.Enabled,
		Args:    cfg.BuildArgs(),
	}, nil
}

// --- mieru core (inbound-only, lucx.117) -----------------------------------

// MieruStatus is the Cores-page payload of the mieru core: binary presence
// plus the aggregate process state across all mieru-{id} instances.
type MieruStatus struct {
	Core         string        `json:"core"`
	DisplayName  string        `json:"displayName"`
	BinaryExists bool          `json:"binaryExists"`
	BinaryPath   string        `json:"binaryPath"`
	Probe        tunnel.Status `json:"probe"`
	LastLog      string        `json:"lastLog"`
}

// MieruStatus assembles the Cores-page payload from the live manager state.
func (s *TunnelService) MieruStatus() (MieruStatus, error) {
	mgr := tunnel.GetManager()
	bin := tunnel.Mieru.BinaryPath()
	info, statErr := os.Stat(bin)
	return MieruStatus{
		Core:         string(tunnel.Mieru),
		DisplayName:  tunnel.Mieru.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		Probe:        tunnel.Status{Running: mgr.AnyRunning("mieru-")},
		LastLog:      mgr.LastLogPrefixed("mieru-"),
	}, nil
}

// MieruLogs returns the most recent output lines of all mieru instances.
func (s *TunnelService) MieruLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().LogsPrefixed("mieru-", lines)
}

// DeleteMieruBinary stops every mieru instance and removes the binary.
func (s *TunnelService) DeleteMieruBinary() error {
	tunnel.GetManager().StopPrefixed("mieru-")
	if err := os.Remove(tunnel.Mieru.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DownloadMieruBinary fetches the mita binary from a URL into place.
func (s *TunnelService) DownloadMieruBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.Mieru.BinaryPath(), downloadURL, wantSHA256)
}

// --- TrustTunnel core (inbound-only, lucx.117) -----------------------------

// TrustTunnelStatus is the Cores-page payload of the TrustTunnel core.
type TrustTunnelStatus struct {
	Core         string        `json:"core"`
	DisplayName  string        `json:"displayName"`
	BinaryExists bool          `json:"binaryExists"`
	BinaryPath   string        `json:"binaryPath"`
	Probe        tunnel.Status `json:"probe"`
	LastLog      string        `json:"lastLog"`
}

// TrustTunnelStatus assembles the Cores-page payload from the live manager.
func (s *TunnelService) TrustTunnelStatus() (TrustTunnelStatus, error) {
	mgr := tunnel.GetManager()
	bin := tunnel.TrustTunnel.BinaryPath()
	info, statErr := os.Stat(bin)
	return TrustTunnelStatus{
		Core:         string(tunnel.TrustTunnel),
		DisplayName:  tunnel.TrustTunnel.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		Probe:        tunnel.Status{Running: mgr.AnyRunning("trusttunnel-")},
		LastLog:      mgr.LastLogPrefixed("trusttunnel-"),
	}, nil
}

// TrustTunnelLogs returns the most recent output lines of all instances.
func (s *TunnelService) TrustTunnelLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().LogsPrefixed("trusttunnel-", lines)
}

// DeleteTrustTunnelBinary stops every instance and removes the binary.
func (s *TunnelService) DeleteTrustTunnelBinary() error {
	tunnel.GetManager().StopPrefixed("trusttunnel-")
	if err := os.Remove(tunnel.TrustTunnel.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DownloadTrustTunnelBinary fetches the endpoint binary from a URL into place.
func (s *TunnelService) DownloadTrustTunnelBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.TrustTunnel.BinaryPath(), downloadURL, wantSHA256)
}

// --- AnyTLS core (inbound-only) ---------------------------------------------

// AnytlsStatus is the Cores-page payload of the AnyTLS core: binary presence
// plus the aggregate process state across all anytls-{id} instances.
type AnytlsStatus struct {
	Core         string        `json:"core"`
	DisplayName  string        `json:"displayName"`
	BinaryExists bool          `json:"binaryExists"`
	BinaryPath   string        `json:"binaryPath"`
	Probe        tunnel.Status `json:"probe"`
	LastLog      string        `json:"lastLog"`
}

// AnytlsStatus assembles the Cores-page payload from the live manager state.
func (s *TunnelService) AnytlsStatus() (AnytlsStatus, error) {
	mgr := tunnel.GetManager()
	bin := tunnel.Anytls.BinaryPath()
	info, statErr := os.Stat(bin)
	return AnytlsStatus{
		Core:         string(tunnel.Anytls),
		DisplayName:  tunnel.Anytls.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		Probe:        tunnel.Status{Running: mgr.AnyRunning("anytls-")},
		LastLog:      mgr.LastLogPrefixed("anytls-"),
	}, nil
}

// AnytlsLogs returns the most recent output lines of all anytls instances.
func (s *TunnelService) AnytlsLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().LogsPrefixed("anytls-", lines)
}

// DeleteAnytlsBinary stops every anytls instance and removes the binary.
func (s *TunnelService) DeleteAnytlsBinary() error {
	tunnel.GetManager().StopPrefixed("anytls-")
	if err := os.Remove(tunnel.Anytls.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DownloadAnytlsBinary fetches the anytls-server binary from a URL into place.
func (s *TunnelService) DownloadAnytlsBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.Anytls.BinaryPath(), downloadURL, wantSHA256)
}

type TproxyStatus struct {
	Core         string        `json:"core"`
	DisplayName  string        `json:"displayName"`
	BinaryExists bool          `json:"binaryExists"`
	BinaryPath   string        `json:"binaryPath"`
	Probe        tunnel.Status `json:"probe"`
	LastLog      string        `json:"lastLog"`
}

func (s *TunnelService) TproxyStatus() (TproxyStatus, error) {
	mgr := tunnel.GetManager()
	bin := tunnel.Tproxy.BinaryPath()
	info, statErr := os.Stat(bin)
	return TproxyStatus{
		Core:         string(tunnel.Tproxy),
		DisplayName:  tunnel.Tproxy.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		Probe:        tunnel.Status{Running: mgr.AnyRunning("tproxy-")},
		LastLog:      mgr.LastLogPrefixed("tproxy-"),
	}, nil
}

func (s *TunnelService) TproxyLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().LogsPrefixed("tproxy-", lines)
}

func (s *TunnelService) DeleteTproxyBinary() error {
	tunnel.GetManager().StopPrefixed("tproxy-")
	tunnel.GetManager().StopPrefixed("tproxycaddy-")
	if err := os.Remove(tunnel.Tproxy.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *TunnelService) DownloadTproxyBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.Tproxy.BinaryPath(), downloadURL, wantSHA256)
}

func (s *TunnelService) MtproxyStatus() (TproxyStatus, error) {
	mgr := tunnel.GetManager()
	bin := tunnel.Mtproxy.BinaryPath()
	info, statErr := os.Stat(bin)
	return TproxyStatus{
		Core:         string(tunnel.Mtproxy),
		DisplayName:  tunnel.Mtproxy.DisplayName(),
		BinaryExists: statErr == nil && !info.IsDir(),
		BinaryPath:   bin,
		Probe:        tunnel.Status{Running: mgr.AnyRunning("mtproxy-")},
		LastLog:      mgr.LastLogPrefixed("mtproxy-"),
	}, nil
}

func (s *TunnelService) MtproxyLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().LogsPrefixed("mtproxy-", lines)
}

func (s *TunnelService) DeleteMtproxyBinary() error {
	tunnel.GetManager().StopPrefixed("mtproxy-")
	if err := os.Remove(tunnel.Mtproxy.BinaryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *TunnelService) DownloadMtproxyBinary(downloadURL, wantSHA256 string) error {
	return s.downloadBinaryTo(tunnel.Mtproxy.BinaryPath(), downloadURL, wantSHA256)
}

func (s *TunnelService) UploadTproxySite(inboundID int, zipBytes []byte) error {
	ib, err := s.inboundService.GetInbound(inboundID)
	if err != nil {
		return err
	}
	if ib == nil || ib.Protocol != model.Tproxy {
		return common.NewError("inbound is not tproxy")
	}
	if err := tunnel.ExtractTproxySiteZip(tunnel.TproxySiteDir(inboundID), zipBytes); err != nil {
		return err
	}
	s.reconcileTproxyInbounds()
	return nil
}
