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
	kernelAvailOnce sync.Once
	kernelAvail     bool
)

// KernelAvailable reports whether the host can run AWG as a kernel interface
// (amneziawg module present and awg-quick on PATH). When false, callers should
// use the embedded amneziawg-go path instead of awg-quick.
func KernelAvailable() bool {
	kernelAvailOnce.Do(func() {
		kernelAvail = kernelAvailable()
	})
	return kernelAvail
}

// ResetKernelAvailableForTest clears the cached probe. Tests only.
func ResetKernelAvailableForTest() {
	kernelAvailOnce = sync.Once{}
	kernelAvail = false
}
