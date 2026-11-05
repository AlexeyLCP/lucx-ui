// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"strings"
)

// TrustCoverForwardedFor returns stream with sockopt.trustedXForwardedFor =
// ["X-Forwarded-For"] for an Xray inbound that only a local HTTP front can
// reach: listening on loopback, an HTTP-based transport, no TLS of its own.
// That front is a Cover site or the SNI gateway, whose Caddy overwrites the
// client's X-Forwarded-For with the real address, so without this the
// inbound (Devices column, IP limit, logs) sees every client as 127.0.0.1.
// Applied when the Xray config is built, never stored; an explicit
// trustedXForwardedFor (even empty) is left alone.
func TrustCoverForwardedFor(listen, stream string) (string, bool) {
	if !IsLoopbackListen(listen) {
		return stream, false
	}
	netw, sec := parseStream(stream)
	if !isHTTPTransport(netw) {
		return stream, false
	}
	if sec != "" && sec != "none" {
		return stream, false
	}
	var m map[string]any
	if json.Unmarshal([]byte(stream), &m) != nil || m == nil {
		return stream, false
	}
	sock, _ := m["sockopt"].(map[string]any)
	if sock == nil {
		sock = map[string]any{}
	}
	if _, set := sock["trustedXForwardedFor"]; set {
		return stream, false
	}
	sock["trustedXForwardedFor"] = []string{"X-Forwarded-For"}
	m["sockopt"] = sock
	b, err := json.Marshal(m)
	if err != nil {
		return stream, false
	}
	return string(b), true
}

// isHTTPTransport is the set TrustCoverForwardedFor applies to.
func isHTTPTransport(netw string) bool {
	return strings.Contains(" ws httpupgrade xhttp splithttp grpc ", " "+netw+" ")
}
