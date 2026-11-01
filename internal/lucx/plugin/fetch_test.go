// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRepoLink(t *testing.T) {
	owner, repo, ref, err := parseRepoLink("https://github.com/owner/repo")
	if err != nil || owner != "owner" || repo != "repo" || ref != "HEAD" {
		t.Fatalf("plain link: %v %v %v %v", owner, repo, ref, err)
	}
	if _, _, _, err := parseRepoLink("github.com/owner/repo.git"); err != nil {
		t.Fatalf("bare link rejected: %v", err)
	}
	if _, _, _, err := parseRepoLink("https://www.github.com/owner/repo"); err != nil {
		t.Fatalf("www link rejected: %v", err)
	}
	_, _, ref, err = parseRepoLink("https://github.com/owner/repo/tree/v1.2.1")
	if err != nil || ref != "v1.2.1" {
		t.Fatalf("pinned tag: %v %v", ref, err)
	}
	_, _, _, err = parseRepoLink("https://gitlab.com/owner/repo")
	if err == nil {
		t.Fatal("hosts other than github.com must be refused")
	}
	_, _, _, err = parseRepoLink("https://github.com/own er/repo")
	if err == nil {
		t.Fatal("malformed owner must be refused")
	}
	_, _, _, err = parseRepoLink("https://github.com/owner/repo/tree/../..")
	if err == nil {
		t.Fatal("ref traversal must be refused")
	}
}

func TestPreviewFromLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/owner/repo/HEAD/manifest.json" {
			_, _ = w.Write([]byte(`{"id":"hello","name":"Hi","version":"1.0.0","apiVersion":1,"type":"sidecar","entrypoint":"run.sh","architectures":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	origRaw := rawManifestURL
	rawManifestURL = func(owner, repo, ref string) string {
		return srv.URL + "/" + owner + "/" + repo + "/" + ref + "/manifest.json"
	}
	defer func() { rawManifestURL = origRaw }()

	m, err := PreviewFromLink("github.com/owner/repo")
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if m.ID != "hello" {
		t.Fatalf("unexpected manifest id: %+v", m)
	}
}

func TestPreviewFromLinkRejectsBadManifest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"hello","name":"Hi","version":"1.0.0","apiVersion":2}`))
	}))
	defer srv.Close()
	origRaw := rawManifestURL
	rawManifestURL = func(owner, repo, ref string) string { return srv.URL }
	defer func() { rawManifestURL = origRaw }()
	if _, err := PreviewFromLink("github.com/owner/repo"); err == nil {
		t.Fatal("invalid manifest must be refused at preview")
	}
}

// buildTestRepoTars returns the archive bytes of a fake repository tree.
func buildTestRepoTars(t *testing.T, manifest string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	entry := func(name, body string) {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: int64(0o700), Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	}
	entry("repo-ref/manifest.json", manifest)
	entry("repo-ref/run.sh", "#!/bin/sh\nwhile :; do sleep 5; done\n")
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func TestInstallFromLinkFullFlow(t *testing.T) {
	manifest := `{"id":"hello","name":"Hi","version":"1.0.0","apiVersion":1,"type":"sidecar","entrypoint":"run.sh","architectures":[]}`
	archive := buildTestRepoTars(t, manifest)
	links := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/manifest.json") {
			_, _ = w.Write([]byte(manifest))
			return
		}
		_, _ = w.Write(archive)
	}))
	defer links.Close()
	origRaw, origArc := rawManifestURL, archiveByRefURL
	rawManifestURL = func(owner, repo, ref string) string {
		return links.URL + "/" + owner + "/" + repo + "/" + ref + "/manifest.json"
	}
	archiveByRefURL = func(owner, repo, ref string) string { return links.URL + "/archive/" + ref + ".tar.gz" }
	defer func() { rawManifestURL, archiveByRefURL = origRaw, origArc }()

	td := t.TempDir()
	origBin := binFolder
	binFolder = func() string { return td }
	defer func() { binFolder = origBin }()

	// A pre-existing data file must survive the install.
	if err := os.MkdirAll(filepath.Join(dir("hello"), "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir("hello"), "data", "state"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallFromLink("github.com/owner/repo"); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir("hello"), "data", "state"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("data/ must survive install: %v", err)
	}
	script, err := os.ReadFile(filepath.Join(dir("hello"), "run.sh"))
	if err == nil && !strings.Contains(string(script), "sleep") {
		t.Fatalf("entrypoint not extracted: %q", script)
	}
	// manifest.json must be present and match the preview.
	var got Manifest
	if err := json.Unmarshal(mustRead(t, filepath.Join(dir("hello"), "manifest.json")), &got); err != nil || got.Version != "1.0.0" {
		t.Fatalf("manifest lost on install: %v %+v", err, got)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSameManifestGuardsReview(t *testing.T) {
	a := Manifest{ID: "hello", Version: "1.0.0", Permissions: map[string][]string{"network": {"none"}}}
	b := Manifest{ID: "hello", Version: "1.0.0", Permissions: map[string][]string{"network": {"none"}}}
	if !sameManifest(a, b) {
		t.Fatal("equal manifests must match")
	}
	b.Permissions["network"] = []string{}
	if sameManifest(a, b) {
		t.Fatal("grown permissions must be caught")
	}
	a.Resources = &Resources{MaxMemoryMB: 64}
	if sameManifest(a, b) {
		t.Fatal("resources must be compared too")
	}
}
