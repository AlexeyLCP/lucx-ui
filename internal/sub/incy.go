// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	incyUserAgentRegex = regexp.MustCompile(`(?i)^incy(?:/|$)`)
	incyHexColorRegex  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

type IncyConfig struct {
	Enabled            bool                    `json:"enabled"`
	ProfileDescription string                  `json:"profileDescription,omitempty"`
	SortOrder          string                  `json:"sortOrder,omitempty"`
	SupportEmail       string                  `json:"supportEmail,omitempty"`
	AnnounceURL        string                  `json:"announceUrl,omitempty"`
	PremiumURL         string                  `json:"premiumUrl,omitempty"`
	BannerText         string                  `json:"bannerText,omitempty"`
	BannerButtonText   string                  `json:"bannerButtonText,omitempty"`
	BannerButtonURL    string                  `json:"bannerButtonUrl,omitempty"`
	BannerBGColor      string                  `json:"bannerBgColor,omitempty"`
	BannerButtonColor  string                  `json:"bannerButtonColor,omitempty"`
	HideURL            string                  `json:"hideUrl,omitempty"`
	HideCheck          string                  `json:"hideCheck,omitempty"`
	PerAppMode         string                  `json:"perAppMode,omitempty"`
	PerAppList         string                  `json:"perAppList,omitempty"`
	Fragmentation      IncyFragmentationConfig `json:"fragmentation,omitempty"`
	Noises             IncyNoisesConfig        `json:"noises,omitempty"`
	Resolve            IncyResolveConfig       `json:"resolve,omitempty"`
	NoLimit            bool                    `json:"noLimit,omitempty"`
}

type IncyFragmentationConfig struct {
	Mode     string `json:"mode,omitempty"`
	Packets  string `json:"packets,omitempty"`
	Length   string `json:"length,omitempty"`
	Interval string `json:"interval,omitempty"`
}

type IncyNoisesConfig struct {
	Mode   string `json:"mode,omitempty"`
	Type   string `json:"type,omitempty"`
	Packet string `json:"packet,omitempty"`
	Delay  string `json:"delay,omitempty"`
}

type IncyResolveConfig struct {
	Mode      string `json:"mode,omitempty"`
	DNSDomain string `json:"dnsDomain,omitempty"`
	DNSIP     string `json:"dnsIp,omitempty"`
}

func ParseIncyConfig(raw string) (IncyConfig, error) {
	cfg := IncyConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return IncyConfig{}, err
	}
	return cfg, nil
}

func IsIncyClient(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(c.GetHeader("x-client")), "INCY") {
		return true
	}
	return incyUserAgentRegex.MatchString(strings.TrimSpace(c.GetHeader("User-Agent")))
}

func ApplyIncyHeaders(c *gin.Context, cfg IncyConfig) {
	if c == nil || c.Writer == nil || !cfg.Enabled || !IsIncyClient(c) {
		return
	}

	setIncyTextHeader(c, "Profile-Description", cfg.ProfileDescription)
	switch strings.ToLower(strings.TrimSpace(cfg.SortOrder)) {
	case "none", "ping", "name":
		c.Writer.Header().Set("Sort-Order", strings.ToLower(strings.TrimSpace(cfg.SortOrder)))
	}
	setIncyPlainHeader(c, "Support-Email", cfg.SupportEmail)
	setIncyPlainHeader(c, "Announce-Url", cfg.AnnounceURL)
	setIncyPlainHeader(c, "Premium-Url", cfg.PremiumURL)
	setIncyTextHeader(c, "Banner-Text", cfg.BannerText)
	setIncyTextHeader(c, "Banner-Button-Text", cfg.BannerButtonText)
	setIncyPlainHeader(c, "Banner-Button-Url", cfg.BannerButtonURL)
	setIncyColorHeader(c, "Banner-Bg-Color", cfg.BannerBGColor)
	setIncyColorHeader(c, "Banner-Button-Color", cfg.BannerButtonColor)
	setIncyTriStateHeader(c, "Hide-Url", cfg.HideURL)
	setIncyTriStateHeader(c, "Hide-Check", cfg.HideCheck)

	switch strings.ToLower(strings.TrimSpace(cfg.PerAppMode)) {
	case "off":
		c.Writer.Header().Set("Per-App-Proxy-Enable", "0")
	case "proxy", "bypass":
		mode := strings.ToLower(strings.TrimSpace(cfg.PerAppMode))
		c.Writer.Header().Set("Per-App-Proxy-Enable", "1")
		c.Writer.Header().Set("Per-App-Proxy-Mode", mode)
		setIncyTextHeader(c, "Per-App-Proxy-List", cfg.PerAppList)
	}

	applyIncyFragmentationHeaders(c, cfg.Fragmentation)
	applyIncyNoisesHeaders(c, cfg.Noises)
	applyIncyResolveHeaders(c, cfg.Resolve)

	if cfg.NoLimit {
		c.Writer.Header().Set("No-Limit-Enabled", "1")
	}
}

func applyIncyFragmentationHeaders(c *gin.Context, cfg IncyFragmentationConfig) {
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "off":
		c.Writer.Header().Set("Fragmentation-Enable", "0")
	case "on":
		c.Writer.Header().Set("Fragmentation-Enable", "1")
		switch strings.ToLower(strings.TrimSpace(cfg.Packets)) {
		case "tlshello", "1", "1-3", "all":
			c.Writer.Header().Set("Fragmentation-Packets", strings.ToLower(strings.TrimSpace(cfg.Packets)))
		}
		setIncyPlainHeader(c, "Fragmentation-Length", cfg.Length)
		setIncyPlainHeader(c, "Fragmentation-Interval", cfg.Interval)
	}
}

func applyIncyNoisesHeaders(c *gin.Context, cfg IncyNoisesConfig) {
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "off":
		c.Writer.Header().Set("Noises-Enable", "0")
	case "on":
		c.Writer.Header().Set("Noises-Enable", "1")
		switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
		case "rand", "str", "hex":
			c.Writer.Header().Set("Noises-Type", strings.ToLower(strings.TrimSpace(cfg.Type)))
		}
		setIncyPlainHeader(c, "Noises-Packet", cfg.Packet)
		setIncyPlainHeader(c, "Noises-Delay", cfg.Delay)
	}
}

func applyIncyResolveHeaders(c *gin.Context, cfg IncyResolveConfig) {
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "off":
		c.Writer.Header().Set("Server-Address-Resolve-Enable", "0")
	case "on":
		c.Writer.Header().Set("Server-Address-Resolve-Enable", "1")
		setIncyPlainHeader(c, "Server-Address-Resolve-Dns-Domain", cfg.DNSDomain)
		if ip := net.ParseIP(strings.TrimSpace(cfg.DNSIP)); ip != nil {
			c.Writer.Header().Set("Server-Address-Resolve-Dns-Ip", ip.String())
		}
	}
}

func setIncyTriStateHeader(c *gin.Context, name, mode string) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on":
		c.Writer.Header().Set(name, "1")
	case "off":
		c.Writer.Header().Set(name, "0")
	}
}

func setIncyTextHeader(c *gin.Context, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, r := range value {
		if r < 0x20 || r > 0x7e {
			value = "base64:" + base64.StdEncoding.EncodeToString([]byte(value))
			break
		}
	}
	c.Writer.Header().Set(name, value)
}

func setIncyPlainHeader(c *gin.Context, name, value string) {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", ""))
	if value != "" {
		c.Writer.Header().Set(name, value)
	}
}

func setIncyColorHeader(c *gin.Context, name, value string) {
	value = strings.TrimSpace(value)
	if incyHexColorRegex.MatchString(value) {
		c.Writer.Header().Set(name, value)
	}
}
