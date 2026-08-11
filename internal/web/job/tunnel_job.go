// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TunnelJob reconciles external tunnel sidecars and folds NaiveProxy access_log
// traffic/online into panel accounting (mirrors AwgJob / MtprotoJob). Naive
// clients never hit Xray user>>>email stats, so without this they stay offline
// with zero traffic forever.
type TunnelJob struct {
	tunnelService  service.TunnelService
	inboundService service.InboundService
	clientService  service.ClientService
	settingService service.SettingService
}

// NewTunnelJob creates a new tunnel reconcile/traffic job instance.
func NewTunnelJob() *TunnelJob {
	return new(TunnelJob)
}

// Run converges tunnel cores once, then scrapes Naive access logs for
// per-client traffic deltas and online status.
func (j *TunnelJob) Run() {
	j.tunnelService.Reconcile()
	j.collectNaiveTraffic()
}

func (j *TunnelJob) collectNaiveTraffic() {
	secret, err := j.settingService.GetSecret()
	if err != nil || len(secret) == 0 {
		return
	}
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel job: get inbounds failed:", err)
		return
	}

	var targets []tunnel.NaiveScrapeTarget
	routedTags := make(map[string]bool)
	activeTags := make([]string, 0)

	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Naive || !ib.Enable || ib.NodeID != nil {
			continue
		}
		cfg, ok := tunnel.ConfigFromInbound(ib)
		if !ok || cfg.UseRawConfig {
			continue
		}
		tag := strings.TrimSpace(ib.Tag)
		if tag == "" {
			continue
		}
		activeTags = append(activeTags, tag)
		if cfg.RouteThroughXray {
			routedTags[tag] = true
		}
		userToEmail := naiveUserMapForInbound(secret, ib)
		if len(userToEmail) == 0 {
			continue
		}
		targets = append(targets, tunnel.NaiveScrapeTarget{
			Key:         tunnel.NaiveKey(ib.Id),
			Tag:         tag,
			UserToEmail: userToEmail,
		})
	}

	// Legacy global lucxTunnel_naive when no protocol=naive inbound exists.
	if len(targets) == 0 {
		if t, tag, routed, ok := j.legacyNaiveTarget(secret); ok {
			targets = append(targets, t)
			activeTags = append(activeTags, tag)
			if routed {
				routedTags[tag] = true
			}
		}
	}

	if len(targets) == 0 {
		// Still refresh online with empty set for active tags so stale emails age out.
		if len(activeTags) > 0 {
			j.inboundService.RefreshLocalOnlineClients(nil, activeTags)
		}
		return
	}

	now := time.Now()
	deltas, onlineEmails := tunnel.GetManager().CollectNaiveTraffic(targets, now, tunnel.NaiveOnlineGrace)

	// Lines without ts get sentinel -1 → treat as online now.
	onlineEmails = normalizeNaiveOnline(onlineEmails, deltas, now)

	clientTraffics := make([]*xray.ClientTraffic, 0, len(deltas))
	inboundUp := make(map[string]int64)
	inboundDown := make(map[string]int64)
	for _, d := range deltas {
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{
			Email: d.Email,
			Up:    d.Up,
			Down:  d.Down,
		})
		// Routed inbounds already meter inbound>>>tag via the SOCKS bridge;
		// only roll up non-routed here (same as awg_job / mtproto_job).
		if !routedTags[d.Tag] {
			inboundUp[d.Tag] += d.Up
			inboundDown[d.Tag] += d.Down
		}
	}
	traffics := make([]*xray.Traffic, 0, len(inboundUp))
	for tag, up := range inboundUp {
		traffics = append(traffics, &xray.Traffic{
			IsInbound: true,
			Tag:       tag,
			Up:        up,
			Down:      inboundDown[tag],
		})
	}
	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("tunnel job: add traffic failed:", err)
		}
	}
	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}

func naiveUserMapForInbound(secret []byte, ib *model.Inbound) map[string]string {
	var s struct {
		Clients []struct {
			Email  string `json:"email"`
			Enable bool   `json:"enable"`
		} `json:"clients"`
	}
	_ = json.Unmarshal([]byte(ib.Settings), &s)
	out := make(map[string]string)
	for _, c := range s.Clients {
		email := strings.TrimSpace(c.Email)
		if !c.Enable || email == "" {
			continue
		}
		pair := tunnel.ClientAuthForInbound(secret, ib.Id, email)
		if pair.User != "" {
			out[pair.User] = email
		}
	}
	return out
}

func (j *TunnelJob) legacyNaiveTarget(secret []byte) (tunnel.NaiveScrapeTarget, string, bool, bool) {
	cfg, err := j.tunnelService.LoadNaiveConfig()
	if err != nil || !cfg.Enabled || cfg.UseRawConfig {
		return tunnel.NaiveScrapeTarget{}, "", false, false
	}
	clients, err := j.clientService.List()
	if err != nil {
		return tunnel.NaiveScrapeTarget{}, "", false, false
	}
	userToEmail := make(map[string]string)
	for _, c := range clients {
		email := strings.TrimSpace(c.Email)
		if !c.Enable || email == "" {
			continue
		}
		pair := tunnel.ClientAuth(secret, email)
		if pair.User != "" {
			userToEmail[pair.User] = email
		}
	}
	if len(userToEmail) == 0 {
		return tunnel.NaiveScrapeTarget{}, "", false, false
	}
	// Legacy core has no inbound tag; skip inbound rollup (empty tag) and still
	// attribute per-client traffic/online.
	return tunnel.NaiveScrapeTarget{
		Key:         string(tunnel.Naive),
		Tag:         "",
		UserToEmail: userToEmail,
	}, "", cfg.RouteThroughXray, true
}

// normalizeNaiveOnline drops empty emails; CollectNaiveTraffic already applied grace.
// Deltas this tick also count as online (activity even when ts was missing).
func normalizeNaiveOnline(online []string, deltas []tunnel.NaiveClientTraffic, _ time.Time) []string {
	set := make(map[string]struct{}, len(online)+len(deltas))
	for _, e := range online {
		if e = strings.TrimSpace(e); e != "" {
			set[e] = struct{}{}
		}
	}
	for _, d := range deltas {
		if e := strings.TrimSpace(d.Email); e != "" {
			set[e] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for e := range set {
		out = append(out, e)
	}
	return out
}
