// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// AwgOutboundController exposes CRUD + status/test endpoints for client-mode
// AmneziaWG outbounds. Routes are registered under /panel/api/awg-outbounds/*
// (see the LUCX-HOOK block in internal/web/controller/api.go). Each outbound
// owns a kernel AWG interface named "awgo-{Id}" that the reconcile job
// (Task 7) keeps in sync with the DB row; this controller only reads the live
// state via awg.GetManager() for status/test, and mutates the DB row — the
// reconcile loop is what actually brings the interface up/down.
type AwgOutboundController struct {
	svc *service.AwgOutboundService
}

// NewAwgOutboundController creates a new AwgOutboundController bound to the
// given gin group and registers its 8 routes. Mirrors NewInboundController's
// self-registering constructor style.
func NewAwgOutboundController(g *gin.RouterGroup) *AwgOutboundController {
	a := &AwgOutboundController{svc: &service.AwgOutboundService{}}
	a.initRouter(g)
	return a
}

func (a *AwgOutboundController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.POST("/add", a.add)
	g.POST("/del/:id", a.del)
	g.POST("/update/:id", a.update)
	g.POST("/enable/:id", a.enable)
	g.GET("/status/:id", a.status)
	g.POST("/test/:id", a.test)
	g.POST("/parseConf", a.parseConf)
}

// awgOutboundRow is the list payload: the DB row plus a human-readable Status
// string derived from the live kernel interface. Status is one of:
//   - "up; handshake <dur> ago; rx=N tx=N"          — interface is up
//   - "down (fallback to default route active — WARNING)" — interface down,
//     traffic is bypassing the VPN via the so fallback. The WARNING is the
//     critical safety signal from the spec: a down outbound looks like an
//     open pipe, not a closed one.
//   - "disabled"                                     — row.Enable == false
type awgOutboundRow struct {
	*model.AwgOutbound
	Status string `json:"status"`
}

// list returns every AWG outbound row, augmented with a live status string.
func (a *AwgOutboundController) list(c *gin.Context) {
	outbounds, err := a.svc.GetOutbounds()
	if err != nil {
		jsonMsg(c, "awg-outbound: failed to list", err)
		return
	}
	rows := make([]awgOutboundRow, 0, len(outbounds))
	m := awg.GetManager()
	for _, o := range outbounds {
		r := awgOutboundRow{AwgOutbound: o}
		if o.Enable {
			ifname := "awgo-" + strconv.Itoa(o.Id)
			age, rx, tx, ok := m.CollectClientTraffic(ifname)
			if ok {
				r.Status = fmt.Sprintf("up; handshake %s ago; rx=%d tx=%d", age.Round(time.Second), rx, tx)
			} else {
				r.Status = "down (fallback to default route active — WARNING)"
			}
		} else {
			r.Status = "disabled"
		}
		rows = append(rows, r)
	}
	jsonObj(c, rows, nil)
}

// add persists a new AWG outbound row. Tag uniqueness + default Settings are
// filled in by the service.
func (a *AwgOutboundController) add(c *gin.Context) {
	var o model.AwgOutbound
	if err := c.ShouldBindJSON(&o); err != nil {
		jsonMsg(c, "awg-outbound: invalid body", err)
		return
	}
	out, err := a.svc.AddOutbound(&o)
	if err != nil {
		jsonMsg(c, "awg-outbound: add failed", err)
		return
	}
	jsonObj(c, out, nil)
}

// del removes a row and tears down the kernel interface immediately rather
// than waiting for the 10s reconcile tick — important so a deleted outbound
// stops carrying traffic right away.
func (a *AwgOutboundController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "awg-outbound: bad id", err)
		return
	}
	if err := a.svc.DelOutbound(id); err != nil {
		jsonMsg(c, "awg-outbound: del failed", err)
		return
	}
	_ = awg.GetManager().RemoveClient("awgo-" + strconv.Itoa(id))
	jsonObj(c, nil, nil)
}

// update overwrites a row. Tag uniqueness is re-checked by the service.
func (a *AwgOutboundController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "awg-outbound: bad id", err)
		return
	}
	var o model.AwgOutbound
	if err := c.ShouldBindJSON(&o); err != nil {
		jsonMsg(c, "awg-outbound: invalid body", err)
		return
	}
	o.Id = id
	if err := a.svc.UpdateOutbound(&o); err != nil {
		jsonMsg(c, "awg-outbound: update failed", err)
		return
	}
	jsonObj(c, o, nil)
}

// enable toggles row.Enable. Body: {"enable": bool}. The reconcile loop is
// what actually brings the kernel interface up/down in response.
func (a *AwgOutboundController) enable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "awg-outbound: bad id", err)
		return
	}
	var body struct {
		Enable bool `json:"enable"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := a.svc.SetOutboundEnable(id, body.Enable); err != nil {
		jsonMsg(c, "awg-outbound: enable failed", err)
		return
	}
	jsonObj(c, nil, nil)
}

// status returns the live kernel state for one outbound. `up` is false when
// the interface is missing or has never completed a handshake — in that case
// rx/tx are zero and the operator should treat the outbound as down (traffic
// is bypassing the VPN via so fallback).
func (a *AwgOutboundController) status(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "awg-outbound: bad id", err)
		return
	}
	o, err := a.svc.GetOutbound(id)
	if err != nil {
		jsonMsg(c, "awg-outbound: not found", err)
		return
	}
	ifname := "awgo-" + strconv.Itoa(o.Id)
	age, rx, tx, ok := awg.GetManager().CollectClientTraffic(ifname)
	jsonObj(c, gin.H{
		"up":           ok,
		"handshakeAge": age.Round(time.Second).String(),
		"rx":           rx,
		"tx":           tx,
		"ifname":       ifname,
	}, nil)
}

// test pings a public target through the outbound's "awgo-{Id}" interface
// (ping -I) to prove the tunnel actually carries traffic — a successful
// handshake is necessary but not sufficient. If the interface is down we
// short-circuit with a clear message rather than letting ping fail opaquely.
// IPv6 fall: when the row's Settings contains an IPv6 Address we switch to
// ping6 and an IPv6 target. Ping output parsing is Linux-oriented (the deploy
// target); on other platforms the raw output is still returned for debugging.
func (a *AwgOutboundController) test(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "awg-outbound: bad id", err)
		return
	}
	o, err := a.svc.GetOutbound(id)
	if err != nil {
		jsonMsg(c, "awg-outbound: not found", err)
		return
	}
	ifname := "awgo-" + strconv.Itoa(o.Id)
	if _, _, _, ok := awg.GetManager().CollectClientTraffic(ifname); !ok {
		jsonMsg(c, "interface "+ifname+" is down — traffic bypasses VPN", nil)
		return
	}
	target := "1.1.1.1"
	binName := "ping"
	if strings.Contains(o.Settings, "fd00::") || strings.Contains(o.Settings, "::/128") {
		binName = "ping6"
		target = "2606:4700:4700::1111"
	}
	cmd := exec.Command(binName, "-c", "3", "-W", "2", "-I", ifname, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		jsonMsg(c, "ping failed: "+string(out), err)
		return
	}
	jsonObj(c, gin.H{
		"ok":         true,
		"latency_ms": parsePingLatency(string(out)),
		"raw":        string(out),
	}, nil)
}

// parsePingLatency extracts the avg latency (ms, integer-truncated) from a
// `ping` rtt summary line. Handles both the IPv4 form
//
//	"rtt min/avg/max/mdev = 12.345/14.567/16.789/1.234 ms"
//
// and the IPv6 "round-trip min/avg/max/mdev = ..." form — both contain the
// substring "avg" and use a "/"-separated quartet after "=". Returns 0 when
// no summary line is found (caller still has the raw output).
func parsePingLatency(out string) int {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "avg") {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) < 2 {
			continue
		}
		vals := strings.TrimSpace(parts[1])
		i := strings.Index(vals, "/")
		if i < 0 {
			continue
		}
		rest := vals[i+1:]
		j := strings.Index(rest, "/")
		if j < 0 {
			continue
		}
		avg := rest[:j]
		var ms int
		_, _ = fmt.Sscanf(avg, "%d", &ms)
		return ms
	}
	return 0
}

// parseConf parses a pasted awg-quick .conf and returns ClientSettings so the
// "Paste .conf" UI drawer can autofill the form. Thin HTTP wrapper around
// service.ParseConf (Task 4 exported it directly — no ParseConfExposed alias).
func (a *AwgOutboundController) parseConf(c *gin.Context) {
	var body struct {
		Conf string `json:"conf"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "awg-outbound: invalid body", err)
		return
	}
	s, err := service.ParseConf(body.Conf)
	if err != nil {
		jsonMsg(c, "awg-outbound: parse failed", err)
		return
	}
	jsonObj(c, s, nil)
}
