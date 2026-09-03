// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	telegramProxySecretURL = "https://core.telegram.org/getProxySecret"
	telegramProxyConfigURL = "https://core.telegram.org/getProxyConfig"
	mtproxyAssetMaxBytes   = 2 << 20
	mtproxyEngineUser      = "mtproxy"
)

func mtproxyAssetsDir() string {
	return filepath.Join(workDir(), "mtproxy-assets")
}

func ensureTelegramMtproxyFiles() error {
	dir := mtproxyAssetsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0o755)
	ensureMtproxyUser()
	secret := filepath.Join(dir, "proxy-secret")
	conf := filepath.Join(dir, "proxy-multi.conf")
	if err := fetchIfMissing(secret, telegramProxySecretURL); err != nil {
		return err
	}
	if err := fetchIfMissing(conf, telegramProxyConfigURL); err != nil {
		return err
	}
	// The engine drops root before its periodic proxy-multi.conf reload, and
	// both assets are public values from core.telegram.org — keep them
	// readable for the unprivileged user.
	_ = os.Chmod(secret, 0o644)
	_ = os.Chmod(conf, 0o644)
	return nil
}

// ensureMtproxyUser creates the system user the MTProxy engine drops root to
// (upstream DEFAULT_ENGINE_USER "mtproxy"). Without it the binary fatal-exits
// "can't find the user mtproxy to switch to" on every root-run host
// (VladufQa, 03.09.2026).
func ensureMtproxyUser() {
	if _, err := user.Lookup(mtproxyEngineUser); err == nil {
		return
	}
	if os.Geteuid() != 0 {
		return
	}
	if err := exec.Command("useradd", "-r", "-M", "-N", "-s", "/usr/sbin/nologin", mtproxyEngineUser).Run(); err != nil {
		logger.Warningf("tunnel: mtproxy: create %s user failed: %v", mtproxyEngineUser, err)
	}
}

// mtproxyRunUser picks the privilege-drop user for -u: the engine's own
// default, else nobody, else the invoking user (a non-root run skips the
// drop inside the engine anyway).
func mtproxyRunUser() string {
	for _, name := range []string{mtproxyEngineUser, "nobody"} {
		if _, err := user.Lookup(name); err == nil {
			return name
		}
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "root"
}

func fetchIfMissing(path, src string) error {
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return fmt.Errorf("tproxy: fetch %s: %w", src, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("tproxy: fetch %s: %w", src, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tproxy: fetch %s: HTTP %d", src, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, mtproxyAssetMaxBytes+1))
	if err != nil {
		return err
	}
	if len(body) == 0 || int64(len(body)) > mtproxyAssetMaxBytes {
		return fmt.Errorf("tproxy: fetch %s: empty or too large", src)
	}
	return os.WriteFile(path, body, 0o600)
}

func mtproxyArgs(statsPort, clientPort int, secret string) []string {
	dir := mtproxyAssetsDir()
	secretFile := filepath.Join(dir, "proxy-secret")
	confFile := filepath.Join(dir, "proxy-multi.conf")
	if st, err := os.Stat(secretFile); err != nil || st.Size() == 0 {
		return nil
	}
	if st, err := os.Stat(confFile); err != nil || st.Size() == 0 {
		return nil
	}
	secret = strings.ToLower(strings.TrimSpace(secret))
	if secret == "" {
		return nil
	}
	return []string{
		"-u", mtproxyRunUser(),
		"-p", strconv.Itoa(statsPort),
		"-H", strconv.Itoa(clientPort),
		"-S", secret,
		"--aes-pwd", absPath(secretFile),
		absPath(confFile),
		"-M", "1",
		"-C", "4096",
	}
}
