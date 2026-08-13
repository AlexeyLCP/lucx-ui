// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
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
