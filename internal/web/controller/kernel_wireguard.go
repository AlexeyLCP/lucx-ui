// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	kernelwg "github.com/mhsanaei/3x-ui/v3/internal/lucx/wireguard"
)

type kernelWireguardImportRequest struct {
	Config     string `json:"config"`
	TproxyPort int    `json:"tproxyPort"`
}

func (a *InboundController) kernelWireguardImport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var req kernelWireguardImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid WireGuard import request", err)
		return
	}
	preview, err := kernelwg.ImportConfig(req.Config, req.TproxyPort)
	jsonObj(c, preview, err)
}
