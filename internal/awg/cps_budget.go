// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"errors"
	"fmt"
	"strings"
)

// The kernel carries a whole device in one 4096-byte netlink message, so the
// I1-I5 limit is a byte budget, not a character count.
const (
	nlBufBytes     = 4096
	nlDeviceBytes  = 296 // device block measured at the 6-char ifname baseline
	nlIfnameBase   = 12  // nlaBytes("awgo-1"), already counted in nlDeviceBytes
	nlHpkBytes     = 36  // WGDEVICE_A_HEADER_PROTECTION_KEY attribute
	nlPeersNest    = 4   // WGDEVICE_A_PEERS nest header
	nlPeerBytes    = 168 // one peer's fixed prefix; a client .conf has exactly one
	nlSafetyMargin = 128
)

// BaselineIfname is the 6-character shape the 3500-byte budget is quoted for.
// An exported client config names its own interface, so it is checked here.
const BaselineIfname = "awgo-1"

// worstIfnameBytes is nlaBytes of the longest name a node can hand this
// interface: "awg" plus up to nine digits, from the node's own id sequence.
const worstIfnameBytes = 20

// nlaBytes is NLA_ALIGN(4 + len + 1), the cost of one NUL-terminated netlink
// string attribute. It quantises, so a sum of lengths is not monotonic in it.
func nlaBytes(v string) int { return (len(v) + 8) &^ 3 }

// IBytes returns the netlink cost of the non-empty I1-I5 fields. Compare it
// against IBytesBudget — never the character sum, which is not monotonic here.
func IBytes(i1, i2, i3, i4, i5 string) int {
	n := 0
	for _, v := range []string{i1, i2, i3, i4, i5} {
		if v = strings.TrimSpace(v); v != "" {
			n += nlaBytes(v)
		}
	}
	return n
}

// ibytesBudget is the arithmetic IBytesBudget and WorstCaseIBytesBudget share,
// parameterised on the ifname's already-computed netlink byte cost.
func ibytesBudget(ifnameBytes int, hasHeaderProtectionKey bool) int {
	b := nlBufBytes - nlDeviceBytes - (ifnameBytes - nlIfnameBase) - nlPeersNest - nlPeerBytes - nlSafetyMargin
	if hasHeaderProtectionKey {
		b -= nlHpkBytes
	}
	return b
}

// IBytesBudget returns the largest IBytes an interface of this shape can carry
// and still answer `awg show`: 3500 for a 6-char ifname with no HPK.
func IBytesBudget(ifname string, hasHeaderProtectionKey bool) int {
	return ibytesBudget(nlaBytes(ifname), hasHeaderProtectionKey)
}

// WorstCaseIBytesBudget is IBytesBudget for the longest ifname a node could
// ever assign, for a caller that has no real ifname to check against.
func WorstCaseIBytesBudget(hasHeaderProtectionKey bool) int {
	return ibytesBudget(worstIfnameBytes, hasHeaderProtectionKey)
}

// MaxIPacketBytes bounds one I-packet. Nothing else does: the kernel sets
// skb->ignore_df, so a larger one is IP-fragmented and sent silently. 1400 sits
// under the worst common UDP payload ceiling (1444 on IPv6-over-PPPoE) and above
// the 1200 bytes RFC 9000 requires of a QUIC Initial.
const MaxIPacketBytes = 1400

// ErrIFieldsTooLarge means the I-set overflows that budget: the config still
// applies and carries traffic, but `awg show` hangs or fails with EMSGSIZE.
var ErrIFieldsTooLarge = errors.New("awg: I1-I5 exceed the netlink read budget")

// ValidateIFields budgets worst-case, not by ifname: a node's real interface
// name comes from its own id sequence, invisible to the master saving this.
func ValidateIFields(ifname, headerProtectionKey, i1, i2, i3, i4, i5 string) error {
	got := IBytes(i1, i2, i3, i4, i5)
	budget := WorstCaseIBytesBudget(strings.TrimSpace(headerProtectionKey) != "")
	if got > budget {
		return fmt.Errorf("%w: %d > %d worst-case bytes for %s — shorten or drop I-fields", ErrIFieldsTooLarge, got, budget, ifname)
	}
	return nil
}
