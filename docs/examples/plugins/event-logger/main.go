// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// event-logger is the reference implementation of the stage 3b events
// contract: the plugin binds the unix socket the panel gives it in
// LUCX_PLUGIN_SOCKET and answers POST /_lucx/events with 2xx. Build it
// statically for the host architecture (see README) and package it as a
// plugin ZIP; `events` in manifest.json selects the subscriptions.
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
	"syscall"
)

type event struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Source string          `json:"source"`
	Time   string          `json:"time"`
	Data   json.RawMessage `json:"data"`
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
	fmt.Fprintf(os.Stdout, "event-logger %s listening on %s\n", pv, sock)

	mux := http.NewServeMux()
	mux.HandleFunc("/_lucx/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var e event
		if json.Valid(b) {
			_ = json.Unmarshal(b, &e)
			fmt.Printf("EVENT %s source=%s data=%s\n", e.Type, e.Source, e.Data)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	serveErr := make(chan error, 1)
	go func() { serveErr <- http.Serve(ln, mux) }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		_ = ln.Close()
	case err := <-serveErr:
		if err != nil {
			fmt.Fprintln(os.Stderr, "serve:", err)
		}
	}
	fmt.Fprintln(os.Stdout, "event-logger stopping")
}
