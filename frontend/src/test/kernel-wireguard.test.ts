// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
import { describe, expect, it } from 'vitest';
import { WireguardInboundSettingsSchema } from '@/schemas/protocols/inbound/wireguard';
import { buildWireguardClientConfig } from '@/pages/clients/wireguardConfig';

describe('kernel WireGuard backend', () => {
  it('keeps existing settings on the Xray backend', () => {
    const settings = WireguardInboundSettingsSchema.parse({ secretKey: 'existing' });
    expect(settings.kernel ?? false).toBe(false);
  });
  it('retains kernel settings across form parsing', () => {
    const settings = WireguardInboundSettingsSchema.parse({
      secretKey: 'existing',
      kernel: true,
      address: '10.90.0.1/24',
      tproxyPort: 32101,
    });
    expect(settings.kernel).toBe(true);
    expect(settings.address).toBe('10.90.0.1/24');
    expect(settings.tproxyPort).toBe(32101);
  });
  it('requires IPv4 address and transparent port only for the kernel backend', () => {
    expect(
      WireguardInboundSettingsSchema.safeParse({ secretKey: 'existing', kernel: true }).success,
    ).toBe(false);
    expect(
      WireguardInboundSettingsSchema.safeParse({
        secretKey: 'existing',
        kernel: true,
        address: 'fd00::1/64',
        tproxyPort: 32101,
      }).success,
    ).toBe(false);
  });
  it('does not export an imported public-only peer with an empty private key', () => {
    expect(
      buildWireguardClientConfig(
        { email: 'imported', publicKey: 'public', allowedIPs: '10.90.0.2/32' },
        undefined,
        'vpn.example',
      ),
    ).toBe('');
  });
});

// LUCX-HOOK: wgobfs — obfuscation block parses only when present; enabling
// without presets is a validation error (vanilla overlay stays untouched).
describe('wireguard obfuscation schema', () => {
  it('keeps vanilla settings parsing without the obfuscation key', () => {
    const parsed = WireguardInboundSettingsSchema.safeParse({
      secretKey: 'k',
      mtu: 1420,
      clients: [],
      peers: [],
      noKernelTun: false,
      subnetIp: '',
    });
    expect(parsed.success).toBe(true);
    if (parsed.success) expect(parsed.data.obfuscation).toBeUndefined();
  });

  it('parses a preset round-trip', () => {
    const parsed = WireguardInboundSettingsSchema.safeParse({
      kernel: true,
      secretKey: 'k',
      address: '10.8.0.1/24',
      tproxyPort: 40000,
      obfuscation: {
        enabled: true,
        blockDirectPort: false,
        presets: [
          {
            id: 'p1',
            name: 'phobos-ru',
            extPort: 51822,
            key: 'k1',
            masking: 'STUN',
            maxDummy: 4,
            idleTimeout: 300,
            clientWgLocalPort: 13255,
            isDefault: true,
          },
        ],
      },
      clients: [],
      peers: [],
      noKernelTun: false,
      subnetIp: '',
    });
    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data.obfuscation?.presets[0].mediaClock).toBe(0);
      expect(parsed.data.obfuscation?.presets[0].isDefault).toBe(true);
    }
  });

  it('rejects enabled obfuscation without presets (kernel only)', () => {
    const parsed = WireguardInboundSettingsSchema.safeParse({
      kernel: true,
      secretKey: 'k',
      address: '10.8.0.1/24',
      tproxyPort: 40000,
      obfuscation: { enabled: true, blockDirectPort: false, presets: [] },
      clients: [],
      peers: [],
      noKernelTun: false,
      subnetIp: '',
    });
    expect(parsed.success).toBe(false);
  });
});
