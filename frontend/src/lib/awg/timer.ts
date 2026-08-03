// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// resolveAwgTimerRange accepts either a single integer ("120" / 120) or an
// inclusive range ("100-500") for an AWG3 device-timer field and returns the
// single integer the kernel UAPI actually consumes — these timers are
// interface-level [Interface] values, not per-peer ranges like H1-H4, so the
// panel resolves a range to one value. A range is rolled once per submit (the
// Zod preprocess runs on form submit), so the operator gets a stable value
// stored in the DB; re-editing shows that value, and re-entering a range
// re-rolls. Empty / unparseable / out-of-range input falls back to 0, which
// the .conf renderer omits (kernel then uses its built-in WG constant).
export function resolveAwgTimerRange(v: unknown): number {
  if (typeof v === 'number') return clampTimerInt(v);
  if (typeof v === 'string') {
    const s = v.trim();
    if (s === '') return 0;
    const range = s.match(/^(\d+)\s*-\s*(\d+)$/);
    if (range) {
      let lo = clampTimerInt(Number(range[1]));
      let hi = clampTimerInt(Number(range[2]));
      if (lo > hi) [lo, hi] = [hi, lo];
      if (hi === 0) return 0;
      return lo + Math.floor(Math.random() * (hi - lo + 1));
    }
    const n = Number(s);
    if (!Number.isNaN(n)) return clampTimerInt(n);
    return 0;
  }
  return 0;
}

function clampTimerInt(n: number): number {
  if (!Number.isFinite(n) || n < 0) return 0;
  if (n > 65535) return 65535;
  return Math.trunc(n);
}
