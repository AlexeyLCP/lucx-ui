// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// sub-extender is the reference implementation of the stage 3d sub/after
// seam: the panel POSTs the subscription context to /_lucx/sub/after on the
// plugin's unix socket (LUCX_PLUGIN_SOCKET) and the plugin answers with
// links to append after the client entries. Build it statically for the
// host architecture (see README) and package it as a plugin ZIP;
// "sub": ["after"] in manifest.json selects the seam.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

type subAfterCall struct {
	SubID  string   `json:"subId"`
	Host   string   `json:"host"`
	Count  int      `json:"count"`
	Emails []string `json:"emails"`
}

type subAfterAnswer struct {
	Links []string `json:"links"`
}

func main() {
	sock := os.Getenv("LUCX_PLUGIN_SOCKET")
	if sock == "" {
		fmt.Fprintln(os.Stderr, "LUCX_PLUGIN_SOCKET is not set: this plugin must be started by the panel")
		os.Exit(1)
	}
	dir := filepath.Dir(sock)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "create socket dir:", err)
		os.Exit(1)
	}
	_ = os.Remove(sock)
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", sock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bind", sock, err)
		os.Exit(1)
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "chmod socket:", err)
		ln.Close()
		os.Exit(1)
	}
	pv := os.Getenv("LUCX_PLUGIN_VERSION")
	fmt.Fprintf(os.Stdout, "sub-extender %s listening on %s\n", pv, sock)

	mux := http.NewServeMux()
	mux.HandleFunc("/_lucx/sub/after", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || !json.Valid(b) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var c subAfterCall
		_ = json.Unmarshal(b, &c)
		// A real plugin would tailor links to c.SubID / c.Emails; the
		// reference answers one static line. The panel appends it AFTER the
		// client entries and never rewrites them.
		fmt.Printf("SUB/AFTER subId=%s host=%s count=%d emails=%d\n", c.SubID, c.Host, c.Count, len(c.Emails))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(subAfterAnswer{
			Links: []string{"socks://127.0.0.1:1080#sub-extender-extra"},
		})
	})
	serveErr := make(chan error, 1)
	go func() { serveErr <- http.Serve(ln, mux) }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		_ = ln.Close()
	case err := <-serveErr:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			fmt.Fprintln(os.Stderr, "serve:", err)
		}
	}
	fmt.Fprintln(os.Stdout, "sub-extender stopping")
}
