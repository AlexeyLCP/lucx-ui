// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// subscriptionClientIP is the address stored for a subscription request (HWID
// devices, lucx.344): the socket peer, or — only when that peer is inside
// trustedProxyCIDRs (the setting the login limiter uses) — the visitor address
// the proxy carried in X-Real-IP / X-Forwarded-For. A peer outside the list
// keeps the socket address, so a direct client can never name its own IP.
// X-Real-IP names the visitor here and only here; it is still never a host to
// connect to (#6589, lucx.125).
func subscriptionClientIP(c *gin.Context) string {
	peer := c.RemoteIP()
	cidrs := trustedProxyCIDRsForSub()
	if peer == "" || !remoteAddrInCIDRs(c.Request.RemoteAddr, cidrs) {
		return peer
	}
	realIP, hasRealIP := headerIP(c.GetHeader("X-Real-IP"))
	forwarded, hasForwarded := forwardedForVisitor(c.GetHeader("X-Forwarded-For"), cidrs)
	switch {
	case hasRealIP && hasForwarded && realIP != forwarded:
		// A proxy rewrites at most one of the two, so the other is the client's
		// own claim; fall back to the proxy's socket address.
		return peer
	case hasForwarded:
		return forwarded
	case hasRealIP:
		return realIP
	default:
		return peer
	}
}

// headerIP parses a forwarded address that must be a bare IP (a proxy never
// sends hostnames), unmapped for IPv4-in-IPv6 forms.
func headerIP(value string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	return addr.Unmap().String(), true
}

// forwardedForVisitor walks X-Forwarded-For from the proxy end: entries a client
// sent come first, so the furthest hop back that is a trusted proxy names the
// visitor it saw. The first non-trusted (or non-IP) entry stops the walk.
func forwardedForVisitor(xff, cidrs string) (string, bool) {
	parts := strings.Split(xff, ",")
	client := ""
	for i := len(parts) - 1; i >= 0; i-- {
		ip, ok := headerIP(parts[i])
		if !ok {
			break
		}
		client = ip
		if !remoteAddrInCIDRs(ip, cidrs) {
			break
		}
	}
	return client, client != ""
}

// trustedProxyCIDRsForSub mirrors the panel's controller-side resolution: the
// stored trustedProxyCIDRs, or the shipped default when unset.
func trustedProxyCIDRsForSub() (cidrs string) {
	cidrs = service.DefaultTrustedProxyCIDRs
	defer func() {
		_ = recover()
	}()
	settingService := service.SettingService{}
	if value, err := settingService.GetTrustedProxyCIDRs(); err == nil && strings.TrimSpace(value) != "" {
		cidrs = value
	}
	return cidrs
}
