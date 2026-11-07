// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/cleanip"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// CleanIpController exposes the on-demand Cloudflare edge scanner. Routes
// live under /panel/api/cleanip/* and share the panel API auth + CSRF
// middleware.
type CleanIpController struct {
	svc *service.CleanIpService
}

// NewCleanIpController creates a new controller bound to the given gin group
// and registers its routes.
func NewCleanIpController(g *gin.RouterGroup) *CleanIpController {
	a := &CleanIpController{svc: &service.CleanIpService{}}
	g.POST("/scan", a.scan)
	return a
}

// scan runs one synchronous scan batch; the operator waits for results.
func (a *CleanIpController) scan(c *gin.Context) {
	var params cleanip.Params
	if err := c.ShouldBindJSON(&params); err != nil {
		jsonMsg(c, "cleanip: invalid scan body", err)
		return
	}
	res, err := a.svc.Scan(params)
	if err != nil {
		jsonMsg(c, "cleanip: scan failed", err)
		return
	}
	jsonObj(c, res, nil)
}
