// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type SidecarOutboundController struct {
	svc         *service.SidecarOutboundService
	xrayService service.XrayService
}

func NewSidecarOutboundController(g *gin.RouterGroup) *SidecarOutboundController {
	a := &SidecarOutboundController{svc: &service.SidecarOutboundService{}}
	a.initRouter(g)
	return a
}

func (a *SidecarOutboundController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.POST("/add", a.add)
	g.POST("/del/:id", a.del)
	g.POST("/update/:id", a.update)
	g.POST("/enable/:id", a.enable)
	g.GET("/status/:id", a.status)
	g.POST("/test/:id", a.test)
	g.POST("/parseLink", a.parseLink)
	g.GET("/binaries", a.binaries)
	g.POST("/upload/:protocol", a.upload)
	g.POST("/deleteBinary/:protocol", a.deleteBinary)
}

type sidecarOutboundRow struct {
	*model.SidecarOutbound
	Status        string `json:"status"`
	BinaryExists  bool   `json:"binaryExists"`
	BinaryMissing bool   `json:"binaryMissing"`
}

func (a *SidecarOutboundController) list(c *gin.Context) {
	outbounds, err := a.svc.GetOutbounds()
	if err != nil {
		jsonMsg(c, "sidecar-outbound: failed to list", err)
		return
	}
	rows := make([]sidecarOutboundRow, 0, len(outbounds))
	m := tunnel.GetManager()
	for _, o := range outbounds {
		core, _ := tunnel.SidecarCore(o.Protocol)
		exists := tunnel.BinaryExists(core)
		r := sidecarOutboundRow{SidecarOutbound: o, BinaryExists: exists, BinaryMissing: !exists}
		if !o.Enable {
			r.Status = "disabled"
		} else if !exists {
			r.Status = "binary missing"
		} else if inst, ok := tunnel.InstanceFromSidecarOutbound(o); ok {
			st := m.StatusOf(inst)
			if st.Listening {
				r.Status = "up"
			} else if st.Running {
				r.Status = "starting"
			} else {
				r.Status = "down"
			}
		} else {
			r.Status = "down"
		}
		rows = append(rows, r)
	}
	jsonObj(c, rows, nil)
}

func (a *SidecarOutboundController) add(c *gin.Context) {
	var o model.SidecarOutbound
	if err := c.ShouldBindJSON(&o); err != nil {
		jsonMsg(c, "sidecar-outbound: invalid body", err)
		return
	}
	out, err := a.svc.AddOutbound(&o)
	if err != nil {
		jsonMsg(c, "sidecar-outbound: add failed", err)
		return
	}
	if err := a.xrayService.RestartXray(true); err != nil {
		logger.Warning("sidecar-outbound: force-restart Xray after add failed:", err)
	}
	jsonObj(c, out, nil)
}

func (a *SidecarOutboundController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sidecar-outbound: bad id", err)
		return
	}
	o, _ := a.svc.GetOutbound(id)
	if err := a.svc.DelOutbound(id); err != nil {
		jsonMsg(c, "sidecar-outbound: del failed", err)
		return
	}
	if o != nil {
		tunnel.GetManager().Remove(tunnel.SidecarManageKey(o.Protocol, o.Id))
	}
	if err := a.xrayService.RestartXray(true); err != nil {
		logger.Warning("sidecar-outbound: force-restart Xray after del failed:", err)
	}
	jsonObj(c, nil, nil)
}

func (a *SidecarOutboundController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sidecar-outbound: bad id", err)
		return
	}
	var o model.SidecarOutbound
	if err := c.ShouldBindJSON(&o); err != nil {
		jsonMsg(c, "sidecar-outbound: invalid body", err)
		return
	}
	o.Id = id
	if err := a.svc.UpdateOutbound(&o); err != nil {
		jsonMsg(c, "sidecar-outbound: update failed", err)
		return
	}
	if err := a.xrayService.RestartXray(true); err != nil {
		logger.Warning("sidecar-outbound: force-restart Xray after update failed:", err)
	}
	jsonObj(c, o, nil)
}

func (a *SidecarOutboundController) enable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sidecar-outbound: bad id", err)
		return
	}
	var body struct {
		Enable bool `json:"enable" form:"enable"`
	}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, "sidecar-outbound: bad request body", err)
		return
	}
	if err := a.svc.SetOutboundEnable(id, body.Enable); err != nil {
		jsonMsg(c, "sidecar-outbound: enable failed", err)
		return
	}
	if !body.Enable {
		if o, err := a.svc.GetOutbound(id); err == nil {
			tunnel.GetManager().Remove(tunnel.SidecarManageKey(o.Protocol, o.Id))
		}
	}
	if err := a.xrayService.RestartXray(true); err != nil {
		logger.Warning("sidecar-outbound: force-restart Xray after enable failed:", err)
	}
	jsonObj(c, nil, nil)
}

func (a *SidecarOutboundController) status(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sidecar-outbound: bad id", err)
		return
	}
	o, err := a.svc.GetOutbound(id)
	if err != nil {
		jsonMsg(c, "sidecar-outbound: not found", err)
		return
	}
	core, _ := tunnel.SidecarCore(o.Protocol)
	up := false
	if inst, ok := tunnel.InstanceFromSidecarOutbound(o); ok {
		up = tunnel.GetManager().StatusOf(inst).Listening
	}
	jsonObj(c, gin.H{
		"up":           up,
		"binaryExists": tunnel.BinaryExists(core),
		"key":          tunnel.SidecarManageKey(o.Protocol, o.Id),
	}, nil)
}

func (a *SidecarOutboundController) test(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sidecar-outbound: bad id", err)
		return
	}
	o, err := a.svc.GetOutbound(id)
	if err != nil {
		jsonMsg(c, "sidecar-outbound: not found", err)
		return
	}
	s, ok := tunnel.ParseSidecarSettings(o)
	if !ok {
		jsonMsg(c, "sidecar-outbound: incomplete settings", nil)
		return
	}
	testURL, _ := (&service.SettingService{}).GetXrayOutboundTestUrl()
	ms, err := a.svc.ProbeHTTP(c.Request.Context(), s.SocksPort, testURL)
	if err != nil {
		jsonMsg(c, "sidecar-outbound: probe failed", err)
		return
	}
	jsonObj(c, gin.H{"ok": true, "latency_ms": ms, "raw": "HTTP via socks :" + strconv.Itoa(s.SocksPort)}, nil)
}

func (a *SidecarOutboundController) parseLink(c *gin.Context) {
	var body struct {
		Link string `json:"link"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "sidecar-outbound: invalid body", err)
		return
	}
	protocol, settings, err := a.svc.ParseLink(body.Link)
	if err != nil {
		jsonMsg(c, "sidecar-outbound: parse failed", err)
		return
	}
	jsonObj(c, gin.H{"protocol": protocol, "settings": settings}, nil)
}

func (a *SidecarOutboundController) binaries(c *gin.Context) {
	jsonObj(c, a.svc.BinaryStatus(), nil)
}

func (a *SidecarOutboundController) upload(c *gin.Context) {
	protocol := c.Param("protocol")
	core, ok := tunnel.SidecarCore(protocol)
	if !ok {
		jsonMsg(c, "sidecar-outbound: unknown protocol", nil)
		return
	}
	dst := core.BinaryPath()
	if err := saveCoreUpload(c, dst); err != nil {
		jsonMsg(c, "sidecar-outbound: upload failed", err)
		return
	}
	jsonObj(c, gin.H{"path": dst}, nil)
}

func (a *SidecarOutboundController) deleteBinary(c *gin.Context) {
	if err := a.svc.DeleteBinary(c.Param("protocol")); err != nil {
		jsonMsg(c, "sidecar-outbound: binary delete failed", err)
		return
	}
	jsonObj(c, nil, nil)
}
