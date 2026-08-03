import { describe, expect, it } from 'vitest';

import { resolveAwgTimerRange } from '@/lib/awg/timer';

describe('resolveAwgTimerRange', () => {
  it('passes a single number through (clamped to 0..65535)', () => {
    expect(resolveAwgTimerRange(120)).toBe(120);
    expect(resolveAwgTimerRange(0)).toBe(0);
    expect(resolveAwgTimerRange(70000)).toBe(65535);
    expect(resolveAwgTimerRange(-5)).toBe(0);
    expect(resolveAwgTimerRange(1.9)).toBe(1);
  });

  it('parses a numeric string', () => {
    expect(resolveAwgTimerRange('120')).toBe(120);
    expect(resolveAwgTimerRange('  42 ')).toBe(42);
    expect(resolveAwgTimerRange('70000')).toBe(65535);
  });

  it('rolls a value inside an inclusive range', () => {
    for (let i = 0; i < 50; i += 1) {
      const v = resolveAwgTimerRange('100-500');
      expect(v).toBeGreaterThanOrEqual(100);
      expect(v).toBeLessThanOrEqual(500);
    }
  });

  it('normalises a reversed range', () => {
    for (let i = 0; i < 30; i += 1) {
      const v = resolveAwgTimerRange('500-100');
      expect(v).toBeGreaterThanOrEqual(100);
      expect(v).toBeLessThanOrEqual(500);
    }
  });

  it('clamps range endpoints to 0..65535', () => {
    const v = resolveAwgTimerRange('0-70000');
    expect(v).toBeGreaterThanOrEqual(0);
    expect(v).toBeLessThanOrEqual(65535);
  });

  it('returns 0 for empty / unparseable / unknown input', () => {
    expect(resolveAwgTimerRange('')).toBe(0);
    expect(resolveAwgTimerRange('abc')).toBe(0);
    expect(resolveAwgTimerRange('100-')).toBe(0);
    expect(resolveAwgTimerRange(undefined)).toBe(0);
    expect(resolveAwgTimerRange(null)).toBe(0);
    expect(resolveAwgTimerRange({})).toBe(0);
  });

  it('returns 0 for a 0-0 range', () => {
    expect(resolveAwgTimerRange('0-0')).toBe(0);
  });
});
