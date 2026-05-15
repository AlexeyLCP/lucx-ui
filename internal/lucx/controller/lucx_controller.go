// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/outbound_link"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/parser"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

// LucXController handles LucX-specific API endpoints.
type LucXController struct {
	NodeService    *service.NodeService
	InboundService *service.InboundService
}

// NewLucXController creates a new LucX controller.
func NewLucXController(nodeSvc *service.NodeService, inboundSvc *service.InboundService) *LucXController {
	return &LucXController{
		NodeService:    nodeSvc,
		InboundService: inboundSvc,
	}
}

// RegisterRoutes registers LucX routes on the given Gin router group.
func (c *LucXController) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/hello", c.Hello)
	g.POST("/parse-ssh", c.ParseSSH)
	g.POST("/inbound-to-outbound", c.InboundToOutbound)
}

// Hello returns node identity info. Used by master to detect LucX vs Vanilla.
func (c *LucXController) Hello(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"version":       "1.0.0",
			"features":      []string{"cluster"},
			"awgVersion":    "",
			"telemtVersion": "",
		},
	})
}

type parseSSHRequest struct {
	Text string `json:"text"`
}

// ParseSSH parses raw SSH console output and returns connection credentials.
func (c *LucXController) ParseSSH(ctx *gin.Context) {
	var req parseSSHRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":     "Invalid request: " + err.Error(),
		})
		return
	}

	creds, err := parser.ParseSSHOutput(req.Text)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":     err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj":     creds,
	})
}

type inboundToOutboundRequest struct {
	NodeID    int `json:"nodeId"`
	InboundID int `json:"inboundId"`
}

// InboundToOutbound generates an outbound config from a remote inbound.
func (c *LucXController) InboundToOutbound(ctx *gin.Context) {
	var req inboundToOutboundRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":     "Invalid request: " + err.Error(),
		})
		return
	}

	inbound, err := c.InboundService.GetInbound(req.InboundID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"msg":     "Inbound not found: " + err.Error(),
		})
		return
	}

	node, err := c.NodeService.GetById(req.NodeID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"msg":     "Node not found: " + err.Error(),
		})
		return
	}

	result, err := outbound_link.GenerateOutbound(
		string(inbound.Protocol),
		inbound.Tag,
		inbound.Port,
		inbound.Settings,
		inbound.StreamSettings,
		node.Address,
	)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"msg":     err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj":     result,
	})
}
