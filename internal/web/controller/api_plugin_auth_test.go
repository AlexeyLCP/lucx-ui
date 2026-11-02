package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/plugin"
)

/* The stage 3c panel API seam over the production auth path: a plugin bearer
hits enforceTokenScope with the manifest's resource matrix, the
never-reachable routes stay dark, and a bad plugin token is a 401. */

func writeAPITestPlugin(t *testing.T, id string, panelApi []string) string {
	t.Helper()
	err := os.MkdirAll(filepath.Join("bin", "plugins", id), 0o700)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join("bin", "plugins", id)) })
	perms := ""
	if len(panelApi) > 0 {
		perms = `"panelApi":[` + quotedJoin(panelApi) + `],`
	}
	manifest := `{"id":"` + id + `","name":"Api test","version":"1.0.0","apiVersion":1,"type":"sidecar","entrypoint":"run.sh","permissions":{` + perms + `"network":[],"bot":[]}}`
	if err := os.WriteFile(filepath.Join("bin", "plugins", id, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	tok, err := plugin.GrantPanelToken(id)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func quotedJoin(list []string) string {
	out := ""
	for i, v := range list {
		if i > 0 {
			out += ","
		}
		out += `"` + v + `"`
	}
	return out
}

func TestPluginBearerScopeEnforcement(t *testing.T) {
	oldWd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	engine, _ := newAPIAuthTestEngine(t)
	tok := writeAPITestPlugin(t, "apia-demo", []string{"server:read"})

	get := func(method, path, token string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w.Code
	}
	if code := get(http.MethodGet, "/panel/api/server/status", tok); code != http.StatusOK {
		t.Fatalf("granted read got %d", code)
	}
	if code := get(http.MethodPost, "/panel/api/server/updatePanel", tok); code != http.StatusForbidden {
		t.Fatalf("never-reachable POST got %d", code)
	}
	if code := get(http.MethodPost, "/panel/api/clients/x/detach", tok); code != http.StatusForbidden {
		t.Fatalf("ungranted client POST got %d", code)
	}
	if code := get(http.MethodGet, "/panel/api/server/status", tok+"x"); code != http.StatusUnauthorized {
		t.Fatalf("bad token got %d", code)
	}
}
