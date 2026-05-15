// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package lucx

import (
	"github.com/gin-gonic/gin"
)

const (
	// APIPrefix is the base path for all LucX API endpoints
	APIPrefix = "/panel/api/lucx"
)

// RegisterRoutes registers all LucX controller routes on the given Gin group.
// Called from web/web.go during router initialization.
func RegisterRoutes(g *gin.RouterGroup) {
	// Routes are registered by the controller package
}
