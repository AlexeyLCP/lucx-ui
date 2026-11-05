// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"strings"
	"testing"
)

func TestTrustCoverForwardedFor(t *testing.T) {
	ws := `{"network":"ws","security":"none","wsSettings":{"path":"/x"}}`
	got, ok := TrustCoverForwardedFor("127.0.0.1", ws)
	if !ok || !strings.Contains(got, `"trustedXForwardedFor":["X-Forwarded-For"]`) || !strings.Contains(got, `"path":"/x"`) {
		t.Fatalf("ws behind a Cover: %v %s", ok, got)
	}
	keep := `{"network":"xhttp","sockopt":{"tcpFastOpen":true}}`
	if got, ok := TrustCoverForwardedFor("127.0.0.1", keep); !ok || !strings.Contains(got, `"tcpFastOpen":true`) {
		t.Fatalf("existing sockopt keys must stay: %s", got)
	}
	for name, c := range map[string][2]string{
		"public listen":    {"0.0.0.0", ws},
		"own TLS":          {"127.0.0.1", `{"network":"ws","security":"tls"}`},
		"tcp transport":    {"127.0.0.1", `{"network":"tcp","security":"none"}`},
		"explicit setting": {"127.0.0.1", `{"network":"ws","sockopt":{"trustedXForwardedFor":[]}}`},
		"bad json":         {"127.0.0.1", `{`},
	} {
		if got, ok := TrustCoverForwardedFor(c[0], c[1]); ok || got != c[1] {
			t.Errorf("%s: must be left alone, got %v %s", name, ok, got)
		}
	}
}

// A Cover route must hand its backend only the connection's address: Xray
// trusts X-Forwarded-For there, so a client-chosen value would fake its IP.
func TestCoverRoutesOverwriteClientIPHeaders(t *testing.T) {
	for _, r := range []CoverRoute{{Path: "/ws", Dest: "127.0.0.1:1443"}, {Path: "/x", Dest: "127.0.0.1:1446", H2C: true}} {
		var b strings.Builder
		writeCoverReverseProxy(&b, r, "")
		out := b.String()
		for _, want := range []string{
			"header_up X-Forwarded-For {http.request.remote.host}\n",
			"header_up -X-Real-IP\n",
			"header_up -Forwarded\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("route %s lacks %q:\n%s", r.Path, want, out)
			}
		}
	}
}
