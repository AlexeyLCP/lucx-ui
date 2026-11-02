// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidatePanelPermissions(t *testing.T) {
	good := []string{"inbounds:read", "clients:write", "tunnel:*"}
	if err := validatePanelPermissions(good); err != nil {
		t.Fatalf("valid panelApi rejected: %v", err)
	}
	if err := validatePanelPermissions([]string{"plugins:read"}); err == nil {
		t.Fatal("the plugin-system resource must not be grantable")
	}
	if err := validatePanelPermissions([]string{"inbounds"}); err == nil {
		t.Fatal("a permission without a kind was accepted")
	}
	if err := validatePanelPermissions([]string{"inbounds:admin"}); err == nil {
		t.Fatal("an unknown kind was accepted")
	}
	if err := validatePanelPermissions([]string{"clients:read", "clients:read"}); err == nil {
		t.Fatal("a duplicate permission was accepted")
	}
	many := make([]string, 17)
	for i := range many {
		many[i] = "server:read"
	}
	if err := validatePanelPermissions(many); err == nil {
		t.Fatal("an oversized panelApi list was accepted")
	}
}

func TestValidateBotPermissions(t *testing.T) {
	if err := validateBotPermissions([]string{"telegram", "discord"}); err != nil {
		t.Fatalf("valid bot list rejected: %v", err)
	}
	if err := validateBotPermissions([]string{"matrix"}); err == nil {
		t.Fatal("an unknown bot was accepted")
	}
	if err := validateBotPermissions([]string{"telegram", "telegram"}); err == nil {
		t.Fatal("a duplicate bot was accepted")
	}
}

func TestPanelDeniedByNetwork(t *testing.T) {
	bad := Manifest{Permissions: map[string][]string{
		"network":  {networkNone},
		"panelApi": {"inbounds:read"},
	}}
	if !panelDeniedByNetwork(bad) {
		t.Fatal("netns + panelApi must be refused")
	}
	if err := validateManifest(bad); err == nil {
		t.Fatal("validateManifest must refuse the netns + panelApi combo")
	}
	badBot := Manifest{Permissions: map[string][]string{
		"network": {networkNone},
		"bot":     {"telegram"},
	}}
	if err := validateManifest(badBot); err == nil {
		t.Fatal("validateManifest must refuse the netns + bot combo")
	}
	ok := Manifest{Permissions: map[string][]string{"panelApi": {"inbounds:read"}}}
	if panelDeniedByNetwork(ok) {
		t.Fatal("plain networking with panelApi is fine")
	}
}

func TestGrantAndMatchPanelToken(t *testing.T) {
	td := t.TempDir()
	old := binFolder
	binFolder = func() string { return td }
	t.Cleanup(func() { binFolder = old })
	forgetPanelToken("tok-demo")

	tok, err := GrantPanelToken("tok-demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Fatalf("token length wrong: %d", len(tok))
	}
	info, err := os.Stat(filepath.Join(dir("tok-demo"), panelTokenFile))
	if err == nil && runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("token file mode: %v", info.Mode().Perm())
	}
	tok2, err := GrantPanelToken("tok-demo")
	if err != nil || tok2 != tok {
		t.Fatalf("grant is not stable: %s vs %s", tok, tok2)
	}
	if id, ok := MatchPanelToken(tok); !ok || id != "tok-demo" {
		t.Fatalf("match failed: %q %v", id, ok)
	}
	if _, ok := MatchPanelToken(tok + "x"); ok {
		t.Fatal("a mutated token matched")
	}
	forgetPanelToken("tok-demo")
	if _, ok := MatchPanelToken(tok); ok {
		t.Fatal("a forgotten token matched")
	}
	LoadPanelTokens()
	if id, ok := MatchPanelToken(tok); !ok || id != "tok-demo" {
		t.Fatal("reloaded tokens do not match")
	}
	forgetPanelToken("tok-demo")
}

func TestPluginPanelAllowedMatrix(t *testing.T) {
	td := t.TempDir()
	old := binFolder
	binFolder = func() string { return td }
	t.Cleanup(func() { binFolder = old })
	if err := os.MkdirAll(dir("px-demo"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"px-demo","name":"Px","version":"1.0.0","apiVersion":1,"type":"sidecar","entrypoint":"run.sh","permissions":{"panelApi":["inbounds:read","clients:write","server:*"],"panelApi_note":""} }`
	manifest = strings.Replace(manifest, `,"panelApi_note":""`, "", 1)
	if err := os.WriteFile(manifestPath("px-demo"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		rel, method string
		want        bool
	}{
		{"/inbounds/list", http.MethodGet, true},
		{"/inbounds/add", http.MethodPost, false},
		{"/inbounds/del/:id", http.MethodPost, false},
		{"/clients/add", http.MethodPost, true},
		{"/clients/list", http.MethodGet, false},
		{"/server/status", http.MethodGet, true},
		{"/server/importDB", http.MethodPost, false},
		{"/server/getDb", http.MethodPost, false},
		{"/server/updatePanel", http.MethodPost, false},
		{"/plugins/status", http.MethodGet, false},
		{"/setting/allSetting/getAllSetting", http.MethodGet, false},
		{"/xray/getXrayConfig", http.MethodGet, false},
		{"/nodes/list", http.MethodGet, false},
		{"/hosts/list", http.MethodGet, false},
		{"/backuptotgbot", http.MethodPost, false},
		{"/unknown", http.MethodGet, false},
	}
	for _, cc := range cases {
		if got := PluginPanelAllowed("px-demo", cc.rel, cc.method); got != cc.want {
			t.Errorf("%s %s: allowed=%v want %v", cc.method, cc.rel, got, cc.want)
		}
	}
	if PluginPanelAllowed("missing-plugin", "/inbounds/list", http.MethodGet) {
		t.Error("an unknown plugin must never pass")
	}
	if PluginPanelAllowed("../evil", "/inbounds/list", http.MethodGet) {
		t.Error("an unsafe plugin id must never pass")
	}
}
