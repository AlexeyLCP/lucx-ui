// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/scanguard"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ScanguardController exposes the scanner guard (RKN scanner subnet ipset
// banning): status, config save+apply and a manual refresh button. Routes
// live under /panel/api/scanguard/* and share the panel API auth + CSRF
// middleware.
type ScanguardController struct {
	svc *service.ScanguardService
}

// NewScanguardController creates a new controller bound to the given gin
// group and registers its routes.
func NewScanguardController(g *gin.RouterGroup) *ScanguardController {
	a := &ScanguardController{svc: &service.ScanguardService{}}
	a.initRouter(g)
	return a
}

func (a *ScanguardController) initRouter(g *gin.RouterGroup) {
	g.GET("/status", a.status)
	g.POST("/config", a.save)
	g.POST("/refresh", a.refresh)
}

func (a *ScanguardController) status(c *gin.Context) {
	jsonObj(c, a.svc.Status(), nil)
}

func (a *ScanguardController) save(c *gin.Context) {
	var cfg scanguard.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "scanguard: invalid config body", err)
		return
	}
	st, err := a.svc.SaveAndApply(cfg)
	if err != nil {
		jsonMsg(c, "scanguard: save failed", err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.scanguard.toasts.saved"), st, nil)
}

func (a *ScanguardController) refresh(c *gin.Context) {
	st, err := a.svc.ForceRefresh()
	if err != nil {
		jsonMsg(c, "scanguard: refresh failed", err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.scanguard.toasts.refreshed"), st, nil)
}
