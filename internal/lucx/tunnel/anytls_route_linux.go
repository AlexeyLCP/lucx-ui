// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

import (
	"errors"
	"os/user"
	"strconv"
	"syscall"
)

// AnyTLS routeThroughXray bridge (lucx.273): anytls-go has no SOCKS dialer
// option, so the sidecar's own outbound TCP must be redirected by uid (shared
// lucx-mtproxy user) into the tproxy-style listener (23990) which relays into
// the hidden Xray SOCKS inbound. The rule/listener are shared with a routed
// tproxy inbound; redirectOwners in tproxy_firewall_linux.go decides when the
// physical rule may actually be removed.
//
// The REDIRECT matches --uid-owner of the bridge user, so the anytls child
// must run UNDER that uid (lucx.301): redirectOwnerUID() hands it to the
// instance, prepareRunAssets chowns the files the uid must read (the cert/key
// ExtraFiles copies and the data dir).

func init() {
	redirectOwnerUID = func() int {
		if u, err := user.Lookup(mtproxyEngineUser); err == nil {
			if n, convErr := strconv.Atoi(u.Uid); convErr == nil && n > 0 {
				return n
			}
		}
		return 0
	}
}

// prepareRunAssets chowns the files a uid-scoped sidecar must read (extra
// files like the cert copies, plus the data dir) to the bridge uid and
// returns the user's primary gid for the process credential. The first
// failure aborts; the caller falls back to running as root (direct egress).
func prepareRunAssets(paths []string, uid int) (int, error) {
	if uid <= 0 {
		return 0, errors.New("tunnel: run-as uid not set")
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return 0, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, err
	}
	for _, p := range paths {
		if err := syscall.Chown(p, uid, gid); err != nil {
			return 0, err
		}
	}
	return gid, nil
}

func EnsureAnytlsXraySocks(socksPort int) {
	redirectOwnerEnsure("anytls", socksPort)
}

func ClearAnytlsXraySocks() {
	redirectOwnerClear("anytls")
}
