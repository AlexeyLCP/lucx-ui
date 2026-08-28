// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package nodetype

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

const (
	TypeLucX    = "lucx"
	TypeVanilla = "vanilla"
)

var DefaultLucXFeatures = []string{
	"awg",
	"mtproto",
	"naive",
	"olcrtc",
	"qwdtt",
	"mieru",
	"trusttunnel",
	"anytls",
	"cluster",
}

var LucXOnlyProtocols = map[string]bool{
	"awg":         true,
	"naive":       true,
	"olcrtc":      true,
	"qwdtt":       true,
	"mieru":       true,
	"trusttunnel": true,
	"anytls":      true,
}

type NodeInfo struct {
	NodeType       string   `json:"nodeType"`
	Features       []string `json:"features"`
	AWGVersion     string   `json:"awgVersion"`
	MTProtoVersion string   `json:"mtprotoVersion"`
	Version        string   `json:"version,omitempty"`
}

type lucxHelloResponse struct {
	Success bool `json:"success"`
	Obj     struct {
		Version        string   `json:"version"`
		Features       []string `json:"features"`
		AWGVersion     string   `json:"awgVersion"`
		MTProtoVersion string   `json:"mtprotoVersion"`
	} `json:"obj"`
}

func IsLucXOnlyProtocol(protocol string) bool {
	return LucXOnlyProtocols[strings.ToLower(strings.TrimSpace(protocol))]
}

func (n *NodeInfo) HasFeature(feature string) bool {
	if n == nil || n.NodeType != TypeLucX {
		return false
	}
	feature = strings.ToLower(strings.TrimSpace(feature))
	if feature == "" {
		return false
	}
	if len(n.Features) == 0 {
		return true
	}
	for _, f := range n.Features {
		if strings.EqualFold(f, feature) {
			return true
		}
	}
	return false
}

func (n *NodeInfo) SupportsProtocol(protocol string) bool {
	if !IsLucXOnlyProtocol(protocol) {
		return true
	}
	return n.HasFeature(protocol)
}

func HelloURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/panel/api/lucx/hello"
}

func DetectNodeType(ctx context.Context, baseURL string, apiToken string) (*NodeInfo, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	return DetectNodeTypeWithClient(ctx, client, baseURL, apiToken)
}

func DetectNodeTypeWithClient(ctx context.Context, client *http.Client, baseURL string, apiToken string) (*NodeInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, HelloURL(baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &NodeInfo{NodeType: TypeVanilla}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var hello lucxHelloResponse
	if err := json.NewDecoder(resp.Body).Decode(&hello); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !hello.Success {
		return nil, fmt.Errorf("lucx/hello returned success=false")
	}

	features := hello.Obj.Features
	if len(features) == 0 {
		features = slices.Clone(DefaultLucXFeatures)
	}

	return &NodeInfo{
		NodeType:       TypeLucX,
		Features:       features,
		AWGVersion:     hello.Obj.AWGVersion,
		MTProtoVersion: hello.Obj.MTProtoVersion,
		Version:        hello.Obj.Version,
	}, nil
}

func FromPanelVersion(panelVersion string) *NodeInfo {
	if strings.Contains(strings.ToLower(panelVersion), "lucx") {
		return &NodeInfo{
			NodeType: TypeLucX,
			Features: slices.Clone(DefaultLucXFeatures),
			Version:  panelVersion,
		}
	}
	return &NodeInfo{NodeType: TypeVanilla}
}

func (n *NodeInfo) ToJSON() string {
	if n == nil {
		return ""
	}
	type featuresJSON struct {
		NodeType       string   `json:"nodeType"`
		Features       []string `json:"features"`
		AWGVersion     string   `json:"awgVersion,omitempty"`
		MTProtoVersion string   `json:"mtprotoVersion,omitempty"`
		Version        string   `json:"version,omitempty"`
	}
	f := featuresJSON{
		NodeType:       n.NodeType,
		Features:       n.Features,
		AWGVersion:     n.AWGVersion,
		MTProtoVersion: n.MTProtoVersion,
		Version:        n.Version,
	}
	if f.NodeType == "" {
		f.NodeType = TypeVanilla
	}
	b, err := json.Marshal(f)
	if err != nil {
		return ""
	}
	return string(b)
}

func FromJSON(s string) *NodeInfo {
	info := &NodeInfo{NodeType: TypeVanilla}
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return info
	}
	type featuresJSON struct {
		NodeType       string   `json:"nodeType"`
		Features       []string `json:"features"`
		AWGVersion     string   `json:"awgVersion"`
		MTProtoVersion string   `json:"mtprotoVersion"`
		Version        string   `json:"version"`
	}
	var f featuresJSON
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		return info
	}
	info.Features = f.Features
	info.AWGVersion = f.AWGVersion
	info.MTProtoVersion = f.MTProtoVersion
	info.Version = f.Version
	switch strings.ToLower(strings.TrimSpace(f.NodeType)) {
	case TypeLucX:
		info.NodeType = TypeLucX
	case TypeVanilla, "":
		if len(f.Features) > 0 {
			info.NodeType = TypeLucX
		} else {
			info.NodeType = TypeVanilla
		}
	default:
		info.NodeType = TypeVanilla
	}
	return info
}
