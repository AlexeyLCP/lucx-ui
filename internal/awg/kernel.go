// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"sync"
)

var (
	kernelAvailMu   sync.Mutex
	kernelAvailTrue bool
	probeKernel     = kernelAvailable
)

// KernelAvailable reports whether the host can run AWG as a kernel interface
// (amneziawg module present and awg-quick found). When false, callers should
// use the embedded amneziawg-go path instead of awg-quick.
//
// Only a positive result is cached. A negative one is transient (module not
// loaded yet at first Xray start, thin systemd PATH, mid-rebuild). Caching
// false via sync.Once (lucx.169) permanently sent every LucX awg inbound to
// userspace and made AwgJob Reconcile(nil) — UI showed module loaded, 0
// interfaces, kernel Stopped (testers on lucx.169–172).
func KernelAvailable() bool {
	kernelAvailMu.Lock()
	defer kernelAvailMu.Unlock()
	if kernelAvailTrue {
		return true
	}
	if probeKernel() {
		kernelAvailTrue = true
		return true
	}
	return false
}

// ResetKernelAvailableForTest clears the cached probe. Tests only.
func ResetKernelAvailableForTest() {
	kernelAvailMu.Lock()
	defer kernelAvailMu.Unlock()
	kernelAvailTrue = false
	probeKernel = kernelAvailable
}
