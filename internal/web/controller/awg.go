// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/awg/cps"
	"github.com/mhsanaei/3x-ui/v3/internal/awg/signature"
)

// awgGenerateObfuscationRequest is the body the AWG inbound form posts to
// /panel/api/inbounds/awg/generateObfuscation. obfProfile selects the
// junk/transport strength (lite/standard/pro); mimicryProfile picks the CPS
// packet shape (tls/dns/sip/quic); region selects the front-domain pool
// (ru/world); domain is an optional explicit front host (empty = random from
// the pool); fullI1I5 reports whether I1-I5 are all emitted (Pro) or just I1
// (Lite/Standard). awgVersion targets the AmneziaWG protocol version
// ("1.5"/"2"/"3"); when "3", the response carries a freshly generated
// HeaderProtectionKey (the AWG3 kernel/tools now parse it).
type awgGenerateObfuscationRequest struct {
	ObfProfile     string `json:"obfProfile"`
	MimicryProfile string `json:"mimicryProfile"`
	BrowserProfile string `json:"browserProfile"`
	Region         string `json:"region"`
	Domain         string `json:"domain"`
	FullI1I5       bool   `json:"fullI1I5"`
	AwgVersion     string `json:"awgVersion"`
}

// awgGenerateObfuscation generates a fresh set of AmneziaWG obfuscation
// parameters (Jc/Jmin/Jmax/S1-S4/H1-H4) and CPS packets (I1-I5) for the AWG
// inbound form. The frontend calls this when the user clicks "generate
// obfuscation" so the panel — not the browser — owns the RNG and the
// invariant-enforcing logic.
//
// LUCX-HOOK: AWG obfuscation generator endpoint.
func (a *InboundController) awgGenerateObfuscation(c *gin.Context) {
	var req awgGenerateObfuscationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid request body", err)
		return
	}
	if req.ObfProfile == "" {
		req.ObfProfile = string(cps.ObfStandard)
	}
	if req.MimicryProfile == "" {
		req.MimicryProfile = string(cps.ProfileTLS)
	}
	if req.Region == "" {
		req.Region = string(cps.RegionWorld)
	}
	if req.BrowserProfile == "" {
		req.BrowserProfile = string(cps.BrowserChrome)
	}
	params, err := cps.GenerateAWGParams(cps.ObfProfile(req.ObfProfile), req.AwgVersion)
	if err != nil {
		jsonMsg(c, "awg obfuscation: bad profile", err)
		return
	}
	cpsResult, err := cps.GenerateCPS(
		cps.MimicryProfile(req.MimicryProfile),
		cps.Region(req.Region),
		req.Domain,
		cps.BrowserProfile(req.BrowserProfile),
		!req.FullI1I5, // GenerateCPS's onlyI1 is the inverse of "full I1-I5"
	)
	if err != nil {
		jsonMsg(c, "awg obfuscation: CPS generation failed", err)
		return
	}
	resp := gin.H{
		"jc":   params.Jc,
		"jmin": params.Jmin,
		"jmax": params.Jmax,
		"s1":   params.S1,
		"s2":   params.S2,
		"s3":   params.S3,
		"s4":   params.S4,
		"h1":   params.H1,
		"h2":   params.H2,
		"h3":   params.H3,
		"h4":   params.H4,
		"i1":   cpsResult.I1,
		"i2":   cpsResult.I2,
		"i3":   cpsResult.I3,
		"i4":   cpsResult.I4,
		"i5":   cpsResult.I5,
	}
	// headerProtectionKey is returned ONLY when awgVersion == "3" AND the host
	// actually runs AWG3 (kernel module + tools, probed functionally by
	// ModuleSupportsAwg3). Generating a key the renderers would then strip
	// leaves a form field that never reaches a .conf — worse, a key an operator
	// copies to an external client implies a server capability the host lacks.
	// feat/awg3 was merged upstream 2026-07-30; GenerateAWGParams already
	// guarantees S1-S4 >= MinSForHPK, so the kernel accepts the key. For
	// v1.5/v2 — or a v3 request on a pre-AWG3 host — the field is omitted (not
	// ""), so the form's Object.entries(obf).forEach(setValue) leaves any
	// hand-typed key untouched — the same property forward-compat relied on.
	if awg.IsAwg3Plus(req.AwgVersion) && awg.ModuleSupportsAwg3() {
		params, err := params.WithHeaderProtectionKey()
		if err != nil {
			jsonMsg(c, "awg obfuscation: header protection key generation failed", err)
			return
		}
		resp["headerProtectionKey"] = params.HeaderProtectionKey
		// AWG 3.0 device timer/padding ranges (lucx.65), generated alongside the
		// HPK so "Regenerate obfuscation" fills the whole v3 block. Keys match the
		// AwgInboundSettingsSchema fields exactly, so the form's blind
		// Object.entries(obf).forEach(setValue) applies them with no handler change.
		// Differentiated by obfProfile exactly like AmneziaWG-Architect.
		timers := cps.GenerateAwg3DeviceTimings(cps.ObfProfile(req.ObfProfile))
		resp["contentPaddingAddition"] = timers.ContentPaddingAddition
		resp["rekeyAfterTime"] = timers.RekeyAfterTime
		resp["rekeyTimeout"] = timers.RekeyTimeout
		resp["rejectAfterTime"] = timers.RejectAfterTime
		resp["keepaliveTimeout"] = timers.KeepaliveTimeout
		resp["maxHandshakeAttempts"] = timers.MaxHandshakeAttempts
	}
	// 3.1 extras (RandomTrailers / DisableCookies) stay off unless the
	// operator flips them. Auto-on RandomTrailers breaks handshake with
	// every current GUI client (Amnezia 5.0.1.1, NekoBox+) — no wire
	// negotiation, silent drop.
	jsonObj(c, resp, nil)
}

// awgCaptureHostRequest is the body the AWG inbound form posts to
// /panel/api/inbounds/awg/captureHost. domain is the front host whose real
// QUIC handshake should be captured and used as the I1-I5 CPS signature.
type awgCaptureHostRequest struct {
	Domain string `json:"domain"`
}

// awgCaptureHost captures a real QUIC handshake from the given domain (UDP
// 443) and returns the I1-I5 packet strings. The user enters a host (e.g.
// google.com) and the AWG traffic is then masked under that host's real
// QUIC-handshake bytes. Ported from hoaxisr/awg-manager. Returns an error
// when the host doesn't speak QUIC — AWG only supports QUIC-fronted hosts.
//
// LUCX-HOOK: AWG host scan endpoint.
func (a *InboundController) awgCaptureHost(c *gin.Context) {
	var req awgCaptureHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, "invalid request body", err)
		return
	}
	if req.Domain == "" {
		jsonMsg(c, "awg capture: domain required", nil)
		return
	}
	res, err := signature.Capture(req.Domain)
	if err != nil {
		jsonMsg(c, "awg capture failed: "+err.Error(), nil)
		return
	}
	jsonObj(c, gin.H{
		"i1": res.I1,
		"i2": res.I2,
		"i3": res.I3,
		"i4": res.I4,
		"i5": res.I5,
	}, nil)
}

// awgDiagnostics probes the live kernel state of an AWG inbound (interface,
// ip_forward, peers/handshakes, and — depending on the mode — MASQUERADE/
// FORWARD rules or the tunN/policy-rule/table chain) and returns the ordered
// check list the panel renders in the AWG diagnostics modal. Read-only: fixes
// belong to the reconcile loop, this endpoint only makes failures visible.
//
// LUCX-HOOK: AWG runtime diagnostics endpoint.
func (a *InboundController) awgDiagnostics(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "awg diagnostics: bad id", err)
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		jsonMsg(c, "awg diagnostics: inbound not found", err)
		return
	}
	inst, ok := awg.InstanceFromInbound(inbound)
	if !ok {
		jsonMsg(c, "awg diagnostics: inbound is not AWG", nil)
		return
	}
	d := awg.Diagnose(inst)
	jsonObj(c, gin.H{
		"ifname":  d.Ifname,
		"mode":    d.Mode,
		"healthy": d.Healthy(),
		"checks":  d.Checks,
	}, nil)
}

// awgTestMtu probes this host's own outbound path MTU (a DF-ping binary
// search against a fixed public anchor) and reports whether the inbound's
// configured MTU fits under it with WireGuard/AWG's own encapsulation
// overhead. This is a ceiling check on the server's uplink only — it cannot
// see the client<->server path, which is why the 1320 "mobile-safe" fallback
// still exists for the operator to pick by hand.
//
// LUCX-HOOK: AWG path-MTU probe endpoint.
func (a *InboundController) awgTestMtu(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "awg test mtu: bad id", err)
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		jsonMsg(c, "awg test mtu: inbound not found", err)
		return
	}
	inst, ok := awg.InstanceFromInbound(inbound)
	if !ok {
		jsonMsg(c, "awg test mtu: inbound is not AWG", nil)
		return
	}
	jsonObj(c, awg.ProbePathMTU(inst.MTU), nil)
}

// END LUCX-HOOK
