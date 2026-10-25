// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { describe, expect, it } from 'vitest';

import { nextPreset } from '@/lib/wgo/preset';
import type { WgoPreset } from '@/schemas/protocols/inbound/wgo';

function taken(over: Partial<WgoPreset>): WgoPreset {
  return {
    id: 'p1',
    name: 'phobos-51822',
    extPort: 51822,
    key: 'k1',
    masking: 'STUN',
    mediaSsrc: '',
    mediaPt: 0,
    mediaClock: 30,
    maxDummy: 4,
    idleTimeout: 300,
    clientWgLocalPort: 13255,
    isDefault: true,
    ...over,
  };
}

describe('nextPreset auto-fill', () => {
  it('first preset takes the base ports and is default', () => {
    const p = nextPreset([], 51820);
    expect(p.extPort).toBe(51822);
    expect(p.clientWgLocalPort).toBe(13255);
    expect(p.isDefault).toBe(true);
    expect(p.masking).toBe('STUN');
    expect(p.name).toBe('phobos-51822');
    expect(p.key).not.toBe('');
    expect(p.id).toMatch(/^[a-z0-9]{4}$/);
  });

  it('skips ports taken by other presets and the WG port', () => {
    const p = nextPreset(
      [taken({ extPort: 51822, id: 'a' }), taken({ extPort: 51823, id: 'b' })],
      51824,
    );
    expect(p.extPort).toBe(51825);
    expect(p.isDefault).toBe(false);
  });

  it('never reuses the WG listen port', () => {
    const p = nextPreset([taken({ extPort: 51822, id: 'a' })], 51823);
    expect(p.extPort).toBe(51824);
  });

  it('gives each preset its own client loopback port', () => {
    const p = nextPreset([taken({ id: 'a' }), taken({ id: 'b', clientWgLocalPort: 13256 })], 51820);
    expect(p.clientWgLocalPort).toBe(13257);
  });
});
