// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/plugin"
)

func RegisterPluginRoutes(g *gin.RouterGroup) {
	g.GET("/list", pluginList)
	g.GET("/:id/status", pluginStatus)
	g.GET("/:id/logs", pluginLogs)
	g.POST("/install", pluginInstall)
	g.POST("/:id/start", pluginStart)
	g.POST("/:id/stop", pluginStop)
	g.POST("/:id/restart", pluginRestart)
	g.POST("/:id/delete", pluginDelete)
}

func pluginList(c *gin.Context) {
	v, err := plugin.List()
	if err != nil {
		jsonMsg(c, "plugin list failed", err)
		return
	}
	jsonObj(c, v, nil)
}

func pluginStatus(c *gin.Context) {
	v, err := plugin.Get(c.Param("id"))
	if err != nil {
		jsonMsg(c, "plugin status failed", err)
		return
	}
	jsonObj(c, v, nil)
}

func pluginLogs(c *gin.Context) {
	lines := 200
	if n, err := strconv.Atoi(c.Query("lines")); err == nil && n > 0 {
		lines = n
	}
	v, err := plugin.Logs(c.Param("id"), lines)
	if err != nil {
		jsonMsg(c, "plugin logs failed", err)
		return
	}
	jsonObj(c, v, nil)
}

func pluginInstall(c *gin.Context) {
	f, err := c.FormFile("file")
	if err != nil {
		jsonMsg(c, "plugin package required", err)
		return
	}
	src, err := f.Open()
	if err != nil {
		jsonMsg(c, "plugin package open failed", err)
		return
	}
	defer src.Close()
	m, err := plugin.InstallPackage(src, f.Size)
	if err != nil {
		jsonMsg(c, "plugin install failed", err)
		return
	}
	jsonObj(c, m, nil)
}

func pluginStart(c *gin.Context) {
	if err := plugin.Start(c.Param("id")); err != nil {
		jsonMsg(c, "plugin start failed", err)
		return
	}
	jsonMsg(c, "plugin started", nil)
}

func pluginStop(c *gin.Context) {
	if err := plugin.Stop(c.Param("id")); err != nil {
		jsonMsg(c, "plugin stop failed", err)
		return
	}
	jsonMsg(c, "plugin stopped", nil)
}

func pluginRestart(c *gin.Context) {
	if err := plugin.Restart(c.Param("id")); err != nil {
		jsonMsg(c, "plugin restart failed", err)
		return
	}
	jsonMsg(c, "plugin restarted", nil)
}

func pluginDelete(c *gin.Context) {
	if err := plugin.Remove(c.Param("id")); err != nil {
		jsonMsg(c, "plugin delete failed", err)
		return
	}
	jsonMsg(c, "plugin deleted", nil)
}
