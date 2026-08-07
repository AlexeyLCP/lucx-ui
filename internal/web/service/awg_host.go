// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func fillAwgHostStatus(status *Status) {
	if status == nil {
		return
	}
	hs := awg.CollectHostStatus()
	status.Awg.ModuleLoaded = hs.ModuleLoaded
	status.Awg.ModuleAwg3 = hs.ModuleAwg3
	status.Awg.Version = hs.Version
	status.Awg.Interfaces = hs.Interfaces
	status.Awg.Ifnames = hs.Ifnames
	switch {
	case !hs.ModuleLoaded:
		status.Awg.State = Error
		status.Awg.ErrorMsg = "amneziawg kernel module not loaded"
	case hs.Interfaces > 0:
		status.Awg.State = Running
		status.Awg.ErrorMsg = ""
	default:
		status.Awg.State = Stop
		status.Awg.ErrorMsg = ""
	}
}

var (
	awgRestartMu      sync.Mutex
	awgRestartRunning bool
)

// RestartAwg stops all managed AWG interfaces and immediately reconciles them
// (inbounds + outbounds) — same as the next AwgJob tick, but on demand.
func (s *ServerService) RestartAwg() error {
	awgRestartMu.Lock()
	if awgRestartRunning {
		awgRestartMu.Unlock()
		return fmt.Errorf("AWG restart already in progress")
	}
	awgRestartRunning = true
	awgRestartMu.Unlock()
	defer func() {
		awgRestartMu.Lock()
		awgRestartRunning = false
		awgRestartMu.Unlock()
	}()

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return fmt.Errorf("list inbounds: %w", err)
	}
	var desired []awg.Instance
	for _, ib := range inbounds {
		if ib.Protocol != model.AWG || !ib.Enable || ib.NodeID != nil {
			continue
		}
		if inst, ok := awg.InstanceFromInbound(ib); ok {
			desired = append(desired, inst)
		}
	}

	mgr := awg.GetManager()
	mgr.StopAll()
	mgr.Reconcile(desired)

	// Outbound clients (awgo-N)
	svc := &AwgOutboundService{}
	outbounds, oerr := svc.GetOutbounds()
	if oerr != nil {
		logger.Warning("awg restart: list outbounds:", oerr)
		return nil
	}
	for _, o := range outbounds {
		ifname := "awgo-" + strconv.Itoa(o.Id)
		if !o.Enable {
			_ = mgr.RemoveClient(ifname)
			continue
		}
		if ci, ok := awg.ClientInstanceFromOutbound(o); ok {
			if err := mgr.EnsureClient(ci); err != nil {
				logger.Warning("awg restart: outbound", o.Tag, err)
			}
		}
	}
	return nil
}
