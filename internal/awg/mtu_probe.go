// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"strconv"
	"strings"
)

// pathMTUProbeTarget is a stable, well-known anycast IP used only to measure
// this host's own outbound path MTU — never traffic-carrying. Same anchor
// already used as the AWG DNS default elsewhere in the panel.
const pathMTUProbeTarget = "1.1.1.1"

// pathMTULo/pathMTUHi bound the binary search: 576 is the IPv4 minimum MTU,
// 1500 the typical Ethernet MTU. Jumbo-frame paths above 1500 are not worth
// discovering automatically — that needs a manual override anyway.
const (
	pathMTULo = 576
	pathMTUHi = 1500
)

// icmpOverhead is the IPv4(20) + ICMP(8) header bytes added on top of a ping
// payload to get the resulting on-wire packet size, i.e. the equivalent
// interface MTU.
const icmpOverhead = 28

// wgOverhead approximates WireGuard/AmneziaWG's own encapsulation on top of
// the path MTU: 20 (outer IPv4) + 8 (outer UDP) + 32 (WG data header) = 60,
// rounded up for AWG's junk/padding fields. A configured MTU that doesn't
// leave this much headroom under the measured path MTU risks the outer UDP
// packet fragmenting — which shows up as stalled or crawling transfers, not
// a visible error, so PathMTUCheck exists to catch it ahead of a support
// ticket.
const wgOverhead = 80

// PathMTUResult is the outcome of a server-side path-MTU probe: the largest
// DF (don't-fragment) ICMP echo that reached the anchor, translated to an
// interface-MTU ceiling, and whether a given configured MTU fits under it
// with WireGuard/AWG's own overhead. This measures the server's OWN outbound
// path — a ceiling on what an AWG inbound's MTU can safely be, since the
// tunnel's UDP packets ride on top of it. It does NOT measure the
// client<->server path, which is usually equal or larger on the client's
// home network but can be smaller behind mobile/CGNAT — hence the 1320
// fallback the form still offers.
type PathMTUResult struct {
	Target        string `json:"target"`
	PathMTU       int    `json:"pathMtu"`
	Reached       bool   `json:"reached"`
	ConfiguredMTU int    `json:"configuredMtu"`
	Fits          bool   `json:"fits"`
	Detail        string `json:"detail"`
}

// ProbePathMTU binary-searches the path MTU to pathMTUProbeTarget and reports
// whether configuredMTU fits under it with wgOverhead headroom to spare.
func ProbePathMTU(configuredMTU int) PathMTUResult {
	return probePathMTU(execProber{}, pathMTUProbeTarget, configuredMTU)
}

func probePathMTU(p prober, target string, configuredMTU int) PathMTUResult {
	lo, hi := pathMTULo-icmpOverhead, pathMTUHi-icmpOverhead
	best := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if pingDF(p, target, mid) {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best == 0 {
		return PathMTUResult{
			Target:        target,
			ConfiguredMTU: configuredMTU,
			Detail:        fmt.Sprintf("no reply from %s even at the minimum size — host may be firewalled or offline", target),
		}
	}
	pathMTU := best + icmpOverhead
	fits := configuredMTU+wgOverhead <= pathMTU
	fitWord := "fits within"
	if !fits {
		fitWord = "EXCEEDS"
	}
	return PathMTUResult{
		Target:        target,
		PathMTU:       pathMTU,
		Reached:       true,
		ConfiguredMTU: configuredMTU,
		Fits:          fits,
		Detail: fmt.Sprintf("largest non-fragmented ping to %s = %d bytes; configured MTU %d %s the %d-byte WG/AWG overhead headroom",
			target, pathMTU, configuredMTU, fitWord, wgOverhead),
	}
}

// pingDF sends one DF (don't-fragment) ICMP echo of the given payload size
// and reports whether it got a reply. `ping -M do` (Linux iputils) sets DF
// and refuses to fragment locally, so an oversized packet either reports
// "Frag needed" or simply gets no reply — both read as failure here.
func pingDF(p prober, target string, payload int) bool {
	out, err := p.Run("ping", "-M", "do", "-c", "1", "-W", "1", "-s", strconv.Itoa(payload), target)
	return err == nil && strings.Contains(out, " 0% packet loss")
}
