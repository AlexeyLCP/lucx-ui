// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type tproxyRemoveOpts struct {
	run        func(name string, args ...string) error
	nginxRoots []string
}

// RemoveTproxyInstall stops the foreign WEB-proxy units and deletes their
// config, site dir, and an nginx vhost that only reverse-proxies that listen
// address. The panel does not take over :443.
func RemoveTproxyInstall(confPath string) error {
	return removeTproxyInstall(confPath, tproxyRemoveOpts{
		nginxRoots: []string{
			"/etc/nginx/sites-enabled",
			"/etc/nginx/sites-available",
			"/etc/nginx/conf.d",
		},
	})
}

func removeTproxyInstall(confPath string, opts tproxyRemoveOpts) error {
	dir := filepath.Dir(confPath)
	if filepath.Base(dir) != "tproxy-server" {
		return fmt.Errorf("tproxy: refuse to remove %s", dir)
	}
	listen, publicDir := tproxyInstallHints(confPath)
	run := opts.run
	if run == nil {
		run = runCmd
	}
	var stopErr error
	for _, unit := range []string{"tproxy-server.service", "mtprotoproxy.service"} {
		if err := run("systemctl", "disable", "--now", unit); err != nil && stopErr == nil && unit == "tproxy-server.service" {
			stopErr = err
		}
	}
	for _, unit := range []string{"tproxy-server.service", "mtprotoproxy.service"} {
		_ = os.Remove(filepath.Join("/etc/systemd/system", unit))
		_ = os.Remove(filepath.Join("/etc/systemd/system/multi-user.target.wants", unit))
	}
	_ = run("systemctl", "daemon-reload")
	if listen != "" {
		if n, err := removeNginxProxySites(opts.nginxRoots, listen); err != nil {
			return err
		} else if n > 0 {
			if err := run("nginx", "-t"); err == nil {
				_ = run("systemctl", "reload", "nginx")
			}
		}
	}
	if removableDir(publicDir) {
		_ = os.RemoveAll(publicDir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return stopErr
}

func tproxyInstallHints(confPath string) (listen, publicDir string) {
	raw, err := os.ReadFile(confPath)
	if err != nil {
		return "", ""
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", ""
	}
	return jsonString(cfg, "listen", "listen_addr"), jsonString(cfg, "public_dir", "publicDir")
}

func removeNginxProxySites(roots []string, listen string) (int, error) {
	needles := proxyPassNeedles(listen)
	if len(needles) == 0 {
		return 0, nil
	}
	seen := map[string]struct{}{}
	n := 0
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(root, e.Name())
			if _, ok := seen[path]; ok {
				continue
			}
			body, err := os.ReadFile(path)
			if err != nil || !siteOnlyProxies(string(body), needles) {
				continue
			}
			target := path
			if dest, err := filepath.EvalSymlinks(path); err == nil {
				target = dest
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return n, err
			}
			seen[path] = struct{}{}
			n++
			if target != path {
				if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
					return n, err
				}
				seen[target] = struct{}{}
			}
		}
	}
	return n, nil
}

func proxyPassNeedles(listen string) []string {
	host, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil || port == "" {
		return nil
	}
	needles := []string{
		"proxy_pass http://127.0.0.1:" + port,
		"proxy_pass http://[::1]:" + port,
	}
	if host != "" && host != "0.0.0.0" && host != "::" && host != "127.0.0.1" && host != "::1" {
		needles = append(needles, "proxy_pass http://"+net.JoinHostPort(host, port))
	}
	return needles
}

func siteOnlyProxies(body string, needles []string) bool {
	hit := false
	rest := body
	for _, n := range needles {
		if hasProxyPass(rest, n) {
			hit = true
		}
		rest = strings.ReplaceAll(rest, n, "")
	}
	return hit && !strings.Contains(rest, "proxy_pass")
}

func hasProxyPass(body, needle string) bool {
	for i := 0; i < len(body); {
		j := strings.Index(body[i:], needle)
		if j < 0 {
			return false
		}
		j += i
		end := j + len(needle)
		if end >= len(body) || strings.ContainsRune(";/ \t\r\n", rune(body[end])) {
			return true
		}
		i = end
	}
	return false
}

func removableDir(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || !filepath.IsAbs(path) || path == filepath.Dir(path) {
		return false
	}
	switch filepath.Base(path) {
	case ".", "..", "etc", "usr", "var", "root", "home", "opt", "srv", "bin":
		return false
	}
	return true
}

func runCmd(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", name, err, bytes.TrimSpace(out))
	}
	return nil
}
