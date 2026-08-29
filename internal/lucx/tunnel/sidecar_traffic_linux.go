//go:build linux

// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func olcrtcProcIO(pid int) (rchar, wchar int64, ok bool) {
	if pid <= 0 {
		return 0, 0, false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/io")
	if err != nil {
		return 0, 0, false
	}
	rchar, wchar = parseProcIO(string(b))
	return rchar, wchar, true
}

func qwdttIfaceStats() (rx, tx int64, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	any := false
	for _, iface := range []string{qwdttIfaceWG, qwdttIfaceRaw} {
		out, err := exec.CommandContext(ctx, "ip", "-s", "link", "show", iface).Output()
		if err != nil {
			continue
		}
		r, t := parseIpLinkStats(string(out))
		rx += r
		tx += t
		any = true
	}
	return rx, tx, any
}
