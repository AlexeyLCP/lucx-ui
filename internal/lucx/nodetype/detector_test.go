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
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true,"obj":{"version":"1.0.0","features":["awg","mtproto","presets","cluster"],"awgVersion":"2.0.1","mtprotoVersion":"1.2.3"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	info, err := DetectNodeType(context.Background(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.NodeType != TypeLucX {
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
	if !info.HasFeature("awg") {
		t.Error("expected HasFeature(awg)")
	}
	if !info.SupportsProtocol("awg") {
		t.Error("expected SupportsProtocol(awg)")
	}
}

func TestDetectNodeType_Vanilla(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	info, err := DetectNodeType(context.Background(), srv.URL, "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.NodeType != TypeVanilla {
		t.Errorf("expected 'vanilla', got '%s'", info.NodeType)
	}
	if info.HasFeature("awg") {
		t.Error("vanilla must not claim awg")
	}
	if info.SupportsProtocol("awg") {
		t.Error("vanilla must not support awg")
	}
	if !info.SupportsProtocol("vless") {
		t.Error("vanilla should allow non-lucx protocols via SupportsProtocol")
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

func TestDetectNodeType_TrailingSlashBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/panel/api/lucx/hello" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true,"obj":{"version":"x","features":["awg"]}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	info, err := DetectNodeType(context.Background(), srv.URL+"/", "t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.NodeType != TypeLucX {
		t.Fatalf("got %s", info.NodeType)
	}
}

func TestFromPanelVersion(t *testing.T) {
	if FromPanelVersion("3.6.0-lucx.113").NodeType != TypeLucX {
		t.Fatal("lucx suffix")
	}
	if FromPanelVersion("3.6.0").NodeType != TypeVanilla {
		t.Fatal("vanilla")
	}
}

func TestToJSONFromJSON_RoundTrip(t *testing.T) {
	orig := &NodeInfo{
		NodeType:   TypeLucX,
		Features:   []string{"awg", "naive"},
		AWGVersion: "v3",
		Version:    "3.6.0-lucx.1",
	}
	got := FromJSON(orig.ToJSON())
	if got.NodeType != TypeLucX {
		t.Fatalf("type %s", got.NodeType)
	}
	if !got.HasFeature("awg") || !got.HasFeature("naive") {
		t.Fatalf("features %#v", got.Features)
	}
	if FromJSON("").NodeType != TypeVanilla {
		t.Fatal("empty")
	}
}

func TestHasFeature_EmptyIsFalse(t *testing.T) {
	n := &NodeInfo{NodeType: TypeLucX}
	if n.HasFeature("awg") {
		t.Fatal("empty features must not grant lucx-only protocols")
	}
}

func TestIsLucXOnlyProtocol(t *testing.T) {
	if !IsLucXOnlyProtocol("awg") || !IsLucXOnlyProtocol("NAIVE") {
		t.Fatal("expected lucx-only")
	}
	if IsLucXOnlyProtocol("vless") || IsLucXOnlyProtocol("mtproto") {
		t.Fatal("vless/mtproto are not lucx-only")
	}
}
