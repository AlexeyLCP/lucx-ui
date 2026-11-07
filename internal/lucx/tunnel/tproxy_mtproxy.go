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
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	telegramProxySecretURL = "https://core.telegram.org/getProxySecret"
	telegramProxyConfigURL = "https://core.telegram.org/getProxyConfig"
	mtproxyAssetMaxBytes   = 2 << 20
	mtproxyEngineUser      = "lucx-mtproxy"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "useradd", "-r", "-M", "-N", "-s", "/usr/sbin/nologin", mtproxyEngineUser).Run(); err != nil {
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

func mtproxyArgs(statsPort, clientPort int, secretFilePath string) []string {
	dir := mtproxyAssetsDir()
	secretFile := filepath.Join(dir, "proxy-secret")
	confFile := filepath.Join(dir, "proxy-multi.conf")
	if st, err := os.Stat(secretFile); err != nil || st.Size() == 0 {
		return nil
	}
	if st, err := os.Stat(confFile); err != nil || st.Size() == 0 {
		return nil
	}
	if strings.TrimSpace(secretFilePath) == "" {
		return nil
	}
	return []string{
		"-u", mtproxyRunUser(),
		"-p", strconv.Itoa(statsPort),
		"-H", strconv.Itoa(clientPort),
		"-S", "@" + absPath(secretFilePath),
		"--aes-pwd", absPath(secretFile),
		absPath(confFile),
		"-M", "1",
		"-C", "4096",
	}
}

// The official MTProxy has no outbound rate limit (`outbound-connections-ps`
// is commented out in mtproto-proxy.c): on a host that cannot reach Telegram
// it reconnects to every DC/middle-proxy address nonstop — XaksFlaX measured
// ~60 new TCP connections per second (~7000 NAT sessions) from one VM. The
// panel therefore verifies reachability before starting the engine and keeps
// it down when nothing answers (issue #170).

// tgProxyAddrRe matches the IPv4:port entries of proxy-multi.conf regardless
// of the key they ride on (proxy_for_medium, proxy_dc_*, statics...).
var tgProxyAddrRe = regexp.MustCompile(`(\d{1,3}(?:\.\d{1,3}){3}):(\d{1,5})`)

// dcProbeDial is a var so tests fake the network.
var dcProbeDial = func(hostPort string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", hostPort, timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

const (
	tgProbePerAttempt = 750 * time.Millisecond
	tgProbeLimit      = 6
	tgReachOkTTL      = 10 * time.Minute
	tgReachFailTTL    = 5 * time.Minute
)

type tgReachState struct {
	mu    sync.Mutex
	until time.Time
	ok    bool
}

// tgReach caches the probe result so a NAT-blocked host probes a handful of
// addresses a few times per TTL instead of on every 10s reconcile tick.
var tgReach tgReachState

// tgProxyAddresses extracts up to limit evenly spread distinct IPv4:port
// entries from the engine's own proxy-multi.conf.
func tgProxyAddresses(confPath string, limit int) []string {
	body, err := os.ReadFile(confPath)
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var all []string
	for _, m := range tgProxyAddrRe.FindAllSubmatch(body, -1) {
		addr := string(m[1]) + ":" + string(m[2])
		if _, dup := seen[addr]; dup {
			continue
		}
		seen[addr] = struct{}{}
		all = append(all, addr)
	}
	if len(all) <= limit {
		return all
	}
	step := len(all) / limit
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, all[i*step])
	}
	return out
}

// tgProxiesReachable reports whether at least one Telegram DC/middle-proxy
// address from the engine config accepts a TCP connection. A config without
// parseable addresses never gates the start (the engine owns that reality).
func tgProxiesReachable(confPath string) bool {
	tgReach.mu.Lock()
	defer tgReach.mu.Unlock()
	now := time.Now()
	if now.Before(tgReach.until) {
		return tgReach.ok
	}
	ok := true
	if addrs := tgProxyAddresses(confPath, tgProbeLimit); len(addrs) > 0 {
		ok = false
		for _, addr := range addrs {
			if dcProbeDial(addr, tgProbePerAttempt) == nil {
				ok = true
				break
			}
		}
	}
	tgReach.ok = ok
	if ok {
		tgReach.until = now.Add(tgReachOkTTL)
	} else {
		tgReach.until = now.Add(tgReachFailTTL)
	}
	return ok
}
