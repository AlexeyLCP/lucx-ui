// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
)

// TestAwgCPSBudget_MatchesSaveTimeGuard: the generator and ValidateIFields
// must never disagree, or "generate" hands back a set "save" then refuses.
func TestAwgCPSBudget_MatchesSaveTimeGuard(t *testing.T) {
	for _, withHPK := range []bool{false, true} {
		if got, want := awgCPSBudget(withHPK), awg.WorstCaseIBytesBudget(withHPK); got != want {
			t.Fatalf("awgCPSBudget(%v) = %d, want %d (ValidateIFields' own budget)", withHPK, got, want)
		}
	}
}

// The generator must not hand a 1.5 request I1-I5 — renderServerConf and
// renderClientConf drop them at 1.5, so offering them would show mimicry that never reaches the wire.
func TestAwgGenerateObfuscation_NoIFieldsOnV15(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInboundController(router.Group("/panel/api/inbounds"))

	for _, tc := range []struct {
		name       string
		awgVersion string
		wantI      bool
	}{
		{"1.5 omits I-fields", "1.5", false},
		{"2 keeps I-fields", "2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"awgVersion":"` + tc.awgVersion + `"}`
			req := httptest.NewRequest(http.MethodPost, "/panel/api/inbounds/awg/generateObfuscation", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
			}
			var m struct {
				Obj map[string]any `json:"obj"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &m); err != nil {
				t.Fatalf("unmarshal response: %v; body=%s", err, resp.Body.String())
			}
			_, got := m.Obj["i1"]
			if got != tc.wantI {
				t.Fatalf("awgVersion %q: i1 present = %v, want %v; obj=%v", tc.awgVersion, got, tc.wantI, m.Obj)
			}
		})
	}
}

// awg3ResponseKeys is the whole v3 block the generator owes a v3 inbound: the
// header-protection key plus the six AWG 3.0 device timer/padding ranges.
var awg3ResponseKeys = []string{
	"headerProtectionKey",
	"contentPaddingAddition",
	"rekeyAfterTime",
	"rekeyTimeout",
	"rejectAfterTime",
	"keepaliveTimeout",
	"maxHandshakeAttempts",
}

func generateObfuscation(t *testing.T, router *gin.Engine, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/panel/api/inbounds/awg/generateObfuscation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	var m struct {
		Obj map[string]any `json:"obj"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, resp.Body.String())
	}
	return m.Obj
}

// A node's AWG3 support is not stored on the master, so the master's own
// module probe must not answer for a node-hosted inbound.
func TestAwgGenerateObfuscation_NodeInboundKeepsAwg3Fields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewInboundController(router.Group("/panel/api/inbounds"))

	unsupported := false
	awg.SetModuleSupportsAwg3(&unsupported)
	t.Cleanup(func() { awg.SetModuleSupportsAwg3(nil) })

	for _, tc := range []struct {
		name        string
		body        string
		wantPresent bool
	}{
		{"node inbound keeps the v3 block the master cannot judge", `{"awgVersion":"3","nodeId":7}`, true},
		{"local inbound stays gated on the master's own probe", `{"awgVersion":"3","nodeId":null}`, false},
		{"nodeId omitted behaves exactly as before the field existed", `{"awgVersion":"3"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := generateObfuscation(t, router, tc.body)
			for _, key := range awg3ResponseKeys {
				if _, got := obj[key]; got != tc.wantPresent {
					t.Errorf("%s: %q present = %v, want %v; response keys = %v",
						tc.body, key, got, tc.wantPresent, slices.Sorted(maps.Keys(obj)))
				}
			}
		})
	}
}
