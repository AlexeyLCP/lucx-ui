// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Mirrors internal/awg/idescriptor.go. Neither AmneziaWG engine is a superset
// of the other — the kernel module has <c> and lacks <d>/<ds>/<dz>, amneziawg-go
// the reverse — so anything handed to a client whose engine we cannot know has
// to sit in the intersection.
//
// Written in the kernel's strict spelling, which the lenient go parser also
// takes: exactly one space before an argument, a lower-case 0x, hex in pairs,
// an unsigned count. Anchored, because both engines silently ignore text
// outside <…> — dead weight that spends the netlink budget and reads as working
// when it is not.
const PORTABLE_I_FIELD = /^(?:<(?:b 0x(?:[0-9a-fA-F]{2})+|t|r [0-9]+|rc [0-9]+|rd [0-9]+)>)+$/;

// awgPortableIField reports whether an I1-I5 value can be handed to a client
// without knowing which engine will read it. A blank value is not portable:
// writing 'I1 = ' into a .conf makes amneziawg-tools refuse the whole file.
//
// Padding around the value is not held against it — no engine ever sees it,
// every .conf reader strips it off the line — so callers emit the trimmed form.
// Padding inside a tag is a different matter and stays refused.
export function awgPortableIField(v?: string): boolean {
  return PORTABLE_I_FIELD.test((v ?? '').trim());
}
