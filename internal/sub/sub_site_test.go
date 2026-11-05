// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSubDomainOrSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Site", "1")
		_, _ = w.Write([]byte("site:" + r.Host + r.URL.Path))
	}))
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	site := httputil.NewSingleHostReverseProxy(u)

	engine := gin.New()
	engine.Use(subDomainOrSite("sub.example", site))
	engine.GET("/sub/:id", func(c *gin.Context) { c.String(200, "subscription") })
	engine.NoRoute(func(c *gin.Context) { site.ServeHTTP(c.Writer, c.Request) })

	cases := []struct {
		host, path, want string
	}{
		{"sub.example", "/sub/abc", "subscription"},
		{"sub.example", "/", "site:sub.example/"},
		{"203.0.113.8", "/sub/abc", ""},
		{"203.0.113.8:443", "/", ""},
		{"[2001:db8::1]", "/", ""},
		{"other.example:443", "/about.html", "site:other.example:443/about.html"},
	}
	front := httptest.NewServer(engine)
	defer front.Close()
	for _, c := range cases {
		req, _ := http.NewRequest(http.MethodGet, front.URL+c.path, nil)
		req.Host = c.host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if got := string(body); got != c.want {
			t.Errorf("%s%s: got %q, want %q", c.host, c.path, got, c.want)
		}
	}
}
