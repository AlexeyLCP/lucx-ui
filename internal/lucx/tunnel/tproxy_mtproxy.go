// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	telegramProxySecretURL = "https://core.telegram.org/getProxySecret"
	telegramProxyConfigURL = "https://core.telegram.org/getProxyConfig"
	mtproxyAssetMaxBytes   = 2 << 20
)

func mtproxyAssetsDir() string {
	return filepath.Join(workDir(), "mtproxy-assets")
}

func ensureTelegramMtproxyFiles() error {
	dir := mtproxyAssetsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := fetchIfMissing(filepath.Join(dir, "proxy-secret"), telegramProxySecretURL); err != nil {
		return err
	}
	return fetchIfMissing(filepath.Join(dir, "proxy-multi.conf"), telegramProxyConfigURL)
}

func fetchIfMissing(path, src string) error {
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return nil
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(src)
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
		"-p", strconv.Itoa(statsPort),
		"-H", strconv.Itoa(clientPort),
		"-S", secret,
		"--aes-pwd", absPath(secretFile),
		absPath(confFile),
		"-M", "1",
		"-C", "4096",
	}
}
