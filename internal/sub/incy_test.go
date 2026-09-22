// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsIncyClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		ua      string
		xClient string
		want    bool
	}{
		{name: "documented user agent", ua: "INCY/1.9.0/android", want: true},
		{name: "case insensitive", ua: "incy/2.0/linux", want: true},
		{name: "x-client fallback", ua: "okhttp/5.0", xClient: "INCY", want: true},
		{name: "unrelated client", ua: "v2rayNG/1.9", want: false},
		{name: "substring is not enough", ua: "my-incy-client/1.0", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
			ctx.Request.Header.Set("User-Agent", tc.ua)
			if tc.xClient != "" {
				ctx.Request.Header.Set("x-client", tc.xClient)
			}
			if got := IsIncyClient(ctx); got != tc.want {
				t.Fatalf("IsIncyClient() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyIncyHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := IncyConfig{
		Enabled:            true,
		ProfileDescription: "Быстрые серверы",
		SortOrder:          "ping",
		SupportEmail:       "support@example.com",
		AnnounceURL:        "https://example.com/news",
		PremiumURL:         "https://example.com/premium",
		BannerText:         "Акция 50%",
		BannerButtonText:   "Подробнее",
		BannerButtonURL:    "https://example.com/promo",
		BannerBGColor:      "#E53E3E",
		BannerButtonColor:  "#38A169",
		HideURL:            "on",
		HideCheck:          "off",
		PerAppMode:         "proxy",
		PerAppList:         "org.telegram.messenger\ncom.google.android.youtube",
		Fragmentation: IncyFragmentationConfig{
			Mode:     "on",
			Packets:  "tlshello",
			Length:   "10-30",
			Interval: "10-30",
		},
		Noises: IncyNoisesConfig{
			Mode:   "on",
			Type:   "rand",
			Packet: "10-20",
			Delay:  "10-50",
		},
		Resolve: IncyResolveConfig{
			Mode:      "on",
			DNSDomain: "https://dns.example.com/dns-query",
			DNSIP:     "1.1.1.1",
		},
		NoLimit: true,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
	ctx.Request.Header.Set("User-Agent", "INCY/2.0/android")

	ApplyIncyHeaders(ctx, cfg)

	h := recorder.Header()
	wantDescription := "base64:" + base64.StdEncoding.EncodeToString([]byte(cfg.ProfileDescription))
	if got := h.Get("Profile-Description"); got != wantDescription {
		t.Fatalf("Profile-Description = %q, want %q", got, wantDescription)
	}
	if got := h.Get("Sort-Order"); got != "ping" {
		t.Fatalf("Sort-Order = %q, want ping", got)
	}
	if got := h.Get("Per-App-Proxy-Enable"); got != "1" {
		t.Fatalf("Per-App-Proxy-Enable = %q, want 1", got)
	}
	if got := h.Get("Per-App-Proxy-Mode"); got != "proxy" {
		t.Fatalf("Per-App-Proxy-Mode = %q, want proxy", got)
	}
	wantList := "base64:" + base64.StdEncoding.EncodeToString([]byte(cfg.PerAppList))
	if got := h.Get("Per-App-Proxy-List"); got != wantList {
		t.Fatalf("Per-App-Proxy-List = %q, want %q", got, wantList)
	}
	for name, want := range map[string]string{
		"Hide-Url":                          "1",
		"Hide-Check":                        "0",
		"Fragmentation-Enable":              "1",
		"Fragmentation-Packets":             "tlshello",
		"Fragmentation-Length":              "10-30",
		"Fragmentation-Interval":            "10-30",
		"Noises-Enable":                     "1",
		"Noises-Type":                       "rand",
		"Noises-Packet":                     "10-20",
		"Noises-Delay":                      "10-50",
		"Server-Address-Resolve-Enable":     "1",
		"Server-Address-Resolve-Dns-Domain": "https://dns.example.com/dns-query",
		"Server-Address-Resolve-Dns-Ip":     "1.1.1.1",
		"No-Limit-Enabled":                  "1",
		"Premium-Url":                       "https://example.com/premium",
		"Banner-Bg-Color":                   "#E53E3E",
		"Banner-Button-Color":               "#38A169",
	} {
		if got := h.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestApplyIncyHeadersGating(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		cfg  IncyConfig
		ua   string
	}{
		{name: "disabled", cfg: IncyConfig{Enabled: false, HideURL: "on"}, ua: "INCY/2.0/android"},
		{name: "other client", cfg: IncyConfig{Enabled: true, HideURL: "on"}, ua: "Happ/2.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/test", nil)
			ctx.Request.Header.Set("User-Agent", tc.ua)

			ApplyIncyHeaders(ctx, tc.cfg)
			if got := recorder.Header().Get("Hide-Url"); got != "" {
				t.Fatalf("Hide-Url emitted unexpectedly: %q", got)
			}
		})
	}
}
