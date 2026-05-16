// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
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
	pubKey := genKey() // service layer derives real Curve25519 pubkey via awg pubkey
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

func genKey() string {
	key := make([]byte, 32)
	rand.Read(key)
	return base64.StdEncoding.EncodeToString(key)
}

func genPSK() string {
	psk := make([]byte, 32)
	rand.Read(psk)
	return base64.StdEncoding.EncodeToString(psk)
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
