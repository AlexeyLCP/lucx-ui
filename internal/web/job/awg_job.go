// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AwgJob reconciles the running AWG kernel-interface sidecars against the
// enabled AWG inbounds in the database, restarts any that crashed, folds the
// per-inbound and per-client traffic scraped from `awg show dump` into the
// usual accounting, and reports online clients from fresh handshakes (AWG
// clients never pass through Xray's stats API, so without this they show
// offline forever). Mirrors MtprotoJob.
type AwgJob struct {
	inboundService service.InboundService
	clientService  service.ClientService
	lastTick       time.Time
}

// NewAwgJob creates a new AWG reconcile/traffic job instance.
func NewAwgJob() *AwgJob {
	return new(AwgJob)
}

// Run reconciles desired AWG inbounds with running interfaces and records
// traffic deltas.
func (j *AwgJob) Run() {
	// LUCX-HOOK: a module rebuild (RebuildAwgModule) stops every AWG interface
	// so rmmod can unload amneziawg. Reconciling here would re-bring them up
	// mid-build and keep the module busy, so skip the tick while it is in
	// flight — traffic collection is moot with the interfaces down anyway.
	if service.AwgRebuildRunning() {
		return
	}
	if !awg.KernelAvailable() {
		awg.GetManager().Reconcile(nil)
		return
	}
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("awg job: get inbounds failed:", err)
		return
	}

	var desired []awg.Instance
	routedTags := make(map[string]bool)
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := awg.InstanceFromInbound(ib); ok {
			desired = append(desired, inst)
			if inst.RouteThroughXray {
				routedTags[inst.Tag] = true
			}
		}
	}

	mgr := awg.GetManager()
	mgr.Reconcile(desired)

	deltas, peerDeltas, onlineByTag := mgr.CollectTraffic()

	// Map peer public keys to panel clients (email) for per-client accounting
	// and online status. One DB read per AWG inbound per tick; AWG inbounds
	// are few and their client lists are small.
	emailsByTag := make(map[string]map[string]string, len(onlineByTag))
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		clients, err := j.clientService.ListForInbound(nil, ib.Id)
		if err != nil {
			logger.Warning("awg job: list clients for inbound", ib.Id, "failed:", err)
			continue
		}
		byKey := make(map[string]string, len(clients))
		for _, c := range clients {
			if c.Enable && c.PublicKey != "" {
				byKey[c.PublicKey] = c.Email
			}
		}
		emailsByTag[ib.Tag] = byKey
	}

	// A routed inbound's total is already metered by Xray at the injected TUN
	// inbound (same tag), so only non-routed inbounds are rolled up here —
	// otherwise routeThroughXray inbounds count every byte twice (kernel
	// counters + Xray stats). Per-client deltas are always kept: the TUN
	// bridge cannot tell AWG peers apart. Mirrors mtproto_job's routedTags.
	traffics := make([]*xray.Traffic, 0, len(deltas))
	for _, d := range deltas {
		if routedTags[d.Tag] {
			continue
		}
		traffics = append(traffics, &xray.Traffic{
			IsInbound: true,
			Tag:       d.Tag,
			Up:        d.Up,
			Down:      d.Down,
		})
	}

	clientTraffics := make([]*xray.ClientTraffic, 0, len(peerDeltas))
	for _, pd := range peerDeltas {
		email, ok := emailsByTag[pd.Tag][pd.PublicKey]
		if !ok {
			continue
		}
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{
			Email: email,
			Up:    pd.Up,
			Down:  pd.Down,
		})
	}

	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("awg job: add traffic failed:", err)
		}
	}

	// LUCX-HOOK: live speed feed. The raw deltas above only reach the DB
	// (totals); the UI speed columns read the 5s Xray broadcast, which never
	// sees AWG. Store this tick's deltas normalized to the 5s window so
	// XrayTrafficJob can fold them into its next frame (awg_speed_buffer.go).
	// An idle tick stores an empty snapshot, clearing the speed columns.
	now := time.Now()
	if !j.lastTick.IsZero() && now.Sub(j.lastTick) >= time.Second {
		nT, nC := normalizeAwgDeltas(traffics, clientTraffics, now.Sub(j.lastTick))
		storeAwgSpeedSnapshot(nT, nC)
	}
	j.lastTick = now
	// END LUCX-HOOK

	// Online status: fresh handshake (<180 s) = online. activeTags marks the
	// running AWG inbounds so the "active inbound" gating works for AWG too.
	var onlineEmails []string
	for tag, keys := range onlineByTag {
		for _, key := range keys {
			if email, ok := emailsByTag[tag][key]; ok {
				onlineEmails = append(onlineEmails, email)
			}
		}
	}
	activeTags := make([]string, 0, len(desired))
	for _, inst := range desired {
		activeTags = append(activeTags, inst.Tag)
	}
	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)

	// LUCX-HOOK: AWG outbound — reconcile client interfaces for enabled
	// awg_outbounds rows, and remove kernel interfaces for disabled/deleted
	// rows. Manager.SweepOrphanClients runs once on first call (sync.Once).
	{
		svc := &service.AwgOutboundService{}
		outbounds, err := svc.GetOutbounds()
		if err == nil {
			m := awg.GetManager()
			needXray := false
			for _, o := range outbounds {
				if !o.Enable {
					_ = m.RemoveClient("awgo-" + strconv.Itoa(o.Id))
					continue
				}
				if ci, ok := awg.ClientInstanceFromOutbound(o); ok {
					_, _, _, wasUp := m.CollectClientTraffic(ci.Ifname)
					if err := m.EnsureClient(ci); err != nil {
						logger.Warning("awg: outbound reconcile failed for", o.Tag, err)
						continue
					}
					if !wasUp {
						if _, _, _, up := m.CollectClientTraffic(ci.Ifname); up {
							needXray = true
						}
					}
				}
			}
			if needXray {
				if err := (&service.XrayService{}).RestartXray(false); err != nil {
					logger.Warning("awg: restart xray after outbound iface up:", err)
				}
			}
		}
	}
	// END LUCX-HOOK
}
