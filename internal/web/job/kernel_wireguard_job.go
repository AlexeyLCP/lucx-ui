// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	kernelwg "github.com/mhsanaei/3x-ui/v3/internal/lucx/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type KernelWireguardJob struct {
	inboundService service.InboundService
	lastTick       time.Time
	pending        []*xray.ClientTraffic
}

func NewKernelWireguardJob() *KernelWireguardJob { return new(KernelWireguardJob) }
func (j *KernelWireguardJob) Run() {
	rows, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("kernel WireGuard: cannot read inbounds")
		return
	}
	var desired []kernelwg.Instance
	var tags []string
	for _, ib := range rows {
		if !kernelwg.IsKernel(ib) || !ib.Enable || ib.NodeID != nil {
			continue
		}
		inst, err := kernelwg.Parse(ib)
		if err != nil {
			logger.Warningf("kernel WireGuard inbound %d: %v", ib.Id, err)
			continue
		}
		desired = append(desired, inst)
		tags = append(tags, ib.Tag)
	}
	manager := kernelwg.GetManager()
	for _, err := range manager.Reconcile(desired) {
		logger.Warning("kernel WireGuard reconcile: ", err)
	}
	online := map[string]bool{}
	byEmail := map[string]*xray.ClientTraffic{}
	for _, old := range j.pending {
		byEmail[old.Email] = old
	}
	for _, usage := range manager.CollectTraffic() {
		if usage.Email == "" {
			continue
		}
		if usage.Online {
			online[usage.Email] = true
		}
		traffic := byEmail[usage.Email]
		if traffic == nil {
			traffic = &xray.ClientTraffic{Email: usage.Email}
			byEmail[usage.Email] = traffic
		}
		traffic.Up += usage.Up
		traffic.Down += usage.Down
	}
	var clients []*xray.ClientTraffic
	for _, row := range byEmail {
		clients = append(clients, row)
	}
	if len(clients) > 0 {
		if _, _, err := j.inboundService.AddTraffic(nil, clients); err != nil {
			j.pending = clients
			logger.Warning("kernel WireGuard: traffic persistence failed")
			return
		}
	}
	j.pending = nil
	var emails []string
	for email := range online {
		emails = append(emails, email)
	}
	j.inboundService.RefreshLocalOnlineClients(emails, tags)
	now := time.Now()
	elapsed := now.Sub(j.lastTick)
	if j.lastTick.IsZero() {
		elapsed = 10 * time.Second
	}
	j.lastTick = now
	_, normalized := normalizeAwgDeltas(nil, clients, elapsed)
	kernelWGSpeed.Lock()
	kernelWGSpeed.snapshot = awgSpeedSnapshot{clients: normalized, at: now}
	kernelWGSpeed.Unlock()
}

var kernelWGSpeed struct {
	sync.Mutex
	snapshot awgSpeedSnapshot
}

func takeKernelWireguardSpeed() []*xray.ClientTraffic {
	kernelWGSpeed.Lock()
	defer kernelWGSpeed.Unlock()
	if time.Since(kernelWGSpeed.snapshot.at) > awgSpeedTTL {
		return nil
	}
	return kernelWGSpeed.snapshot.clients
}
