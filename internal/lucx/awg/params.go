// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// AWGParams holds all configuration parameters for an AWG interface.
type AWGParams struct {
	PrivateKey     string `json:"privateKey"`
	PublicKey      string `json:"publicKey"`
	PresharedKey   string `json:"presharedKey"`
	MTU            int    `json:"mtu"`
	Jc             int    `json:"jc"`
	Jmin           int    `json:"jmin"`
	Jmax           int    `json:"jmax"`
	S1             int    `json:"s1"`
	S2             int    `json:"s2"`
	S3             int    `json:"s3"`
	S4             int    `json:"s4"`
	H1             string `json:"h1"`
	H2             string `json:"h2"`
	H3             string `json:"h3"`
	H4             string `json:"h4"`
	ObfLevel       int    `json:"obfLevel"`
	MimicryProfile string `json:"mimicryProfile"`
	Region         string `json:"region"`
}

var validProfiles = map[string]bool{"quic": true, "sip": true, "dns": true}

func GenerateAWGParams(obfLevel int, profile string, region string) (*AWGParams, error) {
	if obfLevel < 1 || obfLevel > 3 {
		return nil, fmt.Errorf("obfLevel must be 1-3, got %d", obfLevel)
	}
	if !validProfiles[profile] {
		return nil, fmt.Errorf("invalid mimicry profile: %s", profile)
	}

	privKey := genKey()
	pubKey := DerivePubkey(privKey)
	psk := genPSK()

	params := &AWGParams{
		PrivateKey:     privKey,
		PublicKey:      pubKey,
		PresharedKey:   psk,
		MTU:            1320,
		ObfLevel:       obfLevel,
		MimicryProfile: profile,
		Region:         region,
	}

	params.Jc = randInt(4, 16)
	params.Jmin = randInt(50, 256)
	params.Jmax = randInt(300, 1000)
	if params.Jmin >= params.Jmax {
		params.Jmax = params.Jmin + randInt(100, 500)
	}

	params.S1 = randInt(15, 150)
	params.S2 = randInt(15, 150)
	for attempts := 0; params.S1+56 == params.S2 && attempts < 10; attempts++ {
		params.S2 = randInt(15, 150)
	}
	params.S3 = randInt(8, 64)
	params.S4 = randInt(6, 31)

	params.H1 = genQuadrantRange(1)
	params.H2 = genQuadrantRange(2)
	params.H3 = genQuadrantRange(3)
	params.H4 = genQuadrantRange(4)

	return params, nil
}

// awgGenKey runs `awg genkey` to produce a proper Curve25519 private key.
// Falls back to `wg genkey` if awg is not installed.
// Random bytes are NOT valid WireGuard keys — Curve25519 requires clamping.
func awgGenKey() string {
	cmd := exec.Command("awg", "genkey")
	out, err := cmd.Output()
	if err == nil && len(out) == 44 {
		return strings.TrimSpace(string(out))
	}
	// Fallback: try wg genkey
	cmd = exec.Command("wg", "genkey")
	out, err = cmd.Output()
	if err == nil && len(out) == 44 {
		return strings.TrimSpace(string(out))
	}
	// Last resort: random bytes with clamping (not ideal but better than nothing)
	key := make([]byte, 32)
	rand.Read(key)
	// Curve25519 clamping
	key[0] &= 248
	key[31] &= 127
	key[31] |= 64
	return base64.StdEncoding.EncodeToString(key)
}

func genKey() string { return awgGenKey() }

// GenKey generates a proper Curve25519 private key via awg/wg genkey.
func GenKey() string { return awgGenKey() }

func genPSK() string {
	psk := make([]byte, 32)
	rand.Read(psk)
	return base64.StdEncoding.EncodeToString(psk)
}

// GenPSK generates a random 32-byte standard base64 pre-shared key.
// PSK is a shared secret, not a Curve25519 key — random bytes are fine.
func GenPSK() string { return genPSK() }

// DerivePubkey derives a Curve25519 public key from a private key via awg pubkey.
func DerivePubkey(privKey string) string {
	cmd := exec.Command("awg", "pubkey")
	cmd.Stdin = strings.NewReader(privKey)
	out, err := cmd.Output()
	if err == nil && len(out) == 44 {
		return strings.TrimSpace(string(out))
	}
	// Fallback: try wg pubkey
	cmd = exec.Command("wg", "pubkey")
	cmd.Stdin = strings.NewReader(privKey)
	out, err = cmd.Output()
	if err == nil && len(out) == 44 {
		return strings.TrimSpace(string(out))
	}
	// Last resort — return empty, will be caught by validation
	return ""
}

// FromURLSafeKey converts a URL-safe base64 key back to standard base64 with padding.
// Used when receiving client keys from API path parameters (Gin can't handle %2F).
func FromURLSafeKey(urlSafe string) string {
	std := strings.ReplaceAll(strings.ReplaceAll(urlSafe, "-", "+"), "_", "/")
	switch len(std) % 4 {
	case 2:
		std += "=="
	case 3:
		std += "="
	}
	return std
}

func randInt(min, max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return min + int(n.Int64())
}

func genQuadrantRange(quadrant int) string {
	var base int64
	switch quadrant {
	case 1:
		base = 5
	case 2:
		base = 536870912
	case 3:
		base = 1073741824
	case 4:
		base = 1610612736
	default:
		base = 5
	}
	third := int64(178956970)
	lo := base + int64(randInt(0, int(third/3)))
	hi := base + third - int64(randInt(0, int(third/3)))
	if hi-lo < 1000 {
		hi = lo + 1000
	}
	return fmt.Sprintf("%d-%d", lo, hi)
}

// ValidateAWGParams checks all obfuscation parameters are within valid ranges.
// Returns nil if valid, or an error describing the first violation found.
func ValidateAWGParams(params *AWGParams) error {
	if params.Jc < 1 || params.Jc > 128 {
		return fmt.Errorf("jc out of range [1,128]: %d", params.Jc)
	}
	if params.Jmin < 1 || params.Jmin > 2000 {
		return fmt.Errorf("jmin out of range [1,2000]: %d", params.Jmin)
	}
	if params.Jmax < 1 || params.Jmax > 2000 {
		return fmt.Errorf("jmax out of range [1,2000]: %d", params.Jmax)
	}
	if params.Jmin >= params.Jmax {
		return fmt.Errorf("jmin (%d) must be < jmax (%d)", params.Jmin, params.Jmax)
	}
	if params.S1 < 1 || params.S1 > 256 {
		return fmt.Errorf("s1 out of range [1,256]: %d", params.S1)
	}
	if params.S2 < 1 || params.S2 > 256 {
		return fmt.Errorf("s2 out of range [1,256]: %d", params.S2)
	}
	if params.S3 < 1 || params.S3 > 256 {
		return fmt.Errorf("s3 out of range [1,256]: %d", params.S3)
	}
	if params.S4 < 1 || params.S4 > 256 {
		return fmt.Errorf("s4 out of range [1,256]: %d", params.S4)
	}
	if params.S1+56 == params.S2 {
		return fmt.Errorf("s1+56 must not equal s2 (DPI detection risk): s1=%d s2=%d", params.S1, params.S2)
	}
	if params.S2+56 == params.S3 {
		return fmt.Errorf("s2+56 must not equal s3 (DPI detection risk): s2=%d s3=%d", params.S2, params.S3)
	}
	if params.S3+56 == params.S4 {
		return fmt.Errorf("s3+56 must not equal s4 (DPI detection risk): s3=%d s4=%d", params.S3, params.S4)
	}
	if err := validateHRange("h1", params.H1, 5, 536870911); err != nil {
		return err
	}
	if err := validateHRange("h2", params.H2, 536870912, 1073741823); err != nil {
		return err
	}
	if err := validateHRange("h3", params.H3, 1073741824, 1610612735); err != nil {
		return err
	}
	if err := validateHRange("h4", params.H4, 1610612736, 2147483647); err != nil {
		return err
	}
	if params.MTU < 1000 || params.MTU > 1500 {
		return fmt.Errorf("mtu out of range [1000,1500]: %d", params.MTU)
	}
	return nil
}

// MergeParamsToSettings serialises AWGParams and CPS packet data into
// the inbound's Settings JSON string, preserving existing fields.
func MergeParamsToSettings(inbound *model.Inbound, params *AWGParams, i1, i2, i3, i4, i5 string) error {
	var settings map[string]interface{}
	if inbound.Settings != "" {
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			return fmt.Errorf("unmarshal settings: %w", err)
		}
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	settings["mtu"] = params.MTU
	settings["jc"] = params.Jc
	settings["jmin"] = params.Jmin
	settings["jmax"] = params.Jmax
	settings["s1"] = params.S1
	settings["s2"] = params.S2
	settings["s3"] = params.S3
	settings["s4"] = params.S4
	settings["h1"] = params.H1
	settings["h2"] = params.H2
	settings["h3"] = params.H3
	settings["h4"] = params.H4
	settings["obfLevel"] = params.ObfLevel
	settings["mimicryProfile"] = params.MimicryProfile
	settings["region"] = params.Region
	settings["privateKey"] = params.PrivateKey
	settings["publicKey"] = params.PublicKey
	settings["presharedKey"] = params.PresharedKey

	if i1 != "" {
		settings["i1"] = i1
		settings["i2"] = i2
		settings["i3"] = i3
		settings["i4"] = i4
		settings["i5"] = i5
	}

	b, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	inbound.Settings = string(b)
	return nil
}

// validateHRange checks an H-parameter is a valid "lo-hi" range within quadrant bounds.
func validateHRange(name, val string, minLo, maxHi int64) error {
	if !strings.Contains(val, "-") {
		return fmt.Errorf("%s must be a range (missing '-'): %s", name, val)
	}
	parts := strings.SplitN(val, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%s invalid range format: %s", name, val)
	}
	lo, err1 := strconv.ParseInt(parts[0], 10, 64)
	hi, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s range values not numeric: %s", name, val)
	}
	if lo < minLo || lo > maxHi {
		return fmt.Errorf("%s lo=%d out of quadrant bounds [%d, %d]", name, lo, minLo, maxHi)
	}
	if hi < minLo || hi > maxHi {
		return fmt.Errorf("%s hi=%d out of quadrant bounds [%d, %d]", name, hi, minLo, maxHi)
	}
	if lo >= hi {
		return fmt.Errorf("%s lo (%d) must be < hi (%d)", name, lo, hi)
	}
	if hi-lo < 1000 {
		return fmt.Errorf("%s range too narrow: %d (min 1000)", name, hi-lo)
	}
	return nil
}
