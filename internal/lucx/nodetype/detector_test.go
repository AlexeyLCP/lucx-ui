// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package nodetype

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectNodeType_LucX(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/panel/api/lucx/hello" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(`{"success":true,"obj":{"version":"1.0.0","features":["awg","mtproto","presets","cluster"],"awgVersion":"2.0.1","mtprotoVersion":"1.2.3"}}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	info, err := DetectNodeType(context.Background(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.NodeType != "lucx" {
		t.Errorf("expected 'lucx', got '%s'", info.NodeType)
	}
	if len(info.Features) != 4 {
		t.Errorf("expected 4 features, got %d", len(info.Features))
	}
	if info.AWGVersion != "2.0.1" {
		t.Errorf("expected awgVersion '2.0.1', got '%s'", info.AWGVersion)
	}
	if info.MTProtoVersion != "1.2.3" {
		t.Errorf("expected mtprotoVersion '1.2.3', got '%s'", info.MTProtoVersion)
	}
}

func TestDetectNodeType_Vanilla(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	info, err := DetectNodeType(context.Background(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.NodeType != "vanilla" {
		t.Errorf("expected 'vanilla', got '%s'", info.NodeType)
	}
}

func TestDetectNodeType_ConnectionRefused(t *testing.T) {
	info, err := DetectNodeType(context.Background(), "http://127.0.0.1:19999", "token")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if info != nil {
		t.Error("expected nil info on error")
	}
}
