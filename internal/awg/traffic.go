// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Traffic is a per-inbound traffic delta scraped from `awg show <iface> transfer`.
// Up is bytes from client to server (rx on the server); Down is server to
// client (tx on the server). The tag matches the inbound's Xray tag so the
// delta can be folded into the standard inbound traffic accounting.
type Traffic struct {
	Tag  string
	Up   int64
	Down int64
}

// scrapeTransfer runs `awg show <iface> transfer` and parses the per-peer byte
// counters. Output format is one line per peer:
//
//	<peer-pubkey>\t<rx-bytes>\t<tx-bytes>
//
// rx is bytes received from the peer (upload from the client's perspective);
// tx is bytes sent to the peer (download). Returns the summed rx/tx across all
// peers and ok=false if the interface is down or awg is unavailable.
func scrapeTransfer(ifname string) (rx, tx int64, ok bool) {
	cmd := exec.Command("awg", "show", ifname, "transfer")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, false
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	found := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		r, errR := strconv.ParseInt(fields[1], 10, 64)
		t, errT := strconv.ParseInt(fields[2], 10, 64)
		if errR != nil || errT != nil {
			continue
		}
		rx += r
		tx += t
		found = true
	}
	if err := scanner.Err(); err != nil {
		return rx, tx, found
	}
	return rx, tx, found
}

// formatTransfer is a small helper for diagnostics: it returns a human-readable
// summary of the current rx/tx counters for an interface.
func formatTransfer(ifname string) string {
	rx, tx, ok := scrapeTransfer(ifname)
	if !ok {
		return fmt.Sprintf("%s: <unavailable>", ifname)
	}
	return fmt.Sprintf("%s: rx=%d tx=%d", ifname, rx, tx)
}