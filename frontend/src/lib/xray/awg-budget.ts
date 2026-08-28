// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Mirrors internal/awg/cps_budget.go. The 4096 bytes bound one netlink message,
// not the device, so the I1-I5 limit is a byte budget, not a character count.
const NL_BUF_BYTES = 4096;
const NL_DEVICE_BYTES = 296; // device block measured at the 6-char ifname baseline
const NL_IFNAME_BASE = 12; // nlaBytes('awgo-1'), already counted in NL_DEVICE_BYTES
const NL_HPK_BYTES = 36; // WGDEVICE_A_HEADER_PROTECTION_KEY attribute
const NL_PEERS_NEST = 4; // WGDEVICE_A_PEERS nest header
const NL_PEER_BYTES = 256; // one whole peer: 188 fixed + '0.0.0.0/0, ::/0'
const NL_SAFETY_MARGIN = 40;

// WORST_IFNAME_BYTES is nlaBytes of the longest name a node can hand an
// interface: 'awg' plus up to nine digits, from the node's own id sequence.
const WORST_IFNAME_BYTES = 20;

const utf8 = new TextEncoder();

// nlaBytes is NLA_ALIGN(4 + len + 1), the cost of one NUL-terminated netlink
// string attribute. It quantises, so a sum of lengths is not monotonic in it.
function nlaBytes(v: string): number {
  return (utf8.encode(v).length + 8) & ~3;
}

// awgIBytes returns the netlink cost of the non-empty I1-I5 fields. Compare it
// against awgWorstCaseIBytesBudget — never the character sum.
export function awgIBytes(i1?: string, i2?: string, i3?: string, i4?: string, i5?: string): number {
  let n = 0;
  for (const raw of [i1, i2, i3, i4, i5]) {
    const v = (raw ?? '').trim();
    if (v !== '') n += nlaBytes(v);
  }
  return n;
}

// awgWorstCaseIBytesBudget budgets the longest ifname a node could ever assign,
// for a caller with no real ifname to check: 3492, or 3456 with an HPK.
export function awgWorstCaseIBytesBudget(hasHeaderProtectionKey: boolean): number {
  const b =
    NL_BUF_BYTES -
    NL_DEVICE_BYTES -
    (WORST_IFNAME_BYTES - NL_IFNAME_BASE) -
    NL_PEERS_NEST -
    NL_PEER_BYTES -
    NL_SAFETY_MARGIN;
  return hasHeaderProtectionKey ? b - NL_HPK_BYTES : b;
}
