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

// AmneziaWG message sizes before S1-S4 padding — the fixed overhead each
// packet type carries. From amnezia-client's AwgConstant. GenerateAWGParams
// keeps the four total sizes (overhead + S) pairwise distinct because the
// receiver's awg_determine_type_and_padding disambiguates packet types by
// length; two equal total sizes would make it mis-classify and weaken the
// masking (this is AmneziaVPN's isPacketSizeEqual invariant).
const (
	messageInitiationSize  = 148
	messageResponseSize    = 92
	messageCookieReplySize = 64
	messageTransportSize   = 32
)

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

// Profile ranges, calibrated against AmneziaVPN's own generator
// (AwgInstaller::generateAwgParameters, amnezia-client): one conservative
// baseline, scaled into three tiers. Amnezia ships Jc=4..6, Jmin=10, Jmax=50,
// S1/S2 = 12..149, S3 = 12..63, and a FIXED S4 = 12 (transport junk size —
// the padding added to every data packet, which is why it stays small). The
// old ranges were ported from pumbaX/awg-multi-script and let Jmax climb to
// 1000 and S4 to 40, which operators flagged as "over-obfuscation".
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
		return profileRanges{2, 4, 5, 10, 30, 50, MinSForHPK, 64, MinSForHPK, 64, MinSForHPK, 32, MinSForHPK, MinSForHPK}, nil
	case ObfStandard:
		return profileRanges{4, 6, 10, 10, 50, 50, MinSForHPK, 149, MinSForHPK, 149, MinSForHPK, 63, MinSForHPK, MinSForHPK}, nil
	case ObfPro:
		return profileRanges{5, 8, 10, 15, 60, 100, 32, 150, 32, 150, 16, 64, MinSForHPK, MinSForHPK}, nil
	default:
		return profileRanges{}, errors.New("awg params: unknown profile")
	}
}

// hBandWidth is the span of each H's disjoint band. Modest on purpose: the
// panel used to draw each H from a full 2^29 quadrant (pumbaX), producing
// giant "lo-hi" magic-header ranges that operators flagged as
// over-obfuscation. AmneziaVPN keeps H tiny ("1"/"2"/"3"/"4") and relies on
// the header protection key + junk packets; we keep small disjoint bands so
// that AWG v2 (which has no header protection key) still gets a random header
// value without the 2^29 sprawl.
const hBandWidth = 100000

// hBand returns the inclusive [lo, hi] bounds for H-index n (0..3). Each H
// lives in its own disjoint band so H1-H4 never collide and stay >= 5 (the
// lower bound the kernel enforces for type IDs distinct from WireGuard's
// reserved 1-4 range).
func hBand(n int) (lo, hi int) {
	lo = 5 + n*hBandWidth
	hi = lo + hBandWidth - 6
	return lo, hi
}

// genHRange returns one "lo-hi" string within H-band n, width >= 1000 (the
// AmneziaWG minimum recommended span). Used for AWG version "2" configs: with
// no header protection key the magic header value is the only header-layer
// obfuscation, so it is drawn as a random — but narrow — range.
func genHRange(n int) string {
	lo, hi := hBand(n)
	span := hi - lo + 1
	left := lo + randInt(0, span/3)
	right := lo + 2*span/3 + randInt(0, span/3)
	if right-left < 1000 {
		right = left + 1000 + randInt(0, 9999)
		if right > hi {
			right = hi
		}
	}
	return fmt.Sprintf("%d-%d", left, right)
}

// genHSingle returns one single-integer string within H-band n. AWG version
// "1.5" (legacy AmneziaWG 1.x) parses H1-H4 as a fixed msgType replacement
// value — NOT a range — so the .conf line must be `H1 = 1234567`, not
// `H1 = 1234567-2345678`. Older awg-quick (v1.x tools) rejects the "-" form
// with a parse error, which is the user-reported bug ("при выставлении АВГ1.5
// конф выходит формата 2.0"). The value is drawn from the same disjoint band
// as genHRange so H1-H4 never collide and stay >= 5.
func genHSingle(n int) string {
	lo, hi := hBand(n)
	return fmt.Sprintf("%d", randInt(lo, hi))
}

// genHDefault returns the AmneziaVPN magic-header default for index n ("1".."4").
// Used for AWG "3"/"3.1": the header protection key encrypts the message
// header, so the magic values are left at the plain WireGuard types — exactly
// as AmneziaVPN's generator does (it never randomizes H).
func genHDefault(n int) string {
	return []string{"1", "2", "3", "4"}[n]
}

// GenerateAWGParams produces a fresh set of junk/transport obfuscation
// parameters for the given strength profile. It enforces the AmneziaWG
// invariants: Jmin < Jmax (fixed by lifting Jmax), S1-S4 >= MinSForHPK (so
// the AWG3 kernel accepts a HeaderProtectionKey without -EINVAL), and — like
// AmneziaVPN's AwgInstaller::generateAwgParameters — all four total packet
// sizes stay pairwise distinct: 148+S1 (init), 92+S2 (response), 64+S3
// (cookie reply), 32+S4 (transport). awg_determine_type_and_padding issues
// the packet type from the length, so two equal total sizes would make the
// receiver mis-classify and weaken the masking.
//
// awgVersion selects the H1-H4 wire format:
//   - "1.5": single integers in disjoint narrow bands (the v1 awg-quick parser
//     rejects the "lo-hi" form). See genHSingle.
//   - "2": "lo-hi" ranges in disjoint narrow bands (no header protection key,
//     so the magic header is the only header-layer obfuscation). See genHRange.
//   - "3"/"3.1": the plain WireGuard defaults "1"/"2"/"3"/"4" — the header
//     protection key encrypts the header, so the magic value is irrelevant; this
//     matches AmneziaVPN's generator, which never randomizes H.
//
// An empty awgVersion is treated as "2" (the project default; see
// awg.NormalizeAWGVersion). The caller is responsible for adding a
// HeaderProtectionKey (via WithHeaderProtectionKey) only when a version "3"
// or "3.1" inbound is generated.
func GenerateAWGParams(profile ObfProfile, awgVersion string) (AWGParams, error) {
	r, err := rangesFor(profile)
	if err != nil {
		return AWGParams{}, err
	}
	jc := randInt(r.jcmin, r.jcmax)
	jmin := randInt(r.jminLo, r.jminHi)
	jmax := randInt(r.jmaxLo, r.jmaxHi)
	if jmax <= jmin {
		jmax = jmin + randInt(20, 60)
	}
	s1 := enforceSMin(randInt(r.s1Lo, r.s1Hi))
	s4 := enforceSMin(randInt(r.s4Lo, r.s4Hi))
	s2 := enforceSMin(randInt(r.s2Lo, r.s2Hi))
	s3 := enforceSMin(randInt(r.s3Lo, r.s3Hi))
	// Amnezia's collision guard: every S is unique and the total size of each
	// packet type stays distinct. The loops retry within the same profile band;
	// the bands are wide enough that a collision resolves in one iteration.
	for i := 0; i < 50 && (s2 == s1 || s2 == s4 || messageInitiationSize+s1 == messageResponseSize+s2); i++ {
		s2 = enforceSMin(randInt(r.s2Lo, r.s2Hi))
	}
	for i := 0; i < 50 && (s3 == s1 || s3 == s2 || s3 == s4 ||
		messageInitiationSize+s1 == messageCookieReplySize+s3 ||
		messageResponseSize+s2 == messageCookieReplySize+s3); i++ {
		s3 = enforceSMin(randInt(r.s3Lo, r.s3Hi))
	}
	var h1, h2, h3, h4 string
	switch awgVersion {
	case "1.5":
		h1, h2, h3, h4 = genHSingle(0), genHSingle(1), genHSingle(2), genHSingle(3)
	case "3", "3.1":
		h1, h2, h3, h4 = genHDefault(0), genHDefault(1), genHDefault(2), genHDefault(3)
	default:
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

// Awg3DeviceTimings are the AWG 3.0 device-level timer/padding values as
// ready-to-store "lo-hi" range strings (a single "lo" when lo==hi), matching
// the kernel u16_range_t form. Empty means "not generated".
type Awg3DeviceTimings struct {
	ContentPaddingAddition string
	RekeyAfterTime         string
	RekeyTimeout           string
	RejectAfterTime        string
	KeepaliveTimeout       string
	MaxHandshakeAttempts   string
}

// awg3Range formats an inclusive [lo, hi] the way the kernel u16_range_t
// parser accepts: "lo" when lo==hi, otherwise "lo-hi". hi is never below lo.
func awg3Range(lo, hi int) string {
	if lo < 0 {
		lo = 0
	}
	if hi < lo {
		hi = lo
	}
	if lo == hi {
		return fmt.Sprintf("%d", lo)
	}
	return fmt.Sprintf("%d-%d", lo, hi)
}

// GenerateAwg3DeviceTimings produces randomised AWG 3.0 device timer/padding
// ranges, ported from AmneziaWG-Architect (src/engines/awg/generator/awg3.ts),
// which derives them from the amneziawg-go v3.0.1 implementation rather than
// the (still 2.0) docs. Only called for awgVersion "3" on an AWG3-capable
// host (the generateObfuscation controller gates it alongside the HPK).
//
// Profile maps to Architect intensity (lite→low, standard→medium, pro→high).
// Three fields scale with intensity (ContentPaddingAddition, RekeyAfterTime,
// RejectAfterTime — wider jitter flattens the handshake-cadence / packet-size
// fingerprint harder at the cost of bandwidth); the other three (RekeyTimeout,
// KeepaliveTimeout, MaxHandshakeAttempts) stay on fixed safe spans because the
// WireGuard timer invariants constrain them:
//
//	RejectAfterTime must exceed KeepaliveTimeout+RekeyTimeout or the receiving
//	side's key-refresh window collapses to zero; RekeyAfterTime must finish
//	before RejectAfterTime; MaxHandshakeAttempts >= 1.
func GenerateAwg3DeviceTimings(profile ObfProfile) Awg3DeviceTimings {
	// Intensity knobs, keyed by profile. padLo/padHi bound ContentPaddingAddition;
	// spread is the timing-jitter width.
	var padLo, padHi, spread int
	switch profile {
	case ObfLite:
		padLo, padHi, spread = 8, 64, 10
	case ObfPro:
		padLo, padHi, spread = 24, 200, 45
	default: // ObfStandard and anything unrecognised
		padLo, padHi, spread = 16, 128, 25
	}

	// ContentPaddingAddition: random per-transport-packet extra padding range.
	padMin := randInt(padLo, (padLo+padHi)/2)
	contentPadding := awg3Range(padMin, randInt(padMin+8, padHi))

	// RekeyTimeout and KeepaliveTimeout: fixed safe spans (protocol-constrained).
	rekeyTimeoutLo := randInt(4, 6)
	rekeyTimeoutHi := rekeyTimeoutLo + randInt(1, 4)
	keepaliveLo := randInt(8, 14)
	keepaliveHi := keepaliveLo + randInt(2, 8)

	// RekeyAfterTime: base window with intensity-scaled jitter on the high end.
	rekeyAfterLo := randInt(100, 120)
	rekeyAfterHi := rekeyAfterLo + randInt(10, spread)

	// RejectAfterTime: keep a hard margin over the receiving-side refresh window
	// (keepaliveHi + rekeyTimeoutHi) plus rekeyAfterHi so it can never collapse.
	rejectFloor := rekeyAfterHi + keepaliveHi + rekeyTimeoutHi + 15
	rejectLo := rejectFloor
	if rejectLo < 170 {
		rejectLo = 170
	}
	rejectHi := rejectLo + randInt(10, spread)

	// MaxHandshakeAttempts: fixed safe span, always >= 1.
	attemptsLo := randInt(12, 18)
	attemptsHi := attemptsLo + randInt(2, 10)

	return Awg3DeviceTimings{
		ContentPaddingAddition: contentPadding,
		RekeyAfterTime:         awg3Range(rekeyAfterLo, rekeyAfterHi),
		RekeyTimeout:           awg3Range(rekeyTimeoutLo, rekeyTimeoutHi),
		RejectAfterTime:        awg3Range(rejectLo, rejectHi),
		KeepaliveTimeout:       awg3Range(keepaliveLo, keepaliveHi),
		MaxHandshakeAttempts:   awg3Range(attemptsLo, attemptsHi),
	}
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
	// All four total packet sizes must be pairwise distinct (Amnezia's
	// isPacketSizeEqual invariant): the receiver disambiguates packet types by
	// length, so two equal sizes would mis-classify and weaken the masking.
	sizes := []int{
		messageInitiationSize + p.S1,
		messageResponseSize + p.S2,
		messageCookieReplySize + p.S3,
		messageTransportSize + p.S4,
	}
	for i := 0; i < len(sizes); i++ {
		for j := i + 1; j < len(sizes); j++ {
			if sizes[i] == sizes[j] {
				return fmt.Errorf("awg: packet sizes must be pairwise distinct (init=%d response=%d cookie=%d transport=%d)", sizes[0], sizes[1], sizes[2], sizes[3])
			}
		}
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
