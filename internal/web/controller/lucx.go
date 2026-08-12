// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/nodetype"
)

type LucxController struct {
	BaseController
}

func NewLucxController(g *gin.RouterGroup) *LucxController {
	a := &LucxController{}
	a.initRouter(g)
	return a
}

func (a *LucxController) initRouter(g *gin.RouterGroup) {
	g.GET("/hello", a.hello)
}

func (a *LucxController) hello(c *gin.Context) {
	jsonObj(c, nodetype.LocalHello(), nil)
}
