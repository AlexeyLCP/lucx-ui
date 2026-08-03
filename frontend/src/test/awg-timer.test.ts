import { describe, expect, it } from 'vitest';

import { normalizeAwgTimer } from '@/lib/awg/timer';

describe('normalizeAwgTimer', () => {
  it('passes a single value through as a string (clamped to 0..65535)', () => {
    expect(normalizeAwgTimer(120)).toBe('120');
    expect(normalizeAwgTimer('120')).toBe('120');
    expect(normalizeAwgTimer('  42 ')).toBe('42');
    expect(normalizeAwgTimer(0)).toBe('0');
    expect(normalizeAwgTimer(70000)).toBe('65535');
    expect(normalizeAwgTimer(-5)).toBe('0');
    expect(normalizeAwgTimer(1.9)).toBe('1');
  });

  it('passes an inclusive range through VERBATIM (never collapses)', () => {
    expect(normalizeAwgTimer('100-500')).toBe('100-500');
    expect(normalizeAwgTimer('3-7')).toBe('3-7');
    expect(normalizeAwgTimer('10-64')).toBe('10-64');
    expect(normalizeAwgTimer(' 8 - 12 ')).toBe('8-12');
  });

  it('orders a reversed range and folds N-N to N', () => {
    expect(normalizeAwgTimer('500-100')).toBe('100-500');
    expect(normalizeAwgTimer('100-100')).toBe('100');
  });

  it('clamps range endpoints to 0..65535', () => {
    expect(normalizeAwgTimer('0-70000')).toBe('0-65535');
    expect(normalizeAwgTimer('90000-99999')).toBe('65535');
  });

  it('returns "0" for empty / unparseable / unknown input', () => {
    expect(normalizeAwgTimer('')).toBe('0');
    expect(normalizeAwgTimer('abc')).toBe('0');
    expect(normalizeAwgTimer('100-')).toBe('0');
    expect(normalizeAwgTimer(undefined)).toBe('0');
    expect(normalizeAwgTimer(null)).toBe('0');
    expect(normalizeAwgTimer({})).toBe('0');
  });
});
