// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	kernelwg "github.com/mhsanaei/3x-ui/v3/internal/lucx/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type kernelWireguardStore interface {
	GetAllInbounds() ([]*model.Inbound, error)
	AddTraffic([]*xray.Traffic, []*xray.ClientTraffic) (bool, bool, error)
	RefreshLocalOnlineClients([]string, []string)
}
type kernelWireguardManager interface {
	ReconcilePreserving([]kernelwg.Instance, []int) []error
	CollectTraffic() []kernelwg.Usage
	RevokeDisallowed(*model.Inbound) error
}

// kernelWireguardWgo reconciles the wgobfs obfuscation processes; a variable
// indirection so tests can fake it without a live tunnel manager.
var syncWgoFirewall = kernelwg.SyncWgoFirewall

type kernelWireguardWgo interface {
	ReconcileWgo([]tunnel.Instance)
}

type KernelWireguardJob struct {
	store    kernelWireguardStore
	manager  kernelWireguardManager
	wgo      kernelWireguardWgo
	now      func() time.Time
	lastTick time.Time
	pending  []*xray.ClientTraffic
	// reconcileWarned dedupes an unchanged reconcile failure across 10 s
	// ticks (missing binary on the host would otherwise flood the journal).
	reconcileWarned map[string]bool
}

func NewKernelWireguardJob() *KernelWireguardJob {
	return &KernelWireguardJob{store: &service.InboundService{}, manager: kernelwg.GetManager(), wgo: tunnel.GetManager(), now: time.Now}
}

// warnReconcile logs reconcile failures once per distinct message and clears
// the memory on a clean tick, so a host without the wg/awg binary does not
// flood the journal every 10 s. Returns how many lines were logged.
func (j *KernelWireguardJob) warnReconcile(reconcileErrors []error) int {
	warned := 0
	fresh := map[string]bool{}
	for _, err := range reconcileErrors {
		msg := err.Error()
		fresh[msg] = true
		if !j.reconcileWarned[msg] {
			warned++
			logger.Warning("kernel WireGuard reconcile: ", err)
		}
	}
	if len(fresh) == 0 {
		j.reconcileWarned = nil
	} else {
		j.reconcileWarned = fresh
	}
	return warned
}

func (j *KernelWireguardJob) Run() {
	rows, err := j.store.GetAllInbounds()
	if err != nil {
		logger.Warning("kernel WireGuard: cannot read inbounds")
		return
	}
	var desired []kernelwg.Instance
	var wgoWant []tunnel.Instance
	var fwStates []kernelwg.WgoFirewallState
	var preserve []int
	for _, ib := range rows {
		if !kernelwg.IsKernel(ib) || !ib.Enable || ib.NodeID != nil {
			continue
		}
		inst, err := kernelwg.Parse(ib)
		if err != nil {
			logger.Warningf("kernel WireGuard inbound %d device settings retained: %v", ib.Id, err)
			if revokeErr := j.manager.RevokeDisallowed(ib); revokeErr != nil {
				logger.Warningf("kernel WireGuard inbound %d revoke: %v", ib.Id, revokeErr)
			}
			preserve = append(preserve, ib.Id)
			continue
		}
		desired = append(desired, inst)
		if wgoInst, fw, ok := kernelwg.ObfuscationTargets(ib); ok {
			wgoWant = append(wgoWant, wgoInst)
			fwStates = append(fwStates, fw)
		}
	}
	j.warnReconcile(j.manager.ReconcilePreserving(desired, preserve))
	if j.wgo != nil {
		j.wgo.ReconcileWgo(wgoWant)
	}
	syncWgoFirewall(fwStates)
	online := map[string]bool{}
	onlineTags := map[string]bool{}
	fresh := map[string]*xray.ClientTraffic{}
	for _, u := range j.manager.CollectTraffic() {
		if u.Email == "" {
			continue
		}
		if u.Online {
			online[u.Email] = true
			onlineTags[u.Tag] = true
		}
		if fresh[u.Email] == nil {
			fresh[u.Email] = &xray.ClientTraffic{Email: u.Email}
		}
		fresh[u.Email].Up += u.Up
		fresh[u.Email].Down += u.Down
	}
	total := map[string]*xray.ClientTraffic{}
	for _, old := range j.pending {
		copy := *old
		total[old.Email] = &copy
	}
	var deltas []*xray.ClientTraffic
	for _, delta := range fresh {
		deltas = append(deltas, delta)
		if total[delta.Email] == nil {
			total[delta.Email] = &xray.ClientTraffic{Email: delta.Email}
		}
		total[delta.Email].Up += delta.Up
		total[delta.Email].Down += delta.Down
	}
	var clients []*xray.ClientTraffic
	for _, row := range total {
		clients = append(clients, row)
	}
	j.pending = nil
	if len(clients) > 0 {
		if _, _, err := j.store.AddTraffic(nil, clients); err != nil {
			j.pending = clients
			logger.Warning("kernel WireGuard: traffic persistence failed")
		}
	}
	var emails, tags []string
	for email := range online {
		emails = append(emails, email)
	}
	for tag := range onlineTags {
		tags = append(tags, tag)
	}
	j.store.RefreshLocalOnlineClients(emails, tags)
	now := j.now()
	elapsed := now.Sub(j.lastTick)
	if j.lastTick.IsZero() {
		elapsed = 10 * time.Second
	}
	j.lastTick = now
	_, normalized := normalizeAwgDeltas(nil, deltas, elapsed)
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
