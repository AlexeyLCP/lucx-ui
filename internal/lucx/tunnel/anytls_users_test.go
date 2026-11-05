// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAnytlsUsersDerivedPerClient(t *testing.T) {
	ib := &model.Inbound{Id: 7, Protocol: model.Anytls, Settings: `{"password":"shared","clients":[
		{"email":"a@x","enable":true},{"email":"b@x","enable":true},
		{"email":"off@x","enable":false},{"email":"bad:name","enable":true},{"email":"a@x","enable":true}]}`}
	secret := []byte("panel-secret")
	users := AnytlsUsers(secret, ib)
	if len(users) != 2 || users[0].Email != "a@x" || users[1].Email != "b@x" {
		t.Fatalf("users = %+v", users)
	}
	if users[0].Password == "" || users[0].Password == users[1].Password || users[0].Password == "shared" {
		t.Fatalf("passwords not distinct per client: %+v", users)
	}
	if got := InboundAuthPair(secret, ib, "a@x").Pass; got != users[0].Password {
		t.Fatalf("link password %q != server password %q", got, users[0].Password)
	}
	other := *ib
	other.Id = 8
	if AnytlsUsers(secret, &other)[0].Password == users[0].Password {
		t.Fatal("same password on another inbound")
	}
	if TrustTunnelClientAuth(secret, 7, "a@x").Pass == users[0].Password {
		t.Fatal("AnyTLS shares the TrustTunnel credential namespace")
	}
	if AnytlsUsers(nil, ib) != nil {
		t.Fatal("no secret must mean shared password only")
	}
	want := "a@x:" + users[0].Password + "\nb@x:" + users[1].Password + "\n"
	if got := RenderAnytlsUsers(users); got != want {
		t.Fatalf("users file = %q", got)
	}
}

func TestAnytlsBuildArgsUsersAndStats(t *testing.T) {
	cfg := AnytlsConfig{Port: 443, Password: "p", usersFile: "/d/users", statsFile: "/d/stats.json"}
	args := strings.Join(cfg.BuildArgs("", "", "/d/password"), " ")
	if !strings.Contains(args, "-users-file /d/users -stats /d/stats.json") {
		t.Fatalf("args = %s", args)
	}
	cfg.usersFile = ""
	if args := strings.Join(cfg.BuildArgs("", "", "/d/password"), " "); strings.Contains(args, "-users-file") || strings.Contains(args, "-stats") {
		t.Fatalf("args without users = %s", args)
	}
}

func TestReadAnytlsStats(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	dir := dataDirFor(AnytlsKey(5), Anytls)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadAnytlsStats(5); ok {
		t.Fatal("stats without a file")
	}
	write := func(updated int64) {
		b, _ := json.Marshal(map[string]any{
			"started": 1, "updated": updated,
			"users": map[string]any{"a@x": map[string]any{
				"up": 10, "down": 20,
				"ips": map[string]any{"203.0.113.7": map[string]any{"active": 1, "last": updated}},
			}},
		})
		if err := os.WriteFile(filepath.Join(dir, "stats.json"), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(time.Now().Unix())
	st, ok := ReadAnytlsStats(5)
	if !ok || st.Users["a@x"].Down != 20 || st.Users["a@x"].IPs["203.0.113.7"].Active != 1 {
		t.Fatalf("stats = %+v ok=%v", st, ok)
	}
	write(time.Now().Add(-time.Hour).Unix())
	if _, ok := ReadAnytlsStats(5); ok {
		t.Fatal("a stale stats file must be ignored")
	}
}

func TestAnytlsStoredRouteGetsProxyOnceSupported(t *testing.T) {
	anytlsProbeHelp()
	saved := anytlsProxyOK
	t.Cleanup(func() { anytlsProxyOK = saved })
	t.Cleanup(SetTrustTunnelProxyForTest(false))

	others := []*model.Inbound{
		{Id: 19, Protocol: model.Anytls, Listen: "127.0.0.1", Port: 1445},
		{Id: 41, Protocol: model.TrustTunnel, Listen: "127.0.0.1", Port: 1447},
	}
	stored := []GatewayRoute{
		{SNI: "a.example.com", Dest: "127.0.0.1:1445", NoProxy: true},
		{SNI: "t.example.com", Dest: "127.0.0.1:1447", NoProxy: true},
	}
	selected := map[int]bool{19: true, 41: true}

	anytlsProxyOK = false
	if got := sidecarRoutesWithProxy(stored, others, selected); !got[0].NoProxy {
		t.Fatal("old AnyTLS binary must keep NoProxy")
	}
	anytlsProxyOK = true
	got := sidecarRoutesWithProxy(stored, others, selected)
	if got[0].NoProxy || !got[1].NoProxy {
		t.Fatalf("routes = %+v (only AnyTLS may switch to PROXY)", got)
	}
	if !stored[0].NoProxy {
		t.Fatal("stored routes were modified in place")
	}
	if got := sidecarRoutesWithProxy(stored, others, map[int]bool{41: true}); !got[0].NoProxy {
		t.Fatal("an AnyTLS inbound not behind the gateway must be left alone")
	}
	restoreTT := SetTrustTunnelProxyForTest(true)
	both := sidecarRoutesWithProxy(stored, others, selected)
	restoreTT()
	if both[0].NoProxy || both[1].NoProxy {
		t.Fatalf("routes = %+v (TrustTunnel with the LucX build takes PROXY too)", both)
	}
	var b strings.Builder
	writeL4Handler(&b, got[0])
	if !strings.Contains(b.String(), "proxy_protocol v1") {
		t.Fatalf("handler = %q", b.String())
	}
}
