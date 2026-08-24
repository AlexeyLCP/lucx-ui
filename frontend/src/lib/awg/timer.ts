// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// normalizeAwgTimer canonicalises an AWG3 device-timer value into the string
// the backend stores and the kernel consumes. The amneziawg kernel (device.h
// u16_range_t) and tools (u16_range_from_string) accept BOTH a single integer
// ("150") and an inclusive range ("100-500"), randomizing within a range at
// rekey exactly like H1-H4 — so a range is passed through VERBATIM, never
// collapsed to one number. Accepts a number (legacy/panel default) or a
// string; clamps endpoints to 0..65535, orders a reversed range, folds an
// "N-N" range to "N", and falls back to "0" (kernel default) for anything
// unparseable.
export function normalizeAwgTimer(v: unknown): string {
  if (typeof v === 'number') return String(clampTimerInt(v));
  if (typeof v === 'string') {
    const s = v.trim();
    if (s === '') return '0';
    const range = s.match(/^(\d+)\s*-\s*(\d+)$/);
    if (range) {
      let lo = clampTimerInt(Number(range[1]));
      let hi = clampTimerInt(Number(range[2]));
      if (lo > hi) [lo, hi] = [hi, lo];
      return lo === hi ? String(lo) : `${lo}-${hi}`;
    }
    const n = Number(s);
    if (!Number.isNaN(n)) return String(clampTimerInt(n));
    return '0';
  }
  return '0';
}

function clampTimerInt(n: number): number {
  if (!Number.isFinite(n) || n < 0) return 0;
  if (n > 65535) return 65535;
  return Math.trunc(n);
}

export function collapseKeepaliveForVersion(
  keepAlive: unknown,
  isAwg3Plus: boolean,
): string | null {
  const s = normalizeAwgTimer(keepAlive);
  if (s === '0') return null;
  if (!isAwg3Plus) {
    const i = s.indexOf('-');
    return i > 0 ? s.slice(0, i) : s;
  }
  return s;
}
