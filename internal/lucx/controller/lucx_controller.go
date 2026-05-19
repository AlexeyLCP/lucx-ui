// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/outbound_link"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/parser"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/telemt"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"fmt"
)

// LucXController handles LucX-specific API endpoints.
type LucXController struct {
	NodeService    *service.NodeService
	InboundService *service.InboundService
	// LUCX-HOOK: XrayService for AWG restart
	XrayService *service.XrayService
	// END LUCX-HOOK
}

// NewLucXController creates a new LucX controller.
func NewLucXController(nodeSvc *service.NodeService, inboundSvc *service.InboundService, xraySvc *service.XrayService) *LucXController {
	return &LucXController{
		NodeService:    nodeSvc,
		InboundService: inboundSvc,
		XrayService:    xraySvc,
	}
}

// RegisterRoutes registers LucX routes on the given Gin router group.
func (c *LucXController) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/hello", c.Hello)
	g.POST("/parse-ssh", c.ParseSSH)
	g.POST("/inbound-to-outbound", c.InboundToOutbound)

	// LUCX-HOOK: AWG endpoints
	awgGroup := g.Group("/awg")
	awgGroup.POST("/create", c.CreateAWG)
	awgGroup.POST("/delete", c.DeleteAWG)
	awgGroup.POST("/add-client", c.AddAWGClient)
	awgGroup.POST("/del-client", c.DeleteAWGClient)
	awgGroup.GET("/prerequisites", c.AWGPrerequisites)
	// END LUCX-HOOK

	// LUCX-HOOK: Telemt endpoints
	telemtGroup := g.Group("/telemt")
	telemtGroup.POST("/create", c.CreateTelemt)
	telemtGroup.POST("/delete", c.DeleteTelemt)
	telemtGroup.GET("/status/:id", c.TelemtStatus)
	telemtGroup.GET("/link/:id", c.TelemtLink)
	telemtGroup.GET("/version", c.TelemtVersion)
	// END LUCX-HOOK
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

// LUCX-HOOK: AWG handler methods

type createAWGRequest struct {
	Inbound model.Inbound `json:"inbound"`
}

// GenerateAWGKeys returns proper Curve25519 keypair generated via awg genkey/pubkey.
func (c *LucXController) GenerateAWGKeys(ctx *gin.Context) {
	privKey := awg.GenKey()
	pubKey := awg.DerivePubkey(privKey)
	psk := awg.GenPSK()
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"privateKey": privKey,
			"publicKey":  pubKey,
			"psk":        psk,
		},
	})
}

func (c *LucXController) CreateAWG(ctx *gin.Context) {
	var req createAWGRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	awgMgr := awg.NewAWGManager(c.InboundService, c.XrayService)
	inbound, err := awgMgr.Create(&req.Inbound)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "obj": inbound})
}

type deleteAWGRequest struct {
	ID int `json:"id"`
}

func (c *LucXController) DeleteAWG(ctx *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": fmt.Sprintf("panic: %v", r)})
		}
	}()
	var req deleteAWGRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	awgMgr := awg.NewAWGManager(c.InboundService, c.XrayService)
	if err := awgMgr.Delete(req.ID); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

func (c *LucXController) AddAWGClient(ctx *gin.Context) {
	var req struct {
		AWGID  int          `json:"awgId"`
		Client model.Client `json:"client"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	awgMgr := awg.NewAWGManager(c.InboundService, c.XrayService)
	if err := awgMgr.AddClient(req.AWGID, &req.Client); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

func (c *LucXController) DeleteAWGClient(ctx *gin.Context) {
	var req struct {
		AWGID     int    `json:"awgId"`
		PublicKey string `json:"publicKey"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	awgMgr := awg.NewAWGManager(c.InboundService, c.XrayService)
	if err := awgMgr.DeleteClient(req.AWGID, req.PublicKey); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

func (c *LucXController) AWGPrerequisites(ctx *gin.Context) {
	pre := awg.CheckPrerequisites()
	ctx.JSON(http.StatusOK, gin.H{"success": pre.OK(), "obj": pre})
}

// END LUCX-HOOK

// LUCX-HOOK: Telemt handler methods

func (c *LucXController) CreateTelemt(ctx *gin.Context) {
	var req struct {
		Inbound model.Inbound `json:"inbound"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	svc := telemt.NewTelemtService(c.InboundService, c.XrayService)
	inbound, err := svc.CreateTelemtInbound(&req.Inbound)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "obj": inbound})
}

func (c *LucXController) DeleteTelemt(ctx *gin.Context) {
	var req struct{ ID int `json:"id"` }
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	svc := telemt.NewTelemtService(c.InboundService, c.XrayService)
	if err := svc.DeleteTelemtInbound(req.ID); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

func (c *LucXController) TelemtLink(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	inbound, err := c.InboundService.GetInbound(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"success": false, "msg": "inbound not found"})
		return
	}
	// Parse first client from settings
	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	clients, _ := settings["clients"].([]interface{})
	if len(clients) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "no clients"})
		return
	}
	firstClient := clients[0].(map[string]interface{})
	secret, _ := firstClient["secret"].(string)
	host := ctx.Request.Host
	if colon := strings.LastIndex(host, ":"); colon != -1 {
		host = host[:colon]
	}
	link := telemt.GenerateProxyLink(host, inbound.Port, secret)
	ctx.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"link": link}})
}

func (c *LucXController) TelemtStatus(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	mgr := telemt.NewTelemtManager()
	if err := mgr.Healthcheck(9090 + id); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"status": "offline"}})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"status": "online"}})
}

func (c *LucXController) TelemtVersion(ctx *gin.Context) {
	mgr := telemt.NewTelemtManager()
	ver, err := mgr.EnsureBinary()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"version": ver}})
}

// END LUCX-HOOK
