//go:build !linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

func anytlsSessionCount(port int) int { return 0 }

func ensureAnytlsAcct(key string, port int) {}

func anytlsByteCounters(key string) (up, down int64, ok bool) { return 0, 0, false }
