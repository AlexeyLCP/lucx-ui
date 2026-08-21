import { describe, expect, it } from 'vitest';

import { buildWireguardClientConfig } from '@/pages/clients/wireguardConfig';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

const client: ClientRecord = {
  email: 'alice',
  privateKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
  allowedIPs: '10.0.0.2/32',
  preSharedKey: 'cHNrLXZhbHVlLWZvci13aXJlZ3VhcmQtdGVzdC1jYXNlIQ==',
  keepAlive: '25',
  inboundIds: [90],
};

const inbound: InboundOption = {
  id: 90,
  tag: 'in-51820-udp',
  remark: 'wg-mc',
  protocol: 'wireguard',
  port: 51820,
  wgPublicKey: 'DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=',
  wgMtu: 1420,
};

describe('buildWireguardClientConfig', () => {
  it('emits the canonical PresharedKey key, not PreSharedKey', () => {
    const cfg = buildWireguardClientConfig(client, inbound, 'example.com', '');
    expect(cfg).toContain(`PresharedKey = ${client.preSharedKey}`);
    expect(cfg).not.toContain('PreSharedKey =');
  });

  it('defaults DNS to 1.1.1.1, 1.0.0.1 when the inbound sets none', () => {
    const cfg = buildWireguardClientConfig(client, inbound, 'example.com', '');
    expect(cfg).toContain('DNS = 1.1.1.1, 1.0.0.1');
  });

  it('uses the inbound DNS override when present', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, wgDns: '9.9.9.9' }, 'example.com', '');
    expect(cfg).toContain('DNS = 9.9.9.9');
    expect(cfg).not.toContain('DNS = 1.1.1.1, 1.0.0.1');
  });

  it('builds the endpoint from host, port, MTU and server public key', () => {
    const cfg = buildWireguardClientConfig(client, inbound, 'example.com', '');
    expect(cfg).toContain('Endpoint = example.com:51820');
    expect(cfg).toContain('MTU = 1420');
    expect(cfg).toContain(`PublicKey = ${inbound.wgPublicKey}`);
    expect(cfg).toContain('PersistentKeepalive = 25');
  });

  it('omits the PresharedKey line when the client has no preshared key', () => {
    const cfg = buildWireguardClientConfig({ ...client, preSharedKey: undefined }, inbound, 'example.com', '');
    expect(cfg).not.toContain('PresharedKey');
  });

  it('uses the hosting node address as the endpoint host for node-managed inbounds', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, nodeAddress: 'node.example.net' }, 'master.example.com', '');
    expect(cfg).toContain('Endpoint = node.example.net:51820');
    expect(cfg).not.toContain('master.example.com');
  });

  it('falls back to the panel host when the node address is blank', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, nodeAddress: '   ' }, 'master.example.com', '');
    expect(cfg).toContain('Endpoint = master.example.com:51820');
  });

  it('honors the custom share-address strategy over the node address', () => {
    const cfg = buildWireguardClientConfig(
      client,
      { ...inbound, nodeAddress: 'node.example.net', shareAddrStrategy: 'custom', shareAddr: 'vpn.example.com' },
      'master.example.com',
      '',
    );
    expect(cfg).toContain('Endpoint = vpn.example.com:51820');
  });

  it('honors the listen share-address strategy over the node address', () => {
    const cfg = buildWireguardClientConfig(
      client,
      { ...inbound, nodeAddress: 'node.example.net', shareAddrStrategy: 'listen', listen: '198.51.100.7' },
      'master.example.com',
      '',
    );
    expect(cfg).toContain('Endpoint = 198.51.100.7:51820');
  });

  it('keeps a panel hostname that fails share-host normalization instead of emitting an empty endpoint', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, listen: '0.0.0.0' }, 'wg_gw.corp.lan', '');
    expect(cfg).toContain('Endpoint = wg_gw.corp.lan:51820');
    expect(cfg).not.toContain('Endpoint = :51820');
  });
});

// LUCX-HOOK: AWG — version-gated client config. The obfuscation block is the
// inbound's "ceiling" (every field the inbound carries); buildAwgClientConfig
// trims it to the requested export version so older client apps do not choke on
// unknown fields. v1.5 drops S3/S4 + I1-I5 + HeaderProtectionKey; v2 drops only
// HeaderProtectionKey; v3 keeps everything.
import { buildAwgClientConfig, filterAwgObfuscation, findAwgInbounds, findAwgInbound } from '@/pages/clients/wireguardConfig';
import type { AwgVersion } from '@/lib/xray/inbound-link';

const awgCeilingBlock = [
  'Jc = 5', 'Jmin = 50', 'Jmax = 200',
  'S1 = 30', 'S2 = 60', 'S3 = 20', 'S4 = 25',
  'H1 = 100000-500000', 'H2 = 600000-900000', 'H3 = 1000000-1500000', 'H4 = 1600000-2000000',
  'I1 = <b 0xaa>', 'I2 = <b 0xbb>', 'I3 = <b 0xcc>', 'I4 = <b 0xdd>', 'I5 = <b 0xee>',
  'HeaderProtectionKey = aBcD...base64hpk==',
  'ContentPaddingAddition = 64', 'RekeyAfterTime = 120', 'RekeyTimeout = 5',
  'RejectAfterTime = 180', 'KeepaliveTimeout = 10', 'MaxHandshakeAttempts = 18',
  'RandomTrailers = on', 'DisableCookies = on',
].join('\n');

const awgInbound: InboundOption = {
  id: 7, tag: 'awg-7', remark: 'awg', protocol: 'awg', port: 51820,
  wgPublicKey: 'DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=',
  awgObfuscation: awgCeilingBlock, awgVersion: '3',
};

describe('filterAwgObfuscation', () => {
  it('v3 keeps every field including HeaderProtectionKey', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '3');
    expect(out).toContain('HeaderProtectionKey =');
    expect(out).toContain('S3 = 20');
    expect(out).toContain('I5 =');
  });
  it('v2 drops HeaderProtectionKey but keeps S3/S4 and I1-I5', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '2');
    expect(out).not.toContain('HeaderProtectionKey');
    expect(out).toContain('S3 = 20');
    expect(out).toContain('I5 =');
  });
  it('v1.5 drops S3/S4, I1-I5, and HeaderProtectionKey', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '1.5');
    expect(out).not.toContain('HeaderProtectionKey');
    expect(out).not.toContain('S3 =');
    expect(out).not.toContain('S4 =');
    expect(out).not.toContain('I1 =');
    expect(out).toContain('S1 = 30');
    expect(out).toContain('H1 = 100000-500000');
  });
  it('v3 keeps AWG3 device-level timers and padding', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '3');
    expect(out).toContain('ContentPaddingAddition = 64');
    expect(out).toContain('RekeyAfterTime = 120');
    expect(out).toContain('MaxHandshakeAttempts = 18');
  });
  it('v2 drops AWG3 device-level timers and padding', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '2');
    expect(out).not.toContain('ContentPaddingAddition');
    expect(out).not.toContain('RekeyAfterTime');
    expect(out).not.toContain('RekeyTimeout');
    expect(out).not.toContain('RejectAfterTime');
    expect(out).not.toContain('KeepaliveTimeout');
    expect(out).not.toContain('MaxHandshakeAttempts');
  });
  it('v1.5 drops AWG3 device-level timers and padding', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '1.5');
    expect(out).not.toContain('ContentPaddingAddition');
    expect(out).not.toContain('RekeyAfterTime');
    expect(out).not.toContain('MaxHandshakeAttempts');
  });
  it('v3.1 keeps RandomTrailers and DisableCookies', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '3.1');
    expect(out).toContain('RandomTrailers = on');
    expect(out).toContain('DisableCookies = on');
    expect(out).toContain('HeaderProtectionKey =');
  });
  it('v3 drops RandomTrailers and DisableCookies', () => {
    const out = filterAwgObfuscation(awgCeilingBlock, '3');
    expect(out).not.toContain('RandomTrailers');
    expect(out).not.toContain('DisableCookies');
    expect(out).toContain('HeaderProtectionKey =');
  });
});

describe('buildAwgClientConfig version override', () => {
  it('defaults to the inbound ceiling when no version is given', () => {
    const cfg = buildAwgClientConfig(client, awgInbound, 'example.com', '');
    expect(cfg).toContain('HeaderProtectionKey =');
    expect(cfg).toContain('S3 = 20');
  });
  it('clamps to the ceiling when an override exceeds it', () => {
    const v2Inbound: InboundOption = { ...awgInbound, awgVersion: '2' };
    const cfg = buildAwgClientConfig(client, v2Inbound, 'example.com', '', '3');
    // ceiling is 2 — a '3' override must be ignored (no HPK emitted).
    expect(cfg).not.toContain('HeaderProtectionKey');
  });
  it('honors a v1.5 override on a v3 inbound', () => {
    const cfg = buildAwgClientConfig(client, awgInbound, 'example.com', '', '1.5' as AwgVersion);
    expect(cfg).not.toContain('S3 =');
    expect(cfg).not.toContain('HeaderProtectionKey');
    expect(cfg).toContain('Jc = 5');
  });
  it('collapses range keepalive to lo when exporting below v3', () => {
    const ranged: ClientRecord = { ...client, keepAlive: '15-25' };
    const v15 = buildAwgClientConfig(ranged, awgInbound, 'example.com', '', '1.5');
    expect(v15).toContain('PersistentKeepalive = 15');
    expect(v15).not.toContain('PersistentKeepalive = 15-25');
    const v3 = buildAwgClientConfig(ranged, awgInbound, 'example.com', '', '3');
    expect(v3).toContain('PersistentKeepalive = 15-25');
  });
});

describe('findAwgInbounds multi-attach', () => {
  it('returns every AWG inbound in inboundIds order (not only the first)', () => {
    const byId: Record<number, InboundOption> = {
      1: { id: 1, tag: 'awg1', remark: 'AWG1', protocol: 'awg', port: 1, awgVersion: '1.5', enable: true },
      2: { id: 2, tag: 'vless', remark: 'v', protocol: 'vless', port: 443, enable: true },
      3: { id: 3, tag: 'awg2', remark: 'AWG2', protocol: 'awg', port: 2, awgVersion: '2', enable: true },
      4: { id: 4, tag: 'awg3', remark: 'AWG3', protocol: 'awg', port: 3, awgVersion: '3', enable: false },
    };
    const c: ClientRecord = { email: 'multi', inboundIds: [1, 2, 3, 4], privateKey: 'x' };
    const all = findAwgInbounds(c, byId);
    expect(all.map((i) => i.id)).toEqual([1, 3]);
    expect(all.map((i) => i.awgVersion)).toEqual(['1.5', '2']);
    // first-only helper stays for single-inbound call sites
    expect(findAwgInbound(c, byId)?.id).toBe(1);
  });
});
// END LUCX-HOOK
