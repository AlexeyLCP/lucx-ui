// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	crand "math/rand"
	"strings"
)

// rng is the package-level random source. In Go 1.20+ the global rand is
// automatically seeded, but tests need deterministic output. rand.Seed is
// deprecated; instead tests call SetRand with a seeded source.
var rng = crand.New(crand.NewSource(1))

// SetRand replaces the package-level random source. Used by tests for
// deterministic output; production code leaves the auto-seeded source.
func SetRand(r *crand.Rand) { rng = r }

// MinSForHPK is the minimum value every S1-S4 transport-padding field must
// reach for the AWG3 (AmneziaWG 3) kernel module to accept a non-empty
// HeaderProtectionKey: the cipher reads its 12-byte nonce from the first
// bytes of S-padding, so Sx < 12 makes netlink return -EINVAL on setconf.
// We enforce this lower bound unconditionally so any generated config is
// valid for an AWG3 kernel whether or not HeaderProtectionKey is set.
const MinSForHPK = 12

// enforceSMin lifts v to MinSForHPK when below it. Used by GenerateAWGParams
// so the profile ranges can never produce an Sx that would break AWG3.
func enforceSMin(v int) int {
	if v < MinSForHPK {
		return MinSForHPK
	}
	return v
}

// AWGParams are the junk/transport obfuscation parameters written into the
// awg-quick .conf [Interface] section. Jc/Jmin/Jmax control junk-packet
// insertion ahead of the handshake; S1-S4 are transport padding sizes; H1-H4
// are msgType replacement ranges. HeaderProtectionKey is the AWG3
// (AmneziaWG 3) 32-byte ChaCha20 header-protection key (base64, same shape as
// a WireGuard private key); it is populated only when the caller asks for an
// AWG3 (version "3") config — S1-S4 are always >= MinSForHPK so the key, when
// set, is accepted by the kernel. All fields follow the AmneziaWG spec
// invariants (Jmin < Jmax, |S1+56 − S2| >= 10, H1-H4 in disjoint quadrants).
type AWGParams struct {
	Jc   int
	Jmin int
	Jmax int
	S1   int
	S2   int
	S3   int
	S4   int
	H1   string // "lo-hi"
	H2   string
	H3   string
	H4   string
	// HeaderProtectionKey is the AWG3 header-protection key, base64-encoded
	// (empty when version != "3"). Written to the .conf as
	// `HeaderProtectionKey = <base64>`; the upstream kernel module and tools
	// parse it since v3.0.20260731 / v3.0.20260730.
	HeaderProtectionKey string
	// AWG3 device-level timer/padding fields (all optional — 0 = kernel uses
	// the built-in WireGuard constant). Parsed by amneziawg-tools config.c
	// and the kernel module netlink.c since v3.0.20260731. Written to the
	// .conf only when > 0 AND awgVersion == "3"; emitted as single u16 (the
	// kernel also accepts the "N-M" range form, but the panel exposes one
	// number for simplicity). Defaults/semantics from the kernel messages.h
	// enum limits: RekeyAfterTime=120s, RekeyTimeout=5s, RejectAfterTime=180s,
	// KeepaliveTimeout=10s, MaxHandshakeAttempts=18 (90/REKEY_TIMEOUT),
	// ContentPaddingAddition=0 (deterministic WG padding).
	ContentPaddingAddition int
	RekeyAfterTime         int
	RekeyTimeout           int
	RejectAfterTime        int
	KeepaliveTimeout       int
	MaxHandshakeAttempts   int
}

// randInt returns a random int in [lo, hi] inclusive. lo must be <= hi.
func randInt(lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + rng.Intn(hi-lo+1)
}

// Profile ranges, ported from pumbaX/awg-multi-script gen_awg_params.
// Lite = light obfuscation, Standard = medium, Pro = heavy.
type profileRanges struct {
	jcmin, jcmax   int
	jminLo, jminHi int
	jmaxLo, jmaxHi int
	s1Lo, s1Hi     int
	s2Lo, s2Hi     int
	s3Lo, s3Hi     int
	s4Lo, s4Hi     int
}

func rangesFor(p ObfProfile) (profileRanges, error) {
	switch p {
	case ObfLite:
		// S lower bounds >= MinSForHPK (12) so the AWG3 kernel accepts a
		// HeaderProtectionKey without -EINVAL (the cipher nonce is read from
		// the first 12 bytes of S-padding).
		return profileRanges{3, 5, 5, 15, 45, 55, MinSForHPK, 30, 17, 27, 16, 26, MinSForHPK, 20}, nil
	case ObfStandard:
		return profileRanges{5, 8, 30, 80, 100, 250, 30, 80, 30, 80, 15, 32, MinSForHPK, 24}, nil
	case ObfPro:
		return profileRanges{4, 16, 50, 256, 300, 1000, 15, 150, 15, 150, MinSForHPK, 80, MinSForHPK, 40}, nil
	default:
		return profileRanges{}, errors.New("awg params: unknown profile")
	}
}

// quadrant returns the [lo, hi] bounds for H-index n (0..3). The full H
// range [5, 2^31-1] is split into 4 disjoint quadrants of 2^29 each so H1-H4
// never overlap (matching the AmneziaWG recommended ranges). The upper bound
// is 2^31-1 for Windows-client compatibility.
func quadrant(n int) (lo, hi int) {
	const (
		hMin = 5
		hMax = 2147483647 // 2^31 - 1
		span = 1 << 29    // 2^29
	)
	lo = hMin + n*span
	hi = lo + span - 1
	if n == 3 {
		hi = hMax
	}
	return lo, hi
}

// genHRange returns one "lo-hi" string for quadrant n, with a width >= 1000
// (the AmneziaWG minimum recommended span). Mirrors pumbaX _gen_quadrant_pair.
// Used for AWG version "2" and "3" configs, where H1-H4 are msgType
// replacement *ranges* (the client picks a random value in [lo,hi] per packet).
func genHRange(n int) string {
	qmin, qmax := quadrant(n)
	span := qmax - qmin + 1
	lo := qmin + randInt(0, span/3)
	hi := qmin + 2*span/3 + randInt(0, span/3)
	if hi-lo < 1000 {
		hi = lo + 1000 + randInt(0, 9999)
		if hi > qmax {
			hi = qmax
		}
	}
	return fmt.Sprintf("%d-%d", lo, hi)
}

// genHSingle returns one single-integer string for quadrant n. AWG version
// "1.5" (legacy AmneziaWG 1.x) parses H1-H4 as a fixed msgType replacement
// value — NOT a range — so the .conf line must be `H1 = 1234567`, not
// `H1 = 1234567-2345678`. Older awg-quick (v1.x tools) rejects the "-" form
// with a parse error, which is the user-reported bug ("при выставлении АВГ1.5
// конф выходит формата 2.0"). The value is drawn from the same disjoint
// quadrant as genHRange so H1-H4 never collide and stay >= 5 (the lower bound
// the kernel enforces for type IDs distinct from WireGuard's reserved range).
func genHSingle(n int) string {
	qmin, qmax := quadrant(n)
	span := qmax - qmin + 1
	val := qmin + randInt(0, span-1)
	return fmt.Sprintf("%d", val)
}

// GenerateAWGParams produces a fresh set of junk/transport obfuscation
// parameters for the given strength profile. It enforces the AmneziaWG
// invariants: Jmin < Jmax (fixed by lifting Jmax), |S1+56 − S2| >= 10 (retry
// S2 up to 10 times, then shift it), and S1-S4 >= MinSForHPK (so the AWG3
// kernel accepts a HeaderProtectionKey without -EINVAL). H1-H4 are each in
// their own quadrant, so they never collide. The profile ranges already
// enforceSMin, but GenerateAWGParams clamps again as a belt-and-braces guard
// for hand-edited ranges or future profile additions.
//
// awgVersion selects the H1-H4 wire format:
//   - "1.5": legacy AmneziaWG 1.x — H1-H4 are single integers (the v1 awg-quick
//     parser rejects the "lo-hi" range form). See genHSingle.
//   - "2" or "3": H1-H4 are "lo-hi" ranges (the v2+ form, also accepted by the
//     AWG3 kernel/tools). See genHRange.
//
// An empty awgVersion is treated as "2" (the project default; see
// awg.NormalizeAWGVersion), so callers that don't care keep the historical
// range behaviour. The caller is responsible for adding a HeaderProtectionKey
// (via WithHeaderProtectionKey) only when awgVersion == "3".
func GenerateAWGParams(profile ObfProfile, awgVersion string) (AWGParams, error) {
	r, err := rangesFor(profile)
	if err != nil {
		return AWGParams{}, err
	}
	jc := randInt(r.jcmin, r.jcmax)
	jmin := randInt(r.jminLo, r.jminHi)
	jmax := randInt(r.jmaxLo, r.jmaxHi)
	if jmax <= jmin {
		jmax = jmin + randInt(100, 500)
	}
	s1 := enforceSMin(randInt(r.s1Lo, r.s1Hi))
	s2 := enforceSMin(randInt(r.s2Lo, r.s2Hi))
	// AmneziaWG requires S1 + 56 != S2; pumbaX strengthens this to a >=10 gap.
	for i := 0; i < 10 && abs((s1+56)-s2) < 10; i++ {
		s2 = enforceSMin(randInt(r.s2Lo, r.s2Hi))
	}
	if abs((s1+56)-s2) < 10 {
		if s2 >= s1+56 {
			s2 += 10
		} else {
			s2 -= 10
		}
	}
	s3 := enforceSMin(randInt(r.s3Lo, r.s3Hi))
	s4 := enforceSMin(randInt(r.s4Lo, r.s4Hi))
	var h1, h2, h3, h4 string
	if awgVersion == "1.5" {
		h1, h2, h3, h4 = genHSingle(0), genHSingle(1), genHSingle(2), genHSingle(3)
	} else {
		h1, h2, h3, h4 = genHRange(0), genHRange(1), genHRange(2), genHRange(3)
	}
	return AWGParams{
		Jc:   jc,
		Jmin: jmin,
		Jmax: jmax,
		S1:   s1,
		S2:   s2,
		S3:   s3,
		S4:   s4,
		H1:   h1,
		H2:   h2,
		H3:   h3,
		H4:   h4,
	}, nil
}

// WithHeaderProtectionKey returns a copy of the params with a freshly
// generated AWG3 HeaderProtectionKey. Callers must only invoke this for an
// AWG version-"3" inbound; S1-S4 are already >= MinSForHPK from
// GenerateAWGParams, so the kernel will accept the key. Returns an error only
// if the system RNG fails (effectively never).
func (p AWGParams) WithHeaderProtectionKey() (AWGParams, error) {
	k, err := GenerateHeaderProtectionKey()
	if err != nil {
		return p, err
	}
	p.HeaderProtectionKey = k
	return p, nil
}

// GenerateHeaderProtectionKey returns a fresh AWG3 header-protection key:
// 32 random bytes, base64-encoded (44 chars, same shape as a WireGuard
// private key). Uses crypto/rand so the key is cryptographically strong and
// independent of the deterministic test RNG.
func GenerateHeaderProtectionKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("awg: generate header protection key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}

// AsConfLines returns the AWGParams as awg-quick .conf lines (Jc = …, H1 = lo-hi,
// …) suitable for the [Interface] section. The fields are emitted in the
// canonical AmneziaWG order. HeaderProtectionKey is emitted only when set —
// the AWG3 kernel/tools (v3.0.20260731 / v3.0.20260730) parse it; older
// builds reject the line, so callers must leave it empty for non-v3 configs.
func (p AWGParams) AsConfLines() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Jc = %d\n", p.Jc)
	fmt.Fprintf(&b, "Jmin = %d\n", p.Jmin)
	fmt.Fprintf(&b, "Jmax = %d\n", p.Jmax)
	fmt.Fprintf(&b, "S1 = %d\n", p.S1)
	fmt.Fprintf(&b, "S2 = %d\n", p.S2)
	fmt.Fprintf(&b, "S3 = %d\n", p.S3)
	fmt.Fprintf(&b, "S4 = %d\n", p.S4)
	fmt.Fprintf(&b, "H1 = %s\n", p.H1)
	fmt.Fprintf(&b, "H2 = %s\n", p.H2)
	fmt.Fprintf(&b, "H3 = %s\n", p.H3)
	fmt.Fprintf(&b, "H4 = %s\n", p.H4)
	if p.HeaderProtectionKey != "" {
		fmt.Fprintf(&b, "HeaderProtectionKey = %s\n", p.HeaderProtectionKey)
	}
	if p.ContentPaddingAddition > 0 {
		fmt.Fprintf(&b, "ContentPaddingAddition = %d\n", p.ContentPaddingAddition)
	}
	if p.RekeyAfterTime > 0 {
		fmt.Fprintf(&b, "RekeyAfterTime = %d\n", p.RekeyAfterTime)
	}
	if p.RekeyTimeout > 0 {
		fmt.Fprintf(&b, "RekeyTimeout = %d\n", p.RekeyTimeout)
	}
	if p.RejectAfterTime > 0 {
		fmt.Fprintf(&b, "RejectAfterTime = %d\n", p.RejectAfterTime)
	}
	if p.KeepaliveTimeout > 0 {
		fmt.Fprintf(&b, "KeepaliveTimeout = %d\n", p.KeepaliveTimeout)
	}
	if p.MaxHandshakeAttempts > 0 {
		fmt.Fprintf(&b, "MaxHandshakeAttempts = %d\n", p.MaxHandshakeAttempts)
	}
	return b.String()
}

// Validate enforces the AmneziaWG invariants on already-stored params (for
// migration / self-check). Returns nil when valid.
func (p AWGParams) Validate() error {
	if p.Jmax <= p.Jmin {
		return fmt.Errorf("awg: Jmax (%d) must be > Jmin (%d)", p.Jmax, p.Jmin)
	}
	if abs((p.S1+56)-p.S2) < 10 {
		return fmt.Errorf("awg: |S1+56 − S2| must be >= 10 (S1=%d S2=%d)", p.S1, p.S2)
	}
	// S1-S4 must be >= MinSForHPK so an AWG3 (version "3") kernel accepts a
	// HeaderProtectionKey. We enforce this unconditionally because a config
	// generated for v2 today may be promoted to v3 tomorrow by editing only
	// the version, and a too-small S would then break reconcile with -EINVAL.
	for _, s := range []int{p.S1, p.S2, p.S3, p.S4} {
		if s < MinSForHPK {
			return fmt.Errorf("awg: S values must be >= %d for AWG3 header-protection compatibility (got %d)", MinSForHPK, s)
		}
	}
	for _, v := range []struct {
		name string
		val  int
	}{
		{"ContentPaddingAddition", p.ContentPaddingAddition},
		{"RekeyAfterTime", p.RekeyAfterTime},
		{"RekeyTimeout", p.RekeyTimeout},
		{"RejectAfterTime", p.RejectAfterTime},
		{"KeepaliveTimeout", p.KeepaliveTimeout},
		{"MaxHandshakeAttempts", p.MaxHandshakeAttempts},
	} {
		if v.val < 0 || v.val > 65535 {
			return fmt.Errorf("awg: %s (%d) must be 0-65535 (u16 range, 0 = kernel default)", v.name, v.val)
		}
	}
	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
