// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package clientwarp holds the subJsonClientWarp setting: Cloudflare WARP
// tunnels that the subscriber's own device opens for selected domains.
//
// The tunnel starts on the client, so WARP geolocates the user's own network
// rather than the server's. A typical use is YouTube from Russia: WARP exits
// there with loc=RU and YouTube serves no ads. Plain WireGuard to Cloudflare is
// filtered on many such networks, so the outbounds carry an xray finalmask
// (noise packets ahead of the handshake) by default.
package clientwarp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// PeerPublicKey is Cloudflare WARP's server key; it is the same for every account.
const PeerPublicKey = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="

const (
	DefaultMTU = 1280
	minMTU     = 1280
	maxMTU     = 1500
)

// DefaultConfig is what the panel switch writes when the feature is enabled.
// The endpoints passed repeated reconnect tests from Russian residential
// networks; 162.159.192.1:2408 (Cloudflare's default) is often filtered there.
var DefaultConfig = Config{
	Domains:   []string{"geosite:youtube"},
	Endpoints: []string{"162.159.192.1:4500", "8.39.214.9:4177"},
}

// Config is the parsed subJsonClientWarp setting.
type Config struct {
	// Domains are routing.rules domain entries taken out of the proxied rules
	// and sent through WARP instead, e.g. "geosite:youtube".
	Domains []string `json:"domains"`
	// Endpoints are WARP peer addresses, one outbound each, behind a leastPing balancer.
	Endpoints []string `json:"endpoints"`
	// MTU of the WARP interface; 0 means DefaultMTU.
	MTU int `json:"mtu,omitempty"`
	// FinalMask replaces the default streamSettings.finalmask when set.
	FinalMask map[string]any `json:"finalmask,omitempty"`
}

// Parse validates a subJsonClientWarp value. A blank value disables the
// feature and yields (nil, nil).
func Parse(raw string) (*Config, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var cfg Config
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(cfg.Domains) == 0 {
		return nil, errors.New("domains must list at least one entry")
	}
	for i, d := range cfg.Domains {
		d = strings.TrimSpace(d)
		if d == "" {
			return nil, fmt.Errorf("domains[%d] is empty", i)
		}
		cfg.Domains[i] = d
	}
	if len(cfg.Endpoints) == 0 {
		return nil, errors.New("endpoints must list at least one host:port")
	}
	for i, ep := range cfg.Endpoints {
		ep = strings.TrimSpace(ep)
		host, port, err := net.SplitHostPort(ep)
		if err != nil || host == "" {
			return nil, fmt.Errorf("endpoints[%d] %q is not host:port", i, ep)
		}
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("endpoints[%d] %q has an invalid port", i, ep)
		}
		cfg.Endpoints[i] = ep
	}
	if cfg.MTU != 0 && (cfg.MTU < minMTU || cfg.MTU > maxMTU) {
		return nil, fmt.Errorf("mtu must be between %d and %d", minMTU, maxMTU)
	}
	return &cfg, nil
}

// EffectiveMTU returns the configured MTU or DefaultMTU.
func (c *Config) EffectiveMTU() int {
	if c.MTU == 0 {
		return DefaultMTU
	}
	return c.MTU
}

// StreamSettings returns the streamSettings for a WARP outbound.
func (c *Config) StreamSettings() map[string]any {
	if c.FinalMask != nil {
		return map[string]any{"finalmask": c.FinalMask}
	}
	return map[string]any{"finalmask": DefaultFinalMask()}
}

// DefaultFinalMask sends a QUIC-Initial-shaped packet and four random junk
// packets ahead of each WireGuard handshake (the AmneziaWG i1 + jc scheme).
func DefaultFinalMask() map[string]any {
	noise := []any{map[string]any{"type": "exp", "packet": "<b 0x" + quicInitialHex + ">", "delay": "5-10"}}
	for range 4 {
		noise = append(noise, map[string]any{"rand": "40-70", "delay": "5-10"})
	}
	return map[string]any{
		"udp": []any{map[string]any{
			"type":     "noise",
			"settings": map[string]any{"reset": "100-120", "noise": noise},
		}},
	}
}

// quicInitialHex is a captured QUIC Initial packet (1250 bytes).
const quicInitialHex = "ce000000010897a297ecc34cd6dd000044d0ec2e2e1ea2991f467ace4222129b5a098823784694b4897b9986ae0b7280135fa85e196d9ad980b150122129ce2a9379531b0fd3e871ca5fdb883c369832f730e272d7b8b74f393f9f0fa43f11e510ecb2219a52984410c204cf875585340c62238e14ad04dff382f2c200e0ee22fe743b9c6b8b043121c5710ec289f471c91ee414fca8b8be8419ae8ce7ffc53837f6ade262891895f3f4cecd31bc93ac5599e18e4f01b472362b8056c3172b513051f8322d1062997ef4a383b01706598d08d48c221d30e74c7ce000cdad36b706b1bf9b0607c32ec4b3203a4ee21ab64df336212b9758280803fcab14933b0e7ee1e04a7becce3e2633f4852585c567894a5f9efe9706a151b615856647e8b7dba69ab357b3982f554549bef9256111b2d67afde0b496f16962d4957ff654232aa9e845b61463908309cfd9de0a6abf5f425f577d7e5f6440652aa8da5f73588e82e9470f3b21b27b28c649506ae1a7f5f15b876f56abc4615f49911549b9bb39dd804fde182bd2dcec0c33bad9b138ca07d4a4a1650a2c2686acea05727e2a78962a840ae428f55627516e73c83dd8893b02358e81b524b4d99fda6df52b3a8d7a5291326e7ac9d773c5b43b8444554ef5aea104a738ed650aa979674bbed38da58ac29d87c29d387d80b526065baeb073ce65f075ccb56e47533aef357dceaa8293a523c5f6f790be90e4731123d3c6152a70576e90b4ab5bc5ead01576c68ab633ff7d36dcde2a0b2c68897e1acfc4d6483aaaeb635dd63c96b2b6a7a2bfe042f6aed82e5363aa850aace12ee3b1a93f30d8ab9537df483152a5527faca21efc9981b304f11fc95336f5b9637b174c5a0659e2b22e159a9fed4b8e93047371175b1d6d9cc8ab745f3b2281537d1c75fb9451871864efa5d184c38c185fd203de206751b92620f7c369e031d2041e152040920ac2c5ab5340bfc9d0561176abf10a147287ea90758575ac6a9f5ac9f390d0d5b23ee12af583383d994e22c0cf42383834bcd3ada1b3825a0664d8f3fb678261d57601ddf94a8a68a7c273a18c08aa99c7ad8c6c42eab67718843597ec9930457359dfdfbce024afc2dcf9348579a57d8d3490b2fa99f278f1c37d87dad9b221acd575192ffae1784f8e60ec7cee4068b6b988f0433d96d6a1b1865f4e155e9fe020279f434f3bf1bd117b717b92f6cd1cc9bea7d45978bcc3f24bda631a36910110a6ec06da35f8966c9279d130347594f13e9e07514fa370754d1424c0a1545c5070ef9fb2acd14233e8a50bfc5978b5bdf8bc1714731f798d21e2004117c61f2989dd44f0cf027b27d4019e81ed4b5c31db347c4a3a4d85048d7093cf16753d7b0d15e078f5c7a5205dc2f87e330a1f716738dce1c6180e9d02869b5546f1c4d2748f8c90d9693cba4e0079297d22fd61402dea32ff0eb69ebd65a5d0b687d87e3a8b2c42b648aa723c7c7daf37abcc4bb85caea2ee8f55bec20e913b3324ab8f5c3304f820d42ad1b9f2ffc1a3af9927136b4419e1e579ab4c2ae3c776d293d397d575df181e6cae0a4ada5d67ecea171cca3288d57c7bbdaee3befe745fb7d634f70386d873b90c4d6c6596bb65af68f9e5121e67ebf0d89d3c909ceedfb32ce9575a7758ff080724e1ab5d5f43074ecb53a479af21ed03d7b6899c36631c0166f9d47e5e1d4528a5d3d3f744029c4b1c190cbfbad06f5f83f7ad0429fa9a2719c56ffe3783460e166de2d8"
