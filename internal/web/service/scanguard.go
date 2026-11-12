// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/scanguard"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

const (
	scanguardEnabledKey     = "scanguardEnabled"
	scanguardSourcesKey     = "scanguardSources"
	scanguardWhitelistKey   = "scanguardWhitelist"
	scanguardLastUpdatedKey = "scanguardLastUpdated"
	scanguardLastErrorKey   = "scanguardLastError"
)

// how often the reconcile job is allowed to re-fetch the lists; rules are
// re-enforced sooner when they are found missing (reboot / iptables flush).
const scanguardRefreshInterval = 6 * time.Hour

// ScanguardService persists the scanner-guard config (settings table keys)
// and drives the fetch/ipset cycle shared by the controller and the cron job.
type ScanguardService struct {
	mu sync.Mutex
}

// ScanguardStatus is the status payload: stored config + live firewall probe.
type ScanguardStatus struct {
	Enabled     bool                   `json:"enabled"`
	Sources     []string               `json:"sources"`
	Whitelist   []string               `json:"whitelist"`
	LastUpdated string                 `json:"lastUpdated"`
	LastError   string                 `json:"lastError"`
	Firewall    scanguard.ProbedStatus `json:"firewall"`
}

// LoadConfig reads the stored config, falling back to defaults (stock source,
// disabled) when keys are missing or corrupt.
func (s *ScanguardService) LoadConfig() scanguard.Config {
	cfg := scanguard.DefaultConfig()
	cfg.Enabled = false
	if v, ok, err := scanguardSetting(scanguardEnabledKey); err == nil && ok {
		cfg.Enabled = v == "true"
	}
	if v, ok, err := scanguardSetting(scanguardSourcesKey); err == nil && ok {
		var srcs []string
		if jsonErr := json.Unmarshal([]byte(v), &srcs); jsonErr == nil && len(srcs) > 0 {
			cfg.Sources = srcs
		}
	}
	if v, ok, err := scanguardSetting(scanguardWhitelistKey); err == nil && ok {
		var wl []string
		if jsonErr := json.Unmarshal([]byte(v), &wl); jsonErr == nil {
			cfg.Whitelist = wl
		}
	}
	if cfg.Whitelist == nil {
		cfg.Whitelist = []string{}
	}
	return cfg
}

// Status returns the merged view for the UI.
func (s *ScanguardService) Status() ScanguardStatus {
	cfg := s.LoadConfig()
	ports := s.keepPorts()
	st := ScanguardStatus{
		Enabled:   cfg.Enabled,
		Sources:   cfg.Sources,
		Whitelist: cfg.Whitelist,
		Firewall:  scanguard.Status(ports),
	}
	if v, ok, err := scanguardSetting(scanguardLastUpdatedKey); err == nil && ok {
		if unix, parseErr := strconv.ParseInt(strings.TrimSpace(v), 10, 64); parseErr == nil && unix > 0 {
			st.LastUpdated = time.Unix(unix, 0).Format(time.RFC3339)
		}
	}
	if v, ok, err := scanguardSetting(scanguardLastErrorKey); err == nil && ok {
		st.LastError = v
	}
	return st
}

// SaveAndApply persists the config and brings the firewall to match it.
// Applying is atomic (ipset swap) and a source failure keeps the previous
// sets — the panel never drops protection because of a broken mirror.
func (s *ScanguardService) SaveAndApply(cfg scanguard.Config) (ScanguardStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg.Sources = trimNonEmpty(cfg.Sources)
	cfg.Whitelist = trimNonEmpty(cfg.Whitelist)
	if err := scanguard.Validate(cfg); err != nil {
		return s.Status(), err
	}
	if err := s.persistConfig(cfg); err != nil {
		return s.Status(), err
	}
	var err error
	if cfg.Enabled {
		_, err = scanguard.Apply(cfg, s.keepPorts())
	} else {
		err = scanguard.Disable()
		if err != nil {
			logger.Warning("scanguard: disable cleanup:", err)
		}
	}
	s.recordResult(cfg.Enabled, err)
	return s.Status(), err
}

// ForceRefresh re-fetches the lists now (operator button).
func (s *ScanguardService) ForceRefresh() (ScanguardStatus, error) {
	cfg := s.LoadConfig()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !cfg.Enabled {
		return s.Status(), scanguard.ErrDisabled
	}
	_, err := scanguard.Apply(cfg, s.keepPorts())
	s.recordResult(true, err)
	return s.Status(), err
}

// keepPorts lists the listening ports of enabled tunnel sidecars that receive
// traffic from upstream relay networks (csqtt TURN/RTP via the VK turn
// relays, qWDTT DTLS/WG entry) — the scanner guard exempts them so a
// blocklist covering those relay subnets cannot cut the panel's own tunnels
// (lucx.358: csqtt/qwdtt died once a source list added 90.156.232.0/21).
func (s *ScanguardService) keepPorts() []int {
	inbounds, err := (&InboundService{}).GetAllInbounds()
	if err != nil {
		logger.Warning("scanguard: tunnel port probe failed:", err)
		return nil
	}
	ports := map[int]struct{}{}
	for _, ib := range inbounds {
		if ib == nil || ib.NodeID != nil || !ib.Enable {
			continue
		}
		switch ib.Protocol {
		case model.Csqtt:
			if cfg, ok := tunnel.CsqttConfigFromInbound(ib); ok {
				ports[tunnel.CsqttListenPort(cfg)] = struct{}{}
			}
		case model.Qwdtt:
			if cfg, ok := tunnel.QwdttConfigFromInbound(ib); ok {
				ports[tunnel.QwdttDTLSPort(cfg)] = struct{}{}
				if cfg.WGPort > 0 && cfg.WGPort <= 65535 {
					ports[cfg.WGPort] = struct{}{}
				}
			}
		}
	}
	out := make([]int, 0, len(ports))
	for p := range ports {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// Reconcile is the cron tick: sweep rules when disabled; when enabled
// re-apply as soon as rules or sets are missing (reboot, iptables flush by
// fail2ban/docker), the tunnel-port keeps drifted (port changed, tunnel
// toggled) or a list refresh is due — at most once per interval.
func (s *ScanguardService) Reconcile() {
	cfg := s.LoadConfig()
	ports := s.keepPorts()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !cfg.Enabled {
		if err := scanguard.Disable(); err != nil {
			logger.Warning("scanguard: reconcile sweep:", err)
		}
		return
	}
	st := scanguard.Status(ports)
	if st.RulesV4 && (st.V4Entries > 0 || st.V6Entries > 0) && st.KeepOk && !s.refreshDue() {
		return
	}
	_, err := scanguard.Apply(cfg, ports)
	s.recordResult(true, err)
}

func (s *ScanguardService) refreshDue() bool {
	v, ok, err := scanguardSetting(scanguardLastUpdatedKey)
	if err != nil || !ok {
		return true
	}
	unix, parseErr := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if parseErr != nil || unix <= 0 {
		return true
	}
	return time.Since(time.Unix(unix, 0)) >= scanguardRefreshInterval
}

func (s *ScanguardService) recordResult(enabled bool, err error) {
	lastUpdated := ""
	if enabled && err == nil {
		lastUpdated = strconv.FormatInt(time.Now().Unix(), 10)
	}
	lastErr := ""
	if err != nil {
		lastErr = err.Error()
		logger.Warning("scanguard: apply failed:", err)
	}
	if err := saveScanguardSetting(scanguardLastUpdatedKey, lastUpdated); err != nil {
		logger.Warning("scanguard: persist lastUpdated:", err)
	}
	if err := saveScanguardSetting(scanguardLastErrorKey, lastErr); err != nil {
		logger.Warning("scanguard: persist lastError:", err)
	}
}

func (s *ScanguardService) persistConfig(cfg scanguard.Config) error {
	if err := saveScanguardSetting(scanguardEnabledKey, strconv.FormatBool(cfg.Enabled)); err != nil {
		return err
	}
	rawSrc, err := json.Marshal(cfg.Sources)
	if err != nil {
		return err
	}
	if err := saveScanguardSetting(scanguardSourcesKey, string(rawSrc)); err != nil {
		return err
	}
	rawWl, err := json.Marshal(cfg.Whitelist)
	if err != nil {
		return err
	}
	return saveScanguardSetting(scanguardWhitelistKey, string(rawWl))
}

func scanguardSetting(key string) (string, bool, error) {
	setting := &model.Setting{}
	err := database.GetDB().Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if database.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return setting.Value, true, nil
}

func saveScanguardSetting(key, value string) error {
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

func trimNonEmpty(list []string) []string {
	out := []string{}
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
