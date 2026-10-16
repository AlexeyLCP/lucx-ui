// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build !linux

package tunnel

import "errors"

// No bridge on this platform: the routed anytls keeps default credentials and
// direct egress (the builder sees redirectOwnerUID() == 0 and never sets a
// RunUid, so nothing should call prepareRunAssets here).
func prepareRunAssets(paths []string, uid int) (int, error) {
	return 0, errors.New("tunnel: run-as-uid assets are linux-only")
}

func EnsureAnytlsXraySocks(socksPort int) {}

func ClearAnytlsXraySocks() {}
