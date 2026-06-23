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
	"time"
)

// NodeInfo holds the result of node type detection.
type NodeInfo struct {
	NodeType        string   `json:"nodeType"`
	Features        []string `json:"features"`
	AWGVersion      string   `json:"awgVersion"`
	MTProtoVersion  string   `json:"mtprotoVersion"`
}

type lucxHelloResponse struct {
	Success bool `json:"success"`
	Obj     struct {
		Version       string   `json:"version"`
		Features      []string `json:"features"`
		AWGVersion    string   `json:"awgVersion"`
		MTProtoVersion string `json:"mtprotoVersion"`
	} `json:"obj"`
}

// DetectNodeType probes a remote node to determine if it runs LucX-UI or vanilla 3x-ui.
// baseURL should be the panel URL (e.g., "https://5.9.1.2:2053/myBasePath").
func DetectNodeType(ctx context.Context, baseURL string, apiToken string) (*NodeInfo, error) {
	url := baseURL + "/panel/api/lucx/hello"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return &NodeInfo{NodeType: "vanilla"}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var hello lucxHelloResponse
	if err := json.NewDecoder(resp.Body).Decode(&hello); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !hello.Success {
		return nil, fmt.Errorf("lucx/hello returned success=false")
	}

	return &NodeInfo{
		NodeType:       "lucx",
		Features:       hello.Obj.Features,
		AWGVersion:     hello.Obj.AWGVersion,
		MTProtoVersion: hello.Obj.MTProtoVersion,
	}, nil
}

// ToJSON marshals NodeInfo to a JSON string for storage in NodeFeatures.
func (n *NodeInfo) ToJSON() string {
	type featuresJSON struct {
		Features       []string `json:"features"`
		AWGVersion     string   `json:"awgVersion"`
		MTProtoVersion string   `json:"mtprotoVersion"`
	}
	f := featuresJSON{
		Features:       n.Features,
		AWGVersion:     n.AWGVersion,
		MTProtoVersion: n.MTProtoVersion,
	}
	b, _ := json.Marshal(f)
	return string(b)
}

// FromJSON parses a NodeFeatures JSON string back into NodeInfo.
func FromJSON(s string) *NodeInfo {
	type featuresJSON struct {
		Features       []string `json:"features"`
		AWGVersion     string   `json:"awgVersion"`
		MTProtoVersion string   `json:"mtprotoVersion"`
	}
	info := &NodeInfo{NodeType: "vanilla"}
	if s == "" || s == "{}" {
		return info
	}
	var f featuresJSON
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		return info
	}
	info.Features = f.Features
	info.AWGVersion = f.AWGVersion
	info.MTProtoVersion = f.MTProtoVersion
	info.NodeType = "lucx"
	return info
}
