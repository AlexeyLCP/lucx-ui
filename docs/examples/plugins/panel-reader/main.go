// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// panel-reader is the reference implementation of the stage 3c panel API
// seam: the panel grants the plugin a scoped bearer token in
// LUCX_PANEL_TOKEN and the loopback base URL in LUCX_PANEL_URL; the plugin
// calls the API within its manifest's panelApi matrix. Build it statically
// (see README) and install with `bot`/`panelApi` permissions.
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	url := os.Getenv("LUCX_PANEL_URL")
	token := os.Getenv("LUCX_PANEL_TOKEN")
	if url == "" || token == "" {
		fmt.Fprintln(os.Stderr, "LUCX_PANEL_URL / LUCX_PANEL_TOKEN are not set: this plugin must be started by the panel with panelApi permissions")
		os.Exit(1)
	}
	if tokenBot := os.Getenv("LUCX_TELEGRAM_BOT_TOKEN"); tokenBot != "" {
		fmt.Println("TELEGRAM_TOKEN delivered (length", strconv.Itoa(len(tokenBot)), ")")
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		// The panel speaks TLS with the operator's own certificate (typically
		// self-signed); plugins verify nothing on this loopback connection.
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}

	count := 0
	for {
		count++
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
			url+"panel/api/inbounds/list", nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "build request:", err)
			os.Exit(1)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		resp, err := client.Do(req)
		if err == nil {
			var body struct {
				Success bool `json:"success"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			fmt.Printf("inbounds/list -> HTTP %d success=%v (call %d)\n", resp.StatusCode, body.Success, count)
			if resp.StatusCode == http.StatusOK {
				return
			}
		} else {
			fmt.Fprintln(os.Stderr, "call failed:", err)
		}
		time.Sleep(10 * time.Second)
	}
}
