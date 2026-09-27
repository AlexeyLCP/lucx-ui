// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { describe, expect, it } from 'vitest';

import { AwgInboundSettingsSchema } from '@/schemas/protocols/inbound/awg';

describe('AWG TPROXY compatibility', () => {
  it('preserves direct and TUN settings without opting into TPROXY', () => {
    for (const enabled of [false, true]) {
      const settings = AwgInboundSettingsSchema.parse({ routeThroughXray: enabled });
      expect(settings.routeThroughXray).toBe(enabled);
      expect(settings.xrayRoutingMode).toBe('tun');
    }
  });

  it('retains mode and port through form serialization', () => {
    const settings = AwgInboundSettingsSchema.parse({
      routeThroughXray: true,
      xrayRoutingMode: 'tproxy',
      tproxyPort: 51454,
    });
    expect(settings.xrayRoutingMode).toBe('tproxy');
    expect(settings.tproxyPort).toBe(51454);
  });

  it('rejects invalid mode and port', () => {
    expect(AwgInboundSettingsSchema.safeParse({ xrayRoutingMode: 'unknown' }).success).toBe(false);
    expect(AwgInboundSettingsSchema.safeParse({ tproxyPort: 65536 }).success).toBe(false);
  });
});
