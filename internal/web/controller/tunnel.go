// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"os"
	"runtime"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// TunnelController exposes the external tunnel sidecars (NaiveProxy caddy):
// config CRUD, lifecycle, logs, Caddyfile preview/validation and binary
// management. Routes live under /panel/api/tunnel/* and share the panel API
// auth + CSRF middleware.
type TunnelController struct {
	svc *service.TunnelService
}

// NewTunnelController creates a new TunnelController bound to the given gin
// group and registers its routes.
func NewTunnelController(g *gin.RouterGroup) *TunnelController {
	a := &TunnelController{svc: &service.TunnelService{}}
	a.initRouter(g)
	return a
}

func (a *TunnelController) initRouter(g *gin.RouterGroup) {
	naive := g.Group("/naive")
	naive.GET("/status", a.status)
	naive.GET("/config", a.getConfig)
	naive.POST("/config", a.saveConfig)
	naive.POST("/start", a.start)
	naive.POST("/stop", a.stop)
	naive.POST("/restart", a.restart)
	naive.GET("/logs", a.logs)
	naive.POST("/preview", a.preview)
	naive.POST("/validate", a.validate)
	naive.POST("/upload", a.uploadBinary)
	naive.POST("/download", a.downloadBinary)
	naive.POST("/deleteBinary", a.deleteBinary)
}

func (a *TunnelController) status(c *gin.Context) {
	st, err := a.svc.NaiveStatus()
	if err != nil {
		jsonMsg(c, "tunnel: status failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) getConfig(c *gin.Context) {
	cfg, err := a.svc.LoadNaiveConfig()
	if err != nil {
		jsonMsg(c, "tunnel: config load failed", err)
		return
	}
	jsonObj(c, cfg, nil)
}

func (a *TunnelController) saveConfig(c *gin.Context) {
	cfg := tunnel.DefaultNaiveConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "tunnel: invalid config body", err)
		return
	}
	if err := a.svc.SaveNaiveConfig(cfg); err != nil {
		jsonMsg(c, "tunnel: config save failed", err)
		return
	}
	st, err := a.svc.NaiveStatus()
	if err != nil {
		jsonMsg(c, "tunnel: status after save failed", err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *TunnelController) start(c *gin.Context) {
	err := a.svc.StartNaive()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.started"), err)
}

func (a *TunnelController) stop(c *gin.Context) {
	err := a.svc.StopNaive()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.stopped"), err)
}

func (a *TunnelController) restart(c *gin.Context) {
	err := a.svc.RestartNaive()
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.restarted"), err)
}

func (a *TunnelController) logs(c *gin.Context) {
	lines := 200
	if n := c.Query("lines"); n != "" {
		if parsed, err := strconv.Atoi(n); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	jsonObj(c, a.svc.NaiveLogs(lines), nil)
}

func (a *TunnelController) preview(c *gin.Context) {
	cfg := tunnel.DefaultNaiveConfig()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonMsg(c, "tunnel: invalid preview body", err)
		return
	}
	text, err := a.svc.PreviewNaive(cfg)
	if err != nil {
		jsonMsg(c, "tunnel: preview failed", err)
		return
	}
	jsonObj(c, gin.H{"caddyfile": text}, nil)
}

func (a *TunnelController) validate(c *gin.Context) {
	var body struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid validate body", err)
		return
	}
	if err := a.svc.ValidateCaddyfile(body.Text); err != nil {
		jsonMsg(c, "tunnel: Caddyfile invalid", err)
		return
	}
	jsonObj(c, gin.H{"valid": true}, nil)
}

// uploadBinary replaces the core binary on disk (multipart field "file").
// The route is exempt from the global body limit in web.go because caddy
// builds are ~50 MB.
func (a *TunnelController) uploadBinary(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		jsonMsg(c, "tunnel: upload failed", err)
		return
	}
	dst := tunnel.Naive.BinaryPath()
	if err := c.SaveUploadedFile(file, dst); err != nil {
		logger.Warning("tunnel: save uploaded binary failed:", err)
		jsonMsg(c, "tunnel: upload failed", err)
		return
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dst, 0o755); err != nil {
			logger.Warning("tunnel: chmod uploaded binary failed:", err)
		}
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.uploaded"), nil)
}

func (a *TunnelController) downloadBinary(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "tunnel: invalid download body", err)
		return
	}
	if err := a.svc.DownloadBinary(body.URL); err != nil {
		jsonMsg(c, "tunnel: download failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.downloaded"), nil)
}

func (a *TunnelController) deleteBinary(c *gin.Context) {
	if err := a.svc.DeleteBinary(); err != nil {
		jsonMsg(c, "tunnel: binary delete failed", err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.tunnels.naive.toasts.deleted"), nil)
}
