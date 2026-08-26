// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// The CPS descriptor language, and the only place that knows it. Both engines
// compile a descriptor once and refill every non-literal span before each send,
// so a literal anchors bytes while <r>/<rc>/<rd>/<t> vary per packet.
type segKind int

const (
	segLit segKind = iota
	segRand
	segRandChars
	segRandDigits
	segTimestamp
)

type segment struct {
	kind segKind
	lit  []byte
	n    int
}

// Descriptor is one I-field. Build it with the tag methods and render with
// String; nothing outside this file may assemble a descriptor by hand.
type Descriptor struct{ segs []segment }

func (d Descriptor) with(s segment) Descriptor {
	segs := make([]segment, len(d.segs), len(d.segs)+1)
	copy(segs, d.segs)
	return Descriptor{segs: append(segs, s)}
}

// Lit appends literal bytes: the same on every send, so it is what a protocol
// parser keys on and what cross-packet coherence (SNI, DCID, Call-ID) needs.
func (d Descriptor) Lit(b []byte) Descriptor {
	return d.with(segment{kind: segLit, lit: append([]byte(nil), b...)})
}

// Rand appends n bytes redrawn on every send. Use it only where an observer
// cannot verify the bytes — ciphertext, nonces, session ids, key shares.
func (d Descriptor) Rand(n int) Descriptor { return d.with(segment{kind: segRand, n: n}) }

// RandChars appends n random ASCII letters, RandDigits n random ASCII digits —
// the two that keep a plaintext protocol (SIP identifiers) syntactically valid.
func (d Descriptor) RandChars(n int) Descriptor  { return d.with(segment{kind: segRandChars, n: n}) }
func (d Descriptor) RandDigits(n int) Descriptor { return d.with(segment{kind: segRandDigits, n: n}) }

// Timestamp appends the 4-byte big-endian seconds counter both engines emit.
func (d Descriptor) Timestamp() Descriptor { return d.with(segment{kind: segTimestamp}) }

// String renders the kernel's strict spelling, which amneziawg-go also accepts:
// exactly one space, lowercase tag letters and 0x prefix. go tolerates more.
func (d Descriptor) String() string {
	var b strings.Builder
	for _, s := range d.segs {
		switch s.kind {
		case segLit:
			b.WriteString("<b 0x")
			b.WriteString(hex.EncodeToString(s.lit))
			b.WriteByte('>')
		case segRand:
			b.WriteString("<r " + strconv.Itoa(s.n) + ">")
		case segRandChars:
			b.WriteString("<rc " + strconv.Itoa(s.n) + ">")
		case segRandDigits:
			b.WriteString("<rd " + strconv.Itoa(s.n) + ">")
		case segTimestamp:
			b.WriteString("<t>")
		}
	}
	return b.String()
}

// Len is the descriptor's cost against the netlink budget; WireLen is the
// packet it expands to, which nothing below this layer bounds against the MTU.
func (d Descriptor) Len() int { return len(d.String()) }

func (d Descriptor) WireLen() int {
	n := 0
	for _, s := range d.segs {
		switch s.kind {
		case segLit:
			n += len(s.lit)
		case segTimestamp:
			n += 4
		default:
			n += s.n
		}
	}
	return n
}

// Validate rejects what the engines accept but cannot survive: a non-positive
// count reaches an allocation with a negative length in both.
func (d Descriptor) Validate() error {
	for i, s := range d.segs {
		switch s.kind {
		case segLit:
			if len(s.lit) == 0 {
				return fmt.Errorf("awg cps: descriptor segment %d is an empty literal", i)
			}
		case segTimestamp:
		default:
			if s.n <= 0 {
				return fmt.Errorf("awg cps: descriptor segment %d has count %d, want > 0", i, s.n)
			}
		}
	}
	if strings.ContainsRune(d.String(), '#') {
		return fmt.Errorf("awg cps: descriptor contains '#', which truncates the .conf line")
	}
	return nil
}
