// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Install-by-link fetches a public GitHub repository and runs it through the
// same manifest validation and extraction contract as a ZIP upload. The link
// shape and flow (preview the manifest, then install on confirm) follow the
// community plugin system by AllIsA1 <https://github.com/AllIsA1/lucx-plugins>.

var (
	rawManifestURL = func(owner, repo, ref string) string {
		return "https://raw.githubusercontent.com/" + owner + "/" + repo + "/" + ref + "/manifest.json"
	}
	archiveByRefURL = func(owner, repo, ref string) string {
		return "https://github.com/" + owner + "/" + repo + "/archive/" + ref + ".tar.gz"
	}
)

var linkSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// parseRepoLink accepts https://github.com/<owner>/<repo>[/tree/<ref>].
// A bare link resolves to repository HEAD (the default branch); the ref may
// hold slashes (a feature branch name). Only github.com is accepted.
func parseRepoLink(link string) (string, string, string, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(link), ".git"))
	if trimmed == "" {
		return "", "", "", errors.New("plugin link is empty")
	}
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "www.")
	ref := "HEAD"
	if i := strings.Index(trimmed, "/tree/"); i > 0 {
		trimmed, ref = trimmed[:i], trimmed[i+len("/tree/"):]
	} else if i := strings.Index(trimmed, "/blob/"); i > 0 {
		trimmed, ref = trimmed[:i], trimmed[i+len("/blob/"):]
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	if !strings.HasPrefix(trimmed, "github.com/") {
		return "", "", "", errors.New("plugin link must point at a public GitHub repository")
	}
	parts := strings.Split(strings.TrimPrefix(trimmed, "github.com/"), "/")
	if len(parts) != 2 || !linkSegment.MatchString(parts[0]) || !linkSegment.MatchString(parts[1]) {
		return "", "", "", errors.New("plugin link owner/repo is malformed")
	}
	if ref != "HEAD" && (!linkSegment.MatchString(strings.SplitN(ref, "/", 2)[0]) || strings.Contains(ref, "..")) {
		return "", "", "", errors.New("plugin link ref is malformed")
	}
	return parts[0], parts[1], ref, nil
}

func fetchURL(limit int64, timeout time.Duration, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("not found at GitHub (wrong repository or ref)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit+1))
}

func manifestAt(owner, repo, ref string) (Manifest, error) {
	var m Manifest
	b, err := fetchURL(1<<20, 15*time.Second, rawManifestURL(owner, repo, ref))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, errors.New("manifest.json at the link is not valid JSON")
	}
	if m.ID == "" {
		return m, errors.New("manifest.json at the link has no id")
	}
	return m, validateManifest(m)
}

// PreviewFromLink validates the manifest of the linked repository without
// installing anything (a raw.githubusercontent.com fetch only).
func PreviewFromLink(link string) (Manifest, error) {
	owner, repo, ref, err := parseRepoLink(link)
	if err != nil {
		return Manifest{}, err
	}
	return manifestAt(owner, repo, ref)
}

// InstallFromLink downloads the repository tarball and installs it through
// the ZIP-install contract (dir overwrite, data/ preserved, no activation).
func InstallFromLink(link string) (Manifest, error) {
	owner, repo, ref, err := parseRepoLink(link)
	if err != nil {
		return Manifest{}, err
	}
	previewed, err := manifestAt(owner, repo, ref)
	if err != nil {
		return Manifest{}, err
	}
	b, err := fetchURL(maxPackageSize, 120*time.Second, archiveByRefURL(owner, repo, ref))
	if err != nil {
		return Manifest{}, err
	}
	if err := extractTar(b, previewed); err != nil {
		return Manifest{}, err
	}
	installed, err := Load(previewed.ID)
	if err != nil {
		_ = os.RemoveAll(dir(previewed.ID))
		return Manifest{}, err
	}
	if !sameManifest(installed, previewed) {
		_ = os.RemoveAll(dir(previewed.ID))
		return Manifest{}, errors.New("repository manifest changed between preview and install; retry from a pinned ref")
	}
	return installed, nil
}

// sameManifest guards the preview→install window: the repository content must
// still be what the operator reviewed.
func sameManifest(a, b Manifest) bool {
	return a.ID == b.ID && a.Version == b.Version && a.APIVersion == b.APIVersion &&
		a.Type == b.Type && a.Entrypoint == b.Entrypoint && a.Resources.Equal(b.Resources) &&
		a.Health.Equal(b.Health) && equalPermissions(a, b)
}

func equalManifestStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalPermissions(a, b Manifest) bool {
	if len(a.Permissions) != len(b.Permissions) {
		return false
	}
	for k, va := range a.Permissions {
		vb, ok := b.Permissions[k]
		if !ok || !equalManifestStrings(va, vb) {
			return false
		}
	}
	return true
}

// extractTar lays a .tar.gz repository over the plugin directory with the
// InstallPackage guarantees: no unsafe paths, 0700 everywhere, directories and
// regular files only, data/ never touched, and the executable entrypoint
// present afterwards.
func extractTar(body []byte, m Manifest) error {
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return errors.New("repository archive is not tar.gz")
	}
	tr := tar.NewReader(gz)
	var top string
	wroteManifest := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name, rest, cut := strings.Cut(h.Name, "/")
		if !cut && h.Typeflag != tar.TypeDir {
			return errors.New("unexpected repository archive layout")
		}
		if top == "" {
			top = name
		}
		if name != top {
			return errors.New("unexpected repository archive layout")
		}
		if !cut || rest == "" {
			continue
		}
		if rest == "data" || strings.HasPrefix(rest, "data/") {
			continue
		}
		target := filepath.Join(dir(m.ID), filepath.FromSlash(rest))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
			if err == nil {
				_, err = io.Copy(f, tr)
				f.Close()
			}
			if err != nil {
				return err
			}
			if rest == "manifest.json" {
				wroteManifest = true
			}
		default:
			return errors.New("repository archive holds a non-file entry")
		}
	}
	if !wroteManifest {
		return errors.New("repository archive misses manifest.json")
	}
	info, err := os.Stat(filepath.Join(dir(m.ID), m.Entrypoint))
	if err != nil {
		return errors.New("plugin entrypoint is missing after install")
	}
	if info.IsDir() {
		return errors.New("plugin entrypoint is a directory")
	}
	// NTFS carries no exec bit; the 0700 guarantee is Unix-only.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return errors.New("plugin entrypoint is not executable")
	}
	return nil
}
