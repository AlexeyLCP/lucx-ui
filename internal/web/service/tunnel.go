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

// tunnelNaiveSettingKey is the settings-table key holding the NaiveProxy
// core config as a JSON blob. The lucxTunnel_ prefix keeps tunnel configs
// clear of upstream setting keys.
const tunnelNaiveSettingKey = "lucxTunnel_naive"

// maxTunnelBinaryDownload caps the binary download (a caddy-naive build is
// ~50 MB; the cap is headroom, not a target).
const maxTunnelBinaryDownload = 200 << 20

// TunnelService manages the external tunnel-server sidecars (currently the
// NaiveProxy core): config persistence in the settings table, validation,
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
	return cfg.RenderCaddyfile(nil), nil
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

// Reconcile converges the core toward the stored config; called by the cron
// job and after panel boot. A crashed core is revived; a disabled one stays
// down.
func (s *TunnelService) Reconcile() {
	cfg, err := s.LoadNaiveConfig()
	if err != nil {
		logger.Warning("tunnel: reconcile load failed:", err)
		return
	}
	inst, err := s.naiveInstance(cfg)
	if err != nil {
		return
	}
	if err := tunnel.GetManager().Ensure(inst); err != nil {
		logger.Warning("tunnel: naive reconcile failed:", err)
	}
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
		ConfigText: cfg.RenderCaddyfile(extra),
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
