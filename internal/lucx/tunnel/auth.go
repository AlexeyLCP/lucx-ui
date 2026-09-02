// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// scopedClientAuth deterministically derives per-client credentials for one
// tunnel core (scope = core name) and inbound, mirroring naive's ClientAuth.
// The scope keeps credential namespaces disjoint: the same email on a mieru
// and a naive inbound of one panel must not share secrets.
func scopedClientAuth(secret []byte, scope string, inboundId int, email string) AuthPair {
	id := fmt.Sprintf("%d:%s", inboundId, strings.TrimSpace(email))
	userMac := hmac.New(sha256.New, secret)
	userMac.Write([]byte("lucx-" + scope + "-user:" + id))
	passMac := hmac.New(sha256.New, secret)
	passMac.Write([]byte("lucx-" + scope + "-pass:" + id))
	return AuthPair{
		User: scope[:2] + hex.EncodeToString(userMac.Sum(nil))[:10],
		Pass: base64.RawURLEncoding.EncodeToString(passMac.Sum(nil))[:27],
	}
}

// MieruClientAuth derives the mieru user/password pair of one panel client
// for one mieru inbound (nothing stored; re-derived by the config renderer
// and the share-link builder).
func MieruClientAuth(secret []byte, inboundId int, email string) AuthPair {
	return scopedClientAuth(secret, "mieru", inboundId, email)
}

// TrustTunnelClientAuth derives the TrustTunnel username/password pair of one
// panel client for one TrustTunnel inbound.
func TrustTunnelClientAuth(secret []byte, inboundId int, email string) AuthPair {
	return scopedClientAuth(secret, "trusttunnel", inboundId, email)
}

// UsesDerivedAuth is naive / mieru / TrustTunnel: HMAC pairs, not stored passwords.
func UsesDerivedAuth(p model.Protocol) bool {
	return p == model.Naive || p == model.Mieru || p == model.TrustTunnel
}

// AuthSeed reads the per-inbound HMAC key from settings JSON (empty = panel secret + id).
func AuthSeed(settings string) string {
	var s struct {
		AuthSeed string `json:"authSeed"`
	}
	_ = json.Unmarshal([]byte(settings), &s)
	return strings.TrimSpace(s.AuthSeed)
}

// SetAuthSeed writes authSeed without dropping other settings keys.
func SetAuthSeed(settings, seed string) string {
	var m map[string]any
	if raw := strings.TrimSpace(settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	m["authSeed"] = seed
	bs, err := json.Marshal(m)
	if err != nil {
		return settings
	}
	return string(bs)
}

// EnsureAuthSeed mints a seed when missing. changed=false leaves settings as-is.
func EnsureAuthSeed(settings string) (string, bool) {
	if AuthSeed(settings) != "" {
		return settings, false
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return settings, false
	}
	return SetAuthSeed(settings, hex.EncodeToString(b)), true
}

// PreserveAuthSeed copies old seed into new settings when the form stripped it.
func PreserveAuthSeed(oldSettings, newSettings string) string {
	if AuthSeed(newSettings) != "" {
		return newSettings
	}
	if seed := AuthSeed(oldSettings); seed != "" {
		return SetAuthSeed(newSettings, seed)
	}
	return newSettings
}

// InboundAuthPair is the pair the sidecar and the share link must both emit.
// authSeed in settings (synced on node push) wins so master sub matches the node.
// Empty seed = HMAC(panel secret, inbound id) — standalone / pre-seed rows.
func InboundAuthPair(secret []byte, ib *model.Inbound, email string) AuthPair {
	if ib == nil {
		return AuthPair{}
	}
	key, id := secret, ib.Id
	if seed := AuthSeed(ib.Settings); seed != "" {
		key, id = []byte(seed), 0
	}
	switch ib.Protocol {
	case model.Naive:
		return ClientAuthForInbound(key, id, email)
	case model.Mieru:
		return MieruClientAuth(key, id, email)
	case model.TrustTunnel:
		return TrustTunnelClientAuth(key, id, email)
	}
	return AuthPair{}
}
