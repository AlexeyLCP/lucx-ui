// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func subAfterFixture(t *testing.T, id string, grants string, handler http.HandlerFunc) Manifest {
	td := t.TempDir()
	old := binFolder
	binFolder = func() string { return td }
	t.Cleanup(func() { binFolder = old })
	plugDir := dir(id)
	if err := os.MkdirAll(plugDir, 0o700); err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		ID: id, Name: id, Version: "0.1.0", APIVersion: 1, Type: "sidecar",
		Entrypoint: "main", Permissions: map[string][]string{"sub": {grants}},
	}
	if grants == "" {
		m.Permissions = map[string][]string{}
	}
	b, _ := json.Marshal(m)
	if err := os.WriteFile(manifestPath(id), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if handler != nil {
		sock := filepath.Join(td, id+".sock")
		ln, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		srv := &http.Server{Handler: handler}
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(func() {
			_ = srv.Close()
			_ = ln.Close()
			forgetSocket(id)
		})
		registerSocket(id, sock)
	}
	return m
}

func TestValidateSubPermissions(t *testing.T) {
	if err := validateSubPermissions([]string{"after"}); err != nil {
		t.Fatalf("after rejected: %v", err)
	}
	if err := validateSubPermissions([]string{"import"}); err == nil {
		t.Fatal("an unimplemented sub rule was accepted")
	}
	if err := validateSubPermissions([]string{"after", "after"}); err == nil {
		t.Fatal("a duplicate sub rule was accepted")
	}
	p := map[string][]string{"sub": {"after"}}
	if err := validatePermissions(p); err != nil {
		t.Fatalf("sub permission rejected by the grammar: %v", err)
	}
	p = map[string][]string{"sub": {"bogus"}}
	if err := validatePermissions(p); err == nil {
		t.Fatal("an unknown sub rule passed the grammar")
	}
}

func TestValidateManifestRefusesSubWithNetns(t *testing.T) {
	m := Manifest{
		ID: "sub-netns", Name: "n", Version: "0.1", APIVersion: 1, Type: "sidecar",
		Entrypoint: "main", Permissions: map[string][]string{"sub": {"after"}, "network": {"none"}},
	}
	if err := validateManifest(m); err == nil {
		t.Fatal("sub/after combined with network isolation must be refused")
	}
}

func TestSubAfterInjectCallsGrantedPlugin(t *testing.T) {
	var got subAfterRequest
	m := subAfterFixture(t, "sub-ok", "after", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		// \n arrives as a JSON escape; the trailing link must be trimmed out.
		_, _ = w.Write([]byte(`{"links":["  vless://x#extra  ",""]}`))
	})
	links := SubAfterInject(m.ID, "host.example", 3, []string{"a@x"})
	if len(links) != 1 || links[0] != "vless://x#extra" {
		t.Fatalf("unexpected links: %q", links)
	}
	if got.SubID != m.ID || got.Host != "host.example" || got.Count != 3 || got.Emails[0] != "a@x" {
		t.Fatalf("unexpected payload: %+v", got)
	}
	if rs := runStateOf(m.ID); rs == nil || rs.delivered.Load() != 1 || rs.failed.Load() != 0 {
		t.Fatal("delivery counters not updated")
	}
}

func TestSubAfterInjectSkipsUngrantedAndFailures(t *testing.T) {
	subAfterFixture(t, "sub-nogrant", "", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"links":["vless://x"]}`))
	})
	if links := SubAfterInject("sub-nogrant", "h", 0, nil); links != nil {
		t.Fatalf("a plugin without the grant was consulted: %q", links)
	}
	subAfterFixture(t, "sub-fail", "after", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if links := SubAfterInject("sub-fail", "h", 0, nil); links != nil {
		t.Fatalf("a failing plugin leaked links: %q", links)
	}
}

func TestSubAfterInjectCapsLinks(t *testing.T) {
	var lots []string
	for i := 0; i < 40; i++ {
		lots = append(lots, "vless://"+strconv.Itoa(i))
	}
	big := "vless://" + string(make([]byte, subAfterMaxLink))
	reply, _ := json.Marshal(map[string]any{"links": append(lots[:2:2], append([]string{"", big}, lots[2:]...)...)})
	subAfterFixture(t, "sub-cap", "after", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(reply)
	})
	links := SubAfterInject("sub-cap", "h", 0, nil)
	if len(links) != subAfterMaxPerCall || links[0] != "vless://0" || links[1] != "vless://1" {
		t.Fatalf("cap/trim wrong: %d links", len(links))
	}
}

func TestSubAfterSocketFallsBackToChildPath(t *testing.T) {
	subAfterFixture(t, "sub-sock", "after", nil)
	m := Manifest{ID: "sub-sock"}
	got := subAfterSocket(m)
	if filepath.Base(got) != "plugin.sock" {
		t.Fatalf("expected the child-runner shape, got %q", got)
	}
	registerSocket(m.ID, "/recorded/path.sock")
	if got := subAfterSocket(m); got != "/recorded/path.sock" {
		t.Fatalf("the recorded path must win, got %q", got)
	}
}
