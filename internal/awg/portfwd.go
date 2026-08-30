// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

const maxForwardedPorts = 100

func parseForwardedPorts(input string) []int {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	input = strings.ReplaceAll(input, ";", ",")
	seen := map[int]struct{}{}
	var out []int
	for _, tok := range strings.Split(input, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		lo, hi, ok := parsePortToken(tok)
		if !ok {
			continue
		}
		for p := lo; p <= hi; p++ {
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
			if len(out) >= maxForwardedPorts {
				return out
			}
		}
	}
	return out
}

func parsePortToken(tok string) (lo, hi int, ok bool) {
	if i := strings.IndexByte(tok, '-'); i >= 0 {
		a, ok1 := parsePortNum(strings.TrimSpace(tok[:i]))
		b, ok2 := parsePortNum(strings.TrimSpace(tok[i+1:]))
		if !ok1 || !ok2 || a > b {
			return 0, 0, false
		}
		return a, b, true
	}
	p, ok := parsePortNum(tok)
	if !ok {
		return 0, 0, false
	}
	return p, p, true
}

func parsePortNum(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 || n == 22 {
		return 0, false
	}
	return n, true
}

func firstIPv4Host(allowed string) string {
	for _, part := range strings.Split(allowed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(part); err == nil {
			if prefix.Addr().Is4() {
				return prefix.Addr().String()
			}
			continue
		}
		if addr, err := netip.ParseAddr(part); err == nil && addr.Is4() {
			return addr.String()
		}
	}
	return ""
}

func portFwdComment(id int) string {
	return fmt.Sprintf("lucx-awg-fwd-%d", id)
}
