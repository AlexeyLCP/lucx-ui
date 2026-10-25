// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import type { WgoPreset } from '@/schemas/protocols/inbound/wgo';

// Preset UDP port window from the Phobos panel docs (51822-51861 for WG
// presets); we keep scanning past it when the window is exhausted.
const PORT_MIN = 51822;

export function randomKey(len = 16): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  const bytes = new Uint8Array(len);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => alphabet[b % alphabet.length]).join('');
}

// nextPreset mints a self-sufficient preset (Phobos-panel "Add preset" UX:
// next free port, fresh key, unique client loopback port). Ports taken by
// other inbounds are still rejected by backend validation on save.
export function nextPreset(existing: WgoPreset[], wgPort: number | undefined): WgoPreset {
  const usedPorts = new Set(existing.map((p) => p.extPort));
  const usedLocal = new Set(existing.map((p) => p.clientWgLocalPort));
  const usedIds = new Set(existing.map((p) => p.id));
  let extPort = PORT_MIN;
  while (extPort < 65536 && (usedPorts.has(extPort) || extPort === wgPort)) extPort++;
  let clientWgLocalPort = 13255;
  while (usedLocal.has(clientWgLocalPort)) clientWgLocalPort++;
  let id = randomKey(4).toLowerCase();
  while (usedIds.has(id)) id = randomKey(4).toLowerCase();
  return {
    id,
    name: `phobos-${extPort}`,
    extPort,
    key: randomKey(),
    masking: 'STUN',
    mediaSsrc: '',
    mediaPt: 0,
    mediaClock: 30,
    maxDummy: 4,
    idleTimeout: 300,
    clientWgLocalPort,
    isDefault: existing.length === 0,
  };
}
