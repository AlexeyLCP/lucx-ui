// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// Traffic-pattern tuning of a mieru inbound, mirroring enfein/mieru's
// appctlpb.TrafficPattern message. The block is optional end-to-end: an
// all-zero pattern is never rendered into the mita JSON nor into the
// mierus:// share link, so inbounds created before the feature keep their
// exact wire shape. JSON key names match the official mita config.

type MieruTCPFragment struct {
	Enable     bool  `json:"enable,omitempty"`
	MaxSleepMs int32 `json:"maxSleepMs,omitempty"`
}

type MieruNoncePattern struct {
	Type                string   `json:"type,omitempty"`
	ApplyToAllUDPPacket bool     `json:"applyToAllUDPPacket,omitempty"`
	MinLen              int32    `json:"minLen,omitempty"`
	MaxLen              int32    `json:"maxLen,omitempty"`
	CustomHexStrings    []string `json:"customHexStrings,omitempty"`
}

// MieruPaddingPattern carries pointers because 0 is a meaningful value
// ("disable this padding slot"), distinct from "not set" (mita internal
// default).
type MieruPaddingPattern struct {
	MaxMiddlePaddingLen *int32 `json:"maxMiddlePaddingLen,omitempty"`
	MaxEndPaddingLen    *int32 `json:"maxEndPaddingLen,omitempty"`
}

type MieruLowEntropyPattern struct {
	Mode         string `json:"mode,omitempty"`
	MaskRotation string `json:"maskRotation,omitempty"`
}

type MieruTrafficPattern struct {
	Seed        int32                   `json:"seed,omitempty"`
	UnlockAll   bool                    `json:"unlockAll,omitempty"`
	TCPFragment *MieruTCPFragment       `json:"tcpFragment,omitempty"`
	Nonce       *MieruNoncePattern      `json:"nonce,omitempty"`
	Padding     *MieruPaddingPattern    `json:"padding,omitempty"`
	LowEntropy  *MieruLowEntropyPattern `json:"lowEntropy,omitempty"`
}

func (f *MieruTCPFragment) isZero() bool {
	return f == nil || (!f.Enable && f.MaxSleepMs == 0)
}

func (n *MieruNoncePattern) isZero() bool {
	return n == nil ||
		(strings.TrimSpace(n.Type) == "" &&
			!n.ApplyToAllUDPPacket &&
			n.MinLen == 0 &&
			n.MaxLen == 0 &&
			len(n.CustomHexStrings) == 0)
}

func (p *MieruPaddingPattern) isZero() bool {
	return p == nil || (p.MaxMiddlePaddingLen == nil && p.MaxEndPaddingLen == nil)
}

func (l *MieruLowEntropyPattern) isZero() bool {
	if l == nil {
		return true
	}
	mode := strings.TrimSpace(l.Mode)
	rot := strings.TrimSpace(l.MaskRotation)
	return (mode == "" || mode == "LOW_ENTROPY_MODE_OFF") &&
		(rot == "" || rot == "LOW_ENTROPY_MASK_NO_ROTATION")
}

func (p *MieruTrafficPattern) IsZero() bool {
	return p == nil ||
		(p.Seed == 0 &&
			!p.UnlockAll &&
			p.TCPFragment.isZero() &&
			p.Nonce.isZero() &&
			p.Padding.isZero() &&
			p.LowEntropy.isZero())
}

// Normalized returns a copy with zero sub-blocks dropped (so the mita JSON
// never carries empty objects), or nil when the whole pattern is unset.
func (p *MieruTrafficPattern) Normalized() *MieruTrafficPattern {
	if p == nil {
		return nil
	}
	q := *p
	if q.TCPFragment.isZero() {
		q.TCPFragment = nil
	}
	if q.Nonce.isZero() {
		q.Nonce = nil
	}
	if q.Padding.isZero() {
		q.Padding = nil
	}
	if q.LowEntropy.isZero() {
		q.LowEntropy = nil
	}
	if q.IsZero() {
		return nil
	}
	return &q
}

var mieruNonceTypeNumbers = map[string]int32{
	"NONCE_TYPE_RANDOM":           0,
	"NONCE_TYPE_PRINTABLE":        1,
	"NONCE_TYPE_PRINTABLE_SUBSET": 2,
	"NONCE_TYPE_FIXED":            3,
}

var mieruLowEntropyModeNumbers = map[string]int32{
	"LOW_ENTROPY_MODE_OFF": 0,
	"LOW_ENTROPY_MODE_32":  1,
	"LOW_ENTROPY_MODE_40":  2,
	"LOW_ENTROPY_MODE_48":  3,
	"LOW_ENTROPY_MODE_56":  4,
}

func mieruMaskRotationNumber(s string) (int32, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "LOW_ENTROPY_MASK_NO_ROTATION" {
		return 0, true
	}
	for i := int32(1); i <= 15; i++ {
		if s == fmt.Sprintf("LOW_ENTROPY_MASK_ROTATE_RIGHT_%d", i) {
			return i, true
		}
		if s == fmt.Sprintf("LOW_ENTROPY_MASK_ROTATE_LEFT_%d", i) {
			return 16 * i, true
		}
	}
	return 0, false
}

func (p *MieruTrafficPattern) validate() error {
	if p == nil {
		return nil
	}
	if p.Seed < 0 {
		return fmt.Errorf("mieru: trafficPattern.seed must be >= 0")
	}
	if f := p.TCPFragment; f != nil {
		if f.MaxSleepMs < 0 || f.MaxSleepMs > 100 {
			return fmt.Errorf("mieru: tcpFragment.maxSleepMs must be within 0..100")
		}
	}
	if n := p.Nonce; n != nil {
		t := strings.TrimSpace(n.Type)
		if _, ok := mieruNonceTypeNumbers[t]; !ok && t != "" {
			return fmt.Errorf("mieru: nonce.type must be one of NONCE_TYPE_RANDOM | NONCE_TYPE_PRINTABLE | NONCE_TYPE_PRINTABLE_SUBSET | NONCE_TYPE_FIXED")
		}
		if n.MinLen < 0 || n.MinLen > 12 {
			return fmt.Errorf("mieru: nonce.minLen must be within 0..12")
		}
		if n.MaxLen < 0 || n.MaxLen > 12 {
			return fmt.Errorf("mieru: nonce.maxLen must be within 0..12")
		}
		if n.MaxLen > 0 && n.MinLen > n.MaxLen {
			return fmt.Errorf("mieru: nonce.minLen must not exceed nonce.maxLen")
		}
		for _, h := range n.CustomHexStrings {
			hh := strings.TrimSpace(h)
			if hh == "" || len(hh) > 24 || len(hh)%2 != 0 {
				return fmt.Errorf("mieru: nonce.customHexStrings entries must be 1..12 bytes of hex (even length)")
			}
			if _, err := hex.DecodeString(hh); err != nil {
				return fmt.Errorf("mieru: nonce.customHexStrings entry %q is not valid hex", hh)
			}
		}
	}
	if pad := p.Padding; pad != nil {
		if pad.MaxMiddlePaddingLen != nil && (*pad.MaxMiddlePaddingLen < 0 || *pad.MaxMiddlePaddingLen > 255) {
			return fmt.Errorf("mieru: padding.maxMiddlePaddingLen must be within 0..255")
		}
		if pad.MaxEndPaddingLen != nil && (*pad.MaxEndPaddingLen < 0 || *pad.MaxEndPaddingLen > 255) {
			return fmt.Errorf("mieru: padding.maxEndPaddingLen must be within 0..255")
		}
	}
	if le := p.LowEntropy; le != nil {
		m := strings.TrimSpace(le.Mode)
		if _, ok := mieruLowEntropyModeNumbers[m]; !ok && m != "" {
			return fmt.Errorf("mieru: lowEntropy.mode must be one of LOW_ENTROPY_MODE_OFF | LOW_ENTROPY_MODE_32 | LOW_ENTROPY_MODE_40 | LOW_ENTROPY_MODE_48 | LOW_ENTROPY_MODE_56")
		}
		if _, ok := mieruMaskRotationNumber(le.MaskRotation); !ok {
			return fmt.Errorf("mieru: lowEntropy.maskRotation is not a valid LOW_ENTROPY_MASK_* value")
		}
	}
	return nil
}

func appendVarintField(b []byte, num protowire.Number, v int32) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(v))
}

func appendBoolField(b []byte, num protowire.Number) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, 1)
}

func appendMessageField(b, sub []byte, num protowire.Number) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, sub)
}

// EncodeProto serializes the pattern to the enfein/mieru appctlpb.TrafficPattern
// wire format (fields in number order, proto3 optional semantics). The base64
// of this blob is the mierus:// "traffic-pattern" query parameter; mita reads
// the same shape from its server config JSON.
func (p *MieruTrafficPattern) EncodeProto() []byte {
	q := p.Normalized()
	if q == nil {
		return nil
	}
	var b []byte
	if q.Seed != 0 {
		b = appendVarintField(b, 1, q.Seed)
	}
	if q.UnlockAll {
		b = appendBoolField(b, 2)
	}
	if f := q.TCPFragment; f != nil {
		var sub []byte
		if f.Enable {
			sub = appendBoolField(sub, 1)
		}
		if f.MaxSleepMs != 0 {
			sub = appendVarintField(sub, 2, f.MaxSleepMs)
		}
		b = appendMessageField(b, sub, 3)
	}
	if n := q.Nonce; n != nil {
		var sub []byte
		if num := mieruNonceTypeNumbers[strings.TrimSpace(n.Type)]; num != 0 {
			sub = appendVarintField(sub, 1, num)
		}
		if n.ApplyToAllUDPPacket {
			sub = appendBoolField(sub, 2)
		}
		if n.MinLen != 0 {
			sub = appendVarintField(sub, 3, n.MinLen)
		}
		if n.MaxLen != 0 {
			sub = appendVarintField(sub, 4, n.MaxLen)
		}
		for _, h := range n.CustomHexStrings {
			hh := strings.TrimSpace(h)
			if hh == "" {
				continue
			}
			sub = protowire.AppendTag(sub, 5, protowire.BytesType)
			sub = protowire.AppendBytes(sub, []byte(hh))
		}
		b = appendMessageField(b, sub, 4)
	}
	if pad := q.Padding; pad != nil {
		var sub []byte
		if pad.MaxMiddlePaddingLen != nil {
			sub = appendVarintField(sub, 1, *pad.MaxMiddlePaddingLen)
		}
		if pad.MaxEndPaddingLen != nil {
			sub = appendVarintField(sub, 2, *pad.MaxEndPaddingLen)
		}
		b = appendMessageField(b, sub, 5)
	}
	if le := q.LowEntropy; le != nil {
		var sub []byte
		if num := mieruLowEntropyModeNumbers[strings.TrimSpace(le.Mode)]; num != 0 {
			sub = appendVarintField(sub, 1, num)
		}
		if num, _ := mieruMaskRotationNumber(le.MaskRotation); num != 0 {
			sub = appendVarintField(sub, 2, num)
		}
		b = appendMessageField(b, sub, 6)
	}
	return b
}

// LinkParam returns the base64 traffic-pattern value for the mierus:// share
// link, or "" when the pattern is unset.
func (p *MieruTrafficPattern) LinkParam() string {
	b := p.EncodeProto()
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}
