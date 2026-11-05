// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/clientwarp"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// subJsonClientWarp: per-subscription Cloudflare WARP outbounds for selected
// domains, opened by the client device itself (see package clientwarp).

const (
	clientWarpTagPrefix   = "client-warp"
	clientWarpBalancerTag = "client-warp"
	clientWarpProbeURL    = "https://www.gstatic.com/generate_204"
	clientWarpRetryAfter  = 10 * time.Minute
)

// Package-level so tests can point registration at an httptest server.
var (
	clientWarpRegURL     = "https://api.cloudflareclient.com/v0a2158/reg"
	clientWarpHTTPClient = &http.Client{Timeout: 15 * time.Second}
)

// clientWarpAccounts caches registered accounts and remembers failed
// registrations so an unreachable Cloudflare API is not hit on every request.
type clientWarpAccounts struct {
	regMu  sync.Mutex
	cache  sync.Map // subId -> *model.ClientWarpAccount
	failed sync.Map // subId -> time.Time
}

// SetClientWarpConfig applies the subJsonClientWarp setting; an invalid value
// is logged and leaves the feature off, so a typo cannot break subscriptions.
func (s *SubJsonService) SetClientWarpConfig(raw string) {
	cfg, err := clientwarp.Parse(raw)
	if err != nil {
		logger.Warningf("subJsonClientWarp: %v; client WARP disabled", err)
		cfg = nil
	}
	s.clientWarp = cfg
	s.clientWarpAccounts = &clientWarpAccounts{}
}

// applyClientWarp rewrites every document of one subscription. Without an
// account (registration failed or pending retry) documents are returned as is.
func (s *SubJsonService) applyClientWarp(subId string, configs []json_util.RawMessage) []json_util.RawMessage {
	if s.clientWarp == nil || len(configs) == 0 || subId == "" {
		return configs
	}
	acc := s.clientWarpAccounts.get(subId)
	if acc == nil {
		return configs
	}
	for i := range configs {
		configs[i] = injectClientWarp(configs[i], s.clientWarp, acc)
	}
	return configs
}

func (a *clientWarpAccounts) load(subId string) *model.ClientWarpAccount {
	var rows []model.ClientWarpAccount
	if err := database.GetDB().Where("sub_id = ?", subId).Limit(1).Find(&rows).Error; err != nil {
		logger.Warning("subJsonClientWarp: load account:", err)
		return nil
	}
	if len(rows) == 0 || rows[0].PrivateKey == "" {
		return nil
	}
	acc := &rows[0]
	a.cache.Store(subId, acc)
	return acc
}

func (a *clientWarpAccounts) get(subId string) *model.ClientWarpAccount {
	if v, ok := a.cache.Load(subId); ok {
		return v.(*model.ClientWarpAccount)
	}
	if acc := a.load(subId); acc != nil {
		return acc
	}
	if t, ok := a.failed.Load(subId); ok && time.Since(t.(time.Time)) < clientWarpRetryAfter {
		return nil
	}

	a.regMu.Lock()
	defer a.regMu.Unlock()
	if acc := a.load(subId); acc != nil {
		return acc
	}
	acc, err := registerClientWarp()
	if err != nil {
		a.failed.Store(subId, time.Now())
		logger.Warningf("subJsonClientWarp: WARP registration for sub %s failed: %v", subId, err)
		return nil
	}
	acc.SubId = subId
	if err := database.GetDB().Create(acc).Error; err != nil {
		a.failed.Store(subId, time.Now())
		logger.Warning("subJsonClientWarp: store account:", err)
		return nil
	}
	a.failed.Delete(subId)
	a.cache.Store(subId, acc)
	logger.Infof("subJsonClientWarp: registered a WARP account for sub %s", subId)
	return acc
}

// registerClientWarp creates a free WARP account the way the official client
// does: a fresh X25519 key, POST /reg, then enable the tunnel on the account.
func registerClientWarp() (*model.ClientWarpAccount, error) {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{
		"key":           pub,
		"install_id":    "",
		"fcm_token":     "",
		"tos":           time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"model":         "PC",
		"serial_number": "",
		"locale":        "en_US",
	})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, clientWarpRegURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	setClientWarpHeaders(req)
	resp, err := clientWarpHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var reg struct {
		ID     string `json:"id"`
		Token  string `json:"token"`
		Config struct {
			Interface struct {
				Addresses struct {
					V4 string `json:"v4"`
				} `json:"addresses"`
			} `json:"interface"`
		} `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, err
	}
	if reg.ID == "" || reg.Config.Interface.Addresses.V4 == "" {
		return nil, errors.New("registration response has no account id or address")
	}
	if reg.Token != "" {
		patch, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, clientWarpRegURL+"/"+reg.ID, strings.NewReader(`{"warp_enabled":true}`))
		if err == nil {
			setClientWarpHeaders(patch)
			patch.Header.Set("Authorization", "Bearer "+reg.Token)
			// Best effort: free accounts route traffic without it as well.
			if r, err := clientWarpHTTPClient.Do(patch); err == nil {
				r.Body.Close()
			}
		}
	}
	return &model.ClientWarpAccount{
		PrivateKey: priv,
		AddressV4:  reg.Config.Interface.Addresses.V4,
		AccountId:  reg.ID,
		Token:      reg.Token,
	}, nil
}

func setClientWarpHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("CF-Client-Version", "a-6.11-2223")
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
}

func clientWarpOutbound(tag, endpoint string, cfg *clientwarp.Config, acc *model.ClientWarpAccount) map[string]any {
	return map[string]any{
		"tag":      tag,
		"protocol": "wireguard",
		"settings": map[string]any{
			"secretKey":      acc.PrivateKey,
			"address":        []any{acc.AddressV4 + "/32"},
			"mtu":            cfg.EffectiveMTU(),
			"noKernelTun":    true,
			"domainStrategy": "ForceIPv4",
			"peers": []any{map[string]any{
				"publicKey":  clientwarp.PeerPublicKey,
				"endpoint":   endpoint,
				"allowedIPs": []any{"0.0.0.0/0"},
				"keepAlive":  25,
			}},
		},
		"streamSettings": cfg.StreamSettings(),
	}
}

// Rule keys that do not narrow what a rule matches.
var clientWarpRuleMeta = map[string]bool{"type": true, "domain": true, "outboundTag": true, "balancerTag": true, "ruleTag": true}

// injectClientWarp moves the configured domain entries out of every rule that
// proxies them into a rule of their own right before it, pointing at a WARP
// balancer. The balancer falls back to the outbound the first such rule used,
// so traffic keeps flowing when WARP is unreachable. Entries routed direct or
// blocked are left alone, and a document without any proxied entry is
// returned unchanged.
func injectClientWarp(raw json_util.RawMessage, cfg *clientwarp.Config, acc *model.ClientWarpAccount) json_util.RawMessage {
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		return raw
	}
	routing, _ := doc["routing"].(map[string]any)
	outbounds, _ := doc["outbounds"].([]any)
	if routing == nil || len(outbounds) == 0 {
		return raw
	}
	for _, o := range outbounds {
		if om, _ := o.(map[string]any); om != nil {
			if tag, _ := om["tag"].(string); strings.HasPrefix(tag, clientWarpTagPrefix) {
				return raw
			}
		}
	}
	wanted := make(map[string]bool, len(cfg.Domains))
	for _, d := range cfg.Domains {
		wanted[d] = true
	}
	firstTag := ""
	if first, _ := outbounds[0].(map[string]any); first != nil {
		firstTag, _ = first["tag"].(string)
	}

	rules, _ := routing["rules"].([]any)
	newRules := make([]any, 0, len(rules)+1)
	fallback := ""
	for _, r := range rules {
		rm, ok := r.(map[string]any)
		if !ok {
			newRules = append(newRules, r)
			continue
		}
		domains, _ := rm["domain"].([]any)
		var matched, rest []any
		for _, d := range domains {
			if s, _ := d.(string); wanted[s] {
				matched = append(matched, d)
			} else {
				rest = append(rest, d)
			}
		}
		target := ""
		if ob, _ := rm["outboundTag"].(string); ob != "" {
			if ob != "direct" && ob != "block" {
				target = ob
			}
		} else if bt, _ := rm["balancerTag"].(string); bt != "" {
			// A balancer document: fall back to its first member.
			target = firstTag
		}
		if len(matched) == 0 || target == "" {
			newRules = append(newRules, r)
			continue
		}
		if fallback == "" {
			fallback = target
		}
		newRules = append(newRules, map[string]any{"type": "field", "domain": matched, "balancerTag": clientWarpBalancerTag})
		hasOther := false
		for key := range rm {
			if !clientWarpRuleMeta[key] {
				hasOther = true
				break
			}
		}
		if len(rest) == 0 && !hasOther {
			// Nothing else to match; an emptied rule would match everything.
			continue
		}
		clone := maps.Clone(rm)
		if len(rest) == 0 {
			delete(clone, "domain")
		} else {
			clone["domain"] = rest
		}
		newRules = append(newRules, clone)
	}
	if fallback == "" {
		return raw
	}

	for i, ep := range cfg.Endpoints {
		tag := clientWarpTagPrefix
		if i > 0 {
			tag = fmt.Sprintf("%s-%d", clientWarpTagPrefix, i+1)
		}
		outbounds = append(outbounds, clientWarpOutbound(tag, ep, cfg, acc))
	}
	doc["outbounds"] = outbounds
	routing["rules"] = newRules
	balancers, _ := routing["balancers"].([]any)
	routing["balancers"] = append(balancers, map[string]any{
		"tag":         clientWarpBalancerTag,
		"selector":    []any{clientWarpTagPrefix},
		"fallbackTag": fallback,
		"strategy":    map[string]any{"type": "leastPing"},
	})

	// Xray runs one observer per config: extend the balancer feature's
	// burstObservatory when the document already has one.
	if bo, ok := doc["burstObservatory"].(map[string]any); ok {
		sel, _ := bo["subjectSelector"].([]any)
		bo["subjectSelector"] = append(sel, clientWarpTagPrefix)
	} else if ob, ok := doc["observatory"].(map[string]any); ok {
		sel, _ := ob["subjectSelector"].([]any)
		ob["subjectSelector"] = append(sel, clientWarpTagPrefix)
	} else {
		doc["observatory"] = map[string]any{
			"subjectSelector":   []any{clientWarpTagPrefix},
			"probeUrl":          clientWarpProbeURL,
			"probeInterval":     "1m",
			"enableConcurrency": true,
		}
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return raw
	}
	return json_util.RawMessage(out)
}
