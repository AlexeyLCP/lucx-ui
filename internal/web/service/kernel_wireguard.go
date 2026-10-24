// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	kernelwg "github.com/mhsanaei/3x-ui/v3/internal/lucx/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func injectKernelWireguard(cfg *xray.Config, ib *model.Inbound) error {
	if !kernelwg.IsKernel(ib) {
		return nil
	}
	inst, err := kernelwg.Parse(ib)
	if err != nil {
		return err
	}
	for _, existing := range cfg.InboundConfigs {
		if existing.Tag == ib.Tag || existing.Port == inst.TproxyPort {
			return fmt.Errorf("kernel WireGuard TPROXY listener conflicts with another inbound")
		}
	}
	bridge := awg.TproxyInbound(ib.Tag, inst.TproxyPort)
	var sniff struct {
		Enabled bool `json:"enabled"`
	}
	if json.Unmarshal([]byte(ib.Sniffing), &sniff) == nil && sniff.Enabled {
		bridge.Sniffing = json_util.RawMessage(ib.Sniffing)
	}
	cfg.InboundConfigs = append(cfg.InboundConfigs, bridge)
	return nil
}

func (s *InboundService) validateKernelWireguard(ib *model.Inbound) error {
	if !kernelwg.IsKernel(ib) {
		return nil
	}
	probe := *ib
	if probe.Id == 0 {
		probe.Id = 1
	}
	inst, err := kernelwg.Parse(&probe)
	if err != nil {
		return err
	}
	probe = *ib
	probe.Protocol = model.Tunnel
	probe.Listen = "127.0.0.1"
	probe.Port = inst.TproxyPort
	probe.Settings = `{"allowedNetwork":"tcp,udp"}`
	conflict, err := s.checkPortConflict(&probe, ib.Id)
	if err != nil {
		return err
	}
	if conflict != nil {
		return fmt.Errorf("kernel WireGuard TPROXY: %s", conflict.String())
	}
	panelPort, err := (&SettingService{}).GetPort()
	if err != nil {
		return err
	}
	if panelPort == inst.TproxyPort {
		return fmt.Errorf("kernel WireGuard TPROXY port conflicts with panel")
	}
	others, err := s.GetAllInbounds()
	if err != nil {
		return err
	}
	unchanged := false
	for _, other := range others {
		if other.Id == ib.Id && kernelwg.IsKernel(other) && other.Enable {
			old, e := kernelwg.Parse(other)
			unchanged = e == nil && old.TproxyPort == inst.TproxyPort
		}
		if other.Id != ib.Id && other.NodeID == nil && (parseSettingsIntKey(other.Settings, "routeXrayPort") == inst.TproxyPort || parseSettingsIntKey(other.Settings, "tproxyPort") == inst.TproxyPort) {
			return fmt.Errorf("kernel WireGuard TPROXY port reserved by inbound %d", other.Id)
		}
	}
	if !unchanged {
		address := fmt.Sprintf("127.0.0.1:%d", inst.TproxyPort)
		var lc net.ListenConfig
		tcp, e := lc.Listen(context.Background(), "tcp4", address)
		if e != nil {
			return fmt.Errorf("kernel WireGuard TPROXY TCP port unavailable")
		}
		defer tcp.Close()
		udp, e := lc.ListenPacket(context.Background(), "udp4", address)
		if e != nil {
			return fmt.Errorf("kernel WireGuard TPROXY UDP port unavailable")
		}
		udp.Close()
	}
	return nil
}

func kernelWireguardBridgeChanged(old, new *model.Inbound) bool {
	if kernelwg.IsKernel(old) != kernelwg.IsKernel(new) {
		return true
	}
	if !kernelwg.IsKernel(new) {
		return false
	}
	var a, b struct {
		TproxyPort int `json:"tproxyPort"`
	}
	_ = json.Unmarshal([]byte(old.Settings), &a)
	_ = json.Unmarshal([]byte(new.Settings), &b)
	return old.Tag != new.Tag || old.Enable != new.Enable || a.TproxyPort != b.TproxyPort || old.Sniffing != new.Sniffing
}

func isKernelWireguardInbound(ib *model.Inbound) bool { return kernelwg.IsKernel(ib) }

func (s *InboundService) applyLocalKernelWireguard(id int) {
	ib, err := s.GetInbound(id)
	if err != nil || !kernelwg.IsKernel(ib) || ib.NodeID != nil {
		return
	}
	if !ib.Enable {
		err = kernelwg.GetManager().Remove(id)
	} else {
		var inst kernelwg.Instance
		inst, err = kernelwg.Parse(ib)
		if err == nil {
			err = kernelwg.GetManager().Ensure(inst)
		} else {
			err = kernelwg.GetManager().RevokeDisallowed(ib)
		}
	}
	if err != nil {
		logger.Warningf("kernel WireGuard inbound %d client apply: %v", id, err)
	}
}

var kernelWireguardBridges sync.Map

func injectKernelWireguardSafely(cfg *xray.Config, ib *model.Inbound) {
	if err := injectKernelWireguard(cfg, ib); err == nil {
		kernelWireguardBridges.Store(ib.Id, cfg.InboundConfigs[len(cfg.InboundConfigs)-1])
		return
	} else {
		logger.Warningf("kernel WireGuard inbound %d bridge skipped: %v", ib.Id, err)
	}
	if last, ok := kernelWireguardBridges.Load(ib.Id); ok {
		previous := last.(xray.InboundConfig)
		for _, existing := range cfg.InboundConfigs {
			if existing.Tag == previous.Tag || existing.Port == previous.Port {
				logger.Warningf("kernel WireGuard inbound %d previous bridge conflicts with another listener", ib.Id)
				return
			}
		}
		cfg.InboundConfigs = append(cfg.InboundConfigs, previous)
	}
}
