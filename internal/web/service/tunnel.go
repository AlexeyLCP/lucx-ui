// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"context"
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
	inst, err := s.naiveInstance(cfg)
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
		Probe:        mgr.StatusOf(inst),
		LastLog:      mgr.LastLog(tunnel.Naive),
	}, nil
}

// NaiveLogs returns the most recent output lines of the core process.
func (s *TunnelService) NaiveLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().Logs(tunnel.Naive, lines)
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
// installed core binary, returning the parser output on failure.
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
	cmd.Stdin = strings.NewReader(text)
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
func (s *TunnelService) DownloadBinary(downloadURL string) error {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return common.NewError("tunnel: empty download url")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return common.NewErrorf("tunnel: download failed: HTTP %d", resp.StatusCode)
	}
	dst := tunnel.Naive.BinaryPath()
	tmp := dst + ".download"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	n, err := io.Copy(file, io.LimitReader(resp.Body, maxTunnelBinaryDownload+1))
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
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	logger.Infof("tunnel: caddy binary downloaded to %s (%d bytes)", dst, n)
	return nil
}

// Reconcile converges every core toward its stored config; called by the
// cron job and after panel boot. A crashed core is revived; a disabled one
// stays down.
func (s *TunnelService) Reconcile() {
	s.reconcileNaiveInbounds()
	s.reconcileOlcrtcInbounds()
	s.reconcileQwdttInbound()
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
		cfg.Enabled = false
	}
	inst, err := s.naiveInstance(cfg)
	if err != nil {
		return
	}
	// Reconcile (not bare Ensure) so orphan naive-{id} keys are swept even
	// when no naive inbound is left.
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
		cfg.Enabled = false
	}
	inst, err := s.olcrtcInstance(cfg)
	if err != nil {
		return
	}
	// Reconcile (not bare Ensure) so orphan olcrtc-{id} keys are swept even
	// when no olcrtc inbound is left.
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
		cfg.Enabled = false
	}
	inst, err := s.qwdttInstance(cfg)
	if err != nil {
		return
	}
	if err := tunnel.GetManager().Ensure(inst); err != nil {
		logger.Warning("tunnel: qwdtt reconcile failed:", err)
	}
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
	inst, err := s.olcrtcInstance(cfg)
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
		Probe:        mgr.StatusOf(inst),
		LastLog:      mgr.LastLog(tunnel.Olcrtc),
	}, nil
}

// OlcrtcLogs returns the most recent output lines of the core process.
func (s *TunnelService) OlcrtcLogs(lines int) []string {
	if lines <= 0 {
		lines = 200
	}
	return tunnel.GetManager().Logs(tunnel.Olcrtc, lines)
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
func (s *TunnelService) DownloadOlcrtcBinary(downloadURL string) error {
	return s.downloadBinaryTo(tunnel.Olcrtc.BinaryPath(), downloadURL)
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

// downloadBinaryTo is shared by Naive and olcRTC binary downloads.
func (s *TunnelService) downloadBinaryTo(dst, downloadURL string) error {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return common.NewError("tunnel: empty download url")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{}).Do(req)
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
	n, err := io.Copy(file, io.LimitReader(resp.Body, maxTunnelBinaryDownload+1))
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
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	logger.Infof("tunnel: binary downloaded to %s (%d bytes)", dst, n)
	return nil
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
		case model.WireGuard, model.AWG, model.Hysteria:
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
func (s *TunnelService) DownloadQwdttBinary(downloadURL string) error {
	return s.downloadBinaryTo(tunnel.Qwdtt.BinaryPath(), downloadURL)
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
