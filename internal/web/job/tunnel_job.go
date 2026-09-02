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

	"github.com/mhsanaei/3x-ui/v3/internal/database"
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
	(&service.SidecarOutboundService{}).Reconcile()
	j.collectNaiveTraffic()
	j.collectMieruTraffic()
	j.collectTrustTunnelTraffic()
	j.collectAnytlsTraffic()
	j.collectOlcrtcTraffic()
	j.collectQwdttTraffic()
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

// collectMieruTraffic scrapes running mita daemons for per-user byte counters
// (folds deltas like collectNaiveTraffic) and ages out online status.
func (j *TunnelJob) collectMieruTraffic() {
	secret, err := j.settingService.GetSecret()
	if err != nil || len(secret) == 0 {
		return
	}
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel job: get inbounds failed:", err)
		return
	}

	var targets []tunnel.MieruScrapeTarget
	routedTags := make(map[string]bool)
	activeTags := make([]string, 0)

	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Mieru || !ib.Enable || ib.NodeID != nil {
			continue
		}
		tag := strings.TrimSpace(ib.Tag)
		if tag == "" {
			continue
		}
		activeTags = append(activeTags, tag)
		if cfg, ok := tunnel.MieruConfigFromInbound(ib); ok && cfg.RouteThroughXray {
			routedTags[tag] = true
		}
		userToEmail := mieruUserMapForInbound(secret, ib)
		if len(userToEmail) == 0 {
			continue
		}
		targets = append(targets, tunnel.MieruScrapeTarget{
			Key:         tunnel.MieruKey(ib.Id),
			Tag:         tag,
			UserToEmail: userToEmail,
		})
	}

	if len(targets) == 0 {
		if len(activeTags) > 0 {
			j.inboundService.RefreshLocalOnlineClients(nil, activeTags)
		}
		return
	}

	now := time.Now()
	deltas, onlineEmails := tunnel.GetManager().CollectMieruTraffic(targets, now, tunnel.MieruOnlineGrace)

	// Deltas this tick also count as online (activity even when LastActive
	// parsing failed), mirroring normalizeNaiveOnline.
	onlineSet := make(map[string]struct{}, len(onlineEmails)+len(deltas))
	for _, e := range onlineEmails {
		if e = strings.TrimSpace(e); e != "" {
			onlineSet[e] = struct{}{}
		}
	}
	for _, d := range deltas {
		if e := strings.TrimSpace(d.Email); e != "" {
			onlineSet[e] = struct{}{}
		}
	}
	onlineEmails = make([]string, 0, len(onlineSet))
	for e := range onlineSet {
		onlineEmails = append(onlineEmails, e)
	}

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
		// only roll up non-routed here (same as naive/AWG/mtproto jobs).
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
			logger.Warning("tunnel job: add mieru traffic failed:", err)
		}
	}
	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}

// collectTrustTunnelTraffic scrapes endpoint Prometheus metrics. Counters are
// aggregate (no username labels) → inbound rollup always; per-client
// traffic/online only when the inbound has a single enabled client.
func (j *TunnelJob) collectTrustTunnelTraffic() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel job: get inbounds failed:", err)
		return
	}
	var targets []tunnel.TrustTunnelScrapeTarget
	routedTags := map[string]bool{}
	emailsByTag := map[string][]string{}
	activeTags := make([]string, 0)
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.TrustTunnel || !ib.Enable || ib.NodeID != nil {
			continue
		}
		tag := strings.TrimSpace(ib.Tag)
		if tag == "" {
			continue
		}
		activeTags = append(activeTags, tag)
		emailsByTag[tag] = trustTunnelEmailsForInbound(ib)
		cfg, ok := tunnel.TrustTunnelConfigFromInbound(ib)
		if ok && cfg.RouteThroughXray {
			routedTags[tag] = true
		}
		if !ok || cfg.MetricsPort <= 0 {
			continue
		}
		targets = append(targets, tunnel.TrustTunnelScrapeTarget{
			Key:         tunnel.TrustTunnelKey(ib.Id),
			Tag:         tag,
			MetricsPort: cfg.MetricsPort,
		})
	}
	if len(targets) == 0 {
		if len(activeTags) > 0 {
			j.inboundService.RefreshLocalOnlineClients(nil, activeTags)
		}
		return
	}
	snaps := tunnel.GetManager().CollectTrustTunnelTraffic(targets)
	traffics := make([]*xray.Traffic, 0, len(snaps))
	clientTraffics := make([]*xray.ClientTraffic, 0)
	onlineEmails := make([]string, 0)
	for _, d := range snaps {
		if !routedTags[d.Tag] && (d.Up > 0 || d.Down > 0) {
			traffics = append(traffics, &xray.Traffic{
				IsInbound: true,
				Tag:       d.Tag,
				Up:        d.Up,
				Down:      d.Down,
			})
		}
		email := tunnel.TrustTunnelSoleClient(emailsByTag[d.Tag])
		if email == "" {
			continue
		}
		if d.Up > 0 || d.Down > 0 {
			clientTraffics = append(clientTraffics, &xray.ClientTraffic{
				Email: email,
				Up:    d.Up,
				Down:  d.Down,
			})
		}
		if d.Sessions > 0 || d.Up > 0 || d.Down > 0 {
			onlineEmails = append(onlineEmails, email)
		}
	}
	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("tunnel job: add trusttunnel traffic failed:", err)
		}
	}
	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}

func (j *TunnelJob) collectAnytlsTraffic() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel job: get inbounds failed:", err)
		return
	}
	var targets []tunnel.AnytlsScrapeTarget
	activeTags := make([]string, 0)
	emailsByTag := map[string][]string{}
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Anytls || !ib.Enable || ib.NodeID != nil {
			continue
		}
		tag := strings.TrimSpace(ib.Tag)
		if tag == "" {
			continue
		}
		cfg, ok := tunnel.AnytlsConfigFromInbound(ib)
		if !ok {
			continue
		}
		activeTags = append(activeTags, tag)
		emailsByTag[tag] = anytlsEmailsForInbound(ib)
		targets = append(targets, tunnel.AnytlsScrapeTarget{
			Key:  tunnel.AnytlsKey(ib.Id),
			Tag:  tag,
			Port: cfg.Port,
		})
	}
	if len(targets) == 0 {
		if len(activeTags) > 0 {
			j.inboundService.RefreshLocalOnlineClients(nil, activeTags)
		}
		return
	}
	snaps := tunnel.GetManager().CollectAnytlsTraffic(targets)
	traffics := make([]*xray.Traffic, 0, len(snaps))
	clientTraffics := make([]*xray.ClientTraffic, 0)
	onlineEmails := make([]string, 0)
	for _, d := range snaps {
		if d.Up > 0 || d.Down > 0 {
			traffics = append(traffics, &xray.Traffic{
				IsInbound: true,
				Tag:       d.Tag,
				Up:        d.Up,
				Down:      d.Down,
			})
		}
		email := tunnel.TrustTunnelSoleClient(emailsByTag[d.Tag])
		if email == "" {
			continue
		}
		if d.Up > 0 || d.Down > 0 {
			clientTraffics = append(clientTraffics, &xray.ClientTraffic{
				Email: email,
				Up:    d.Up,
				Down:  d.Down,
			})
		}
		if d.Sessions > 0 || d.Up > 0 || d.Down > 0 {
			onlineEmails = append(onlineEmails, email)
		}
	}
	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("tunnel job: add anytls traffic failed:", err)
		}
	}
	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}

func (j *TunnelJob) collectOlcrtcTraffic() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel job: get inbounds failed:", err)
		return
	}
	var snaps []tunnel.SidecarTraffic
	activeTags := make([]string, 0)
	emailsByTag := map[string][]string{}
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Olcrtc || !ib.Enable || ib.NodeID != nil {
			continue
		}
		tag := strings.TrimSpace(ib.Tag)
		if tag == "" {
			continue
		}
		activeTags = append(activeTags, tag)
		emailsByTag[tag] = anytlsEmailsForInbound(ib)
		snaps = append(snaps, tunnel.GetManager().CollectOlcrtcTraffic(tunnel.OlcrtcKey(ib.Id), tag))
	}
	j.commitSidecarScrape("olcrtc", snaps, emailsByTag, activeTags)
}

func (j *TunnelJob) collectQwdttTraffic() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tunnel job: get inbounds failed:", err)
		return
	}
	var snaps []tunnel.SidecarTraffic
	activeTags := make([]string, 0)
	emailsByTag := map[string][]string{}
	for _, ib := range inbounds {
		if ib == nil || ib.Protocol != model.Qwdtt || !ib.Enable || ib.NodeID != nil {
			continue
		}
		tag := strings.TrimSpace(ib.Tag)
		if tag == "" {
			continue
		}
		activeTags = append(activeTags, tag)
		emailsByTag[tag] = anytlsEmailsForInbound(ib)
		snaps = append(snaps, tunnel.GetManager().CollectQwdttTraffic(tag))
	}
	j.commitSidecarScrape("qwdtt", snaps, emailsByTag, activeTags)
}

func (j *TunnelJob) commitSidecarScrape(name string, snaps []tunnel.SidecarTraffic, emailsByTag map[string][]string, activeTags []string) {
	if len(snaps) == 0 {
		if len(activeTags) > 0 {
			j.inboundService.RefreshLocalOnlineClients(nil, activeTags)
		}
		return
	}
	traffics := make([]*xray.Traffic, 0, len(snaps))
	clientTraffics := make([]*xray.ClientTraffic, 0)
	onlineEmails := make([]string, 0)
	for _, d := range snaps {
		if d.Up > 0 || d.Down > 0 {
			traffics = append(traffics, &xray.Traffic{
				IsInbound: true,
				Tag:       d.Tag,
				Up:        d.Up,
				Down:      d.Down,
			})
		}
		email := tunnel.TrustTunnelSoleClient(emailsByTag[d.Tag])
		if email == "" {
			continue
		}
		if d.Up > 0 || d.Down > 0 {
			clientTraffics = append(clientTraffics, &xray.ClientTraffic{
				Email: email,
				Up:    d.Up,
				Down:  d.Down,
			})
		}
		if d.Sessions > 0 || d.Up > 0 || d.Down > 0 {
			onlineEmails = append(onlineEmails, email)
		}
	}
	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warningf("tunnel job: add %s traffic failed: %v", name, err)
		}
	}
	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}

func anytlsEmailsForInbound(ib *model.Inbound) []string {
	if ib == nil {
		return nil
	}
	clients, err := (&service.ClientService{}).ListForInbound(database.GetDB(), ib.Id)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(clients))
	for i := range clients {
		if !clients[i].Enable {
			continue
		}
		if e := strings.TrimSpace(clients[i].Email); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func trustTunnelEmailsForInbound(ib *model.Inbound) []string {
	var s struct {
		Clients []struct {
			Email  string `json:"email"`
			Enable bool   `json:"enable"`
		} `json:"clients"`
	}
	_ = json.Unmarshal([]byte(ib.Settings), &s)
	out := make([]string, 0, len(s.Clients))
	for _, c := range s.Clients {
		email := strings.TrimSpace(c.Email)
		if !c.Enable || email == "" {
			continue
		}
		out = append(out, email)
	}
	return out
}

func mieruUserMapForInbound(secret []byte, ib *model.Inbound) map[string]string {
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
		pair := tunnel.InboundAuthPair(secret, ib, email)
		if pair.User != "" {
			out[pair.User] = email
		}
	}
	return out
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
		pair := tunnel.InboundAuthPair(secret, ib, email)
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
