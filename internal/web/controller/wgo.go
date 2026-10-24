// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	kernelwg "github.com/mhsanaei/3x-ui/v3/internal/lucx/wireguard"
)

// wgoEngineOf resolves the wgobfs engine process of one inbound: the wgo
// proxy of a kernel WireGuard inbound (UDP — no TCP probe) or the wgosocks
// exit itself (TCP probe on the listen port).
func wgoEngineOf(inbound *model.Inbound) (core tunnel.Name, key string, probePort int, ok bool) {
	switch {
	case kernelwg.IsKernel(inbound):
		return tunnel.Wgo, tunnel.WgoKey(inbound.Id), 0, true
	case inbound.Protocol == model.Wgosocks:
		return tunnel.Wgosocks, tunnel.WgosocksKey(inbound.Id), inbound.Port, true
	}
	return "", "", 0, false
}

// wgoStatus reports the wgobfs engine health of one inbound for the preset
// status line in the WG form / the wgosocks card (read-only; the reconcile
// loop owns the lifecycle).
//
// LUCX-HOOK: wgobfs per-inbound engine status.
func (a *InboundController) wgoStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "wgo status: bad id", err)
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		jsonMsg(c, "wgo status: inbound not found", err)
		return
	}
	core, key, probePort, ok := wgoEngineOf(inbound)
	if !ok {
		jsonMsg(c, "wgo status: inbound has no obfuscation engine", nil)
		return
	}
	mgr := tunnel.GetManager()
	st := mgr.StatusOf(tunnel.Instance{Core: core, Key: key, Enabled: inbound.Enable, ProbePort: probePort})
	jsonObj(c, gin.H{
		"core":        string(core),
		"displayName": core.DisplayName(),
		"running":     st.Running,
		"listening":   st.Listening,
		"responding":  st.Responding,
		"lastLog":     mgr.LastLogKey(key),
		"binaryPath":  core.BinaryPath(),
	}, nil)
}

// wgoLogs tails the wgobfs engine ring log of one inbound.
//
// LUCX-HOOK: wgobfs per-inbound engine logs.
func (a *InboundController) wgoLogs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "wgo logs: bad id", err)
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		jsonMsg(c, "wgo logs: inbound not found", err)
		return
	}
	_, key, _, ok := wgoEngineOf(inbound)
	if !ok {
		jsonMsg(c, "wgo logs: inbound has no obfuscation engine", nil)
		return
	}
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, tunnel.GetManager().LogsKey(key, lines), nil)
}
