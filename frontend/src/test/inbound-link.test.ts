/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import {
  amneziawgConfigFromLink,
  genAmneziaWGConfig,
  genAmneziaWGLink,
  genHysteriaLink,
  genInboundLinks,
  genShadowsocksLink,
  genTrojanLink,
  applyVlessRoute,
  genVlessLink,
  genVmessLink,
  genWireguardConfig,
  genWireguardLink,
  genAnytlsLink,
  genTproxyLink,
  preferPublicHost,
  resolveAddr,
} from '@/lib/xray/inbound-link';
import { InboundSchema } from '@/schemas/api/inbound';
import type { AmneziawgInboundSettings } from '@/schemas/protocols/inbound/amneziawg';
import type { WireguardInboundSettings } from '@/schemas/protocols/inbound/wireguard';
// LUCX-HOOK: envelope decode helpers for the vpn:// JSON container assertions
import { bytesFromBase64Url, inflateStored, vpnConfFromLink } from '@/lib/awg/vpnuri';
// END LUCX-HOOK

// Snapshot baseline for the share-link generators. Snapshots were locked
// at the close of the legacy class migration — at that point each
// generator was verified byte-equal to the corresponding legacy Inbound
// class method. Future drift past this baseline is a regression.

const fullFixtures = import.meta.glob<unknown>('./golden/fixtures/inbound-full/*.json', {
  eager: true,
  import: 'default',
});

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

function fixturesForProtocol(protocol: string): Array<[string, Record<string, unknown>]> {
  return Object.entries(fullFixtures)
    .filter(([, raw]) => (raw as { protocol?: string }).protocol === protocol)
    .map(([path, raw]): [string, Record<string, unknown>] => [
      fixtureName(path),
      raw as Record<string, unknown>,
    ])
    .sort(([a], [b]) => a.localeCompare(b));
}

describe('genVmessLink', () => {
  const fixtures = fixturesForProtocol('vmess');
  expect(fixtures.length, 'need at least one vmess full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ id: string; security?: string }> } })
        .settings;
      const client = settings.clients[0];

      const link = genVmessLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientId: client.id,
        security: client.security as never,
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('genVlessLink', () => {
  const fixtures = fixturesForProtocol('vless');
  expect(fixtures.length, 'need at least one vless full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ id: string; flow?: string }> } })
        .settings;
      const client = settings.clients[0];

      const link = genVlessLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientId: client.id,
        flow: client.flow as never,
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('applyVlessRoute', () => {
  const id = '11111111-2222-4333-8444-555555555555';
  it('encodes a single value into the 3rd group and no-ops on invalid input', () => {
    expect(applyVlessRoute(id, '443')).toBe('11111111-2222-01bb-8444-555555555555');
    expect(applyVlessRoute(id, '53')).toBe('11111111-2222-0035-8444-555555555555');
    expect(applyVlessRoute(id, '0')).toBe('11111111-2222-0000-8444-555555555555');
    expect(applyVlessRoute(id, '65535')).toBe('11111111-2222-ffff-8444-555555555555');
    expect(applyVlessRoute(id, '')).toBe(id);
    expect(applyVlessRoute(id, undefined)).toBe(id);
    expect(applyVlessRoute(id, '70000')).toBe(id);
    expect(applyVlessRoute(id, '53,443')).toBe(id);
    expect(applyVlessRoute(id, 'abc')).toBe(id);
    expect(applyVlessRoute('short', '443')).toBe('short');
  });
});

describe('genVlessLink vlessRoute', () => {
  const [, raw] = fixturesForProtocol('vless')[0];
  const typed = InboundSchema.parse(raw);

  it('bakes a host route value into the link UUID 3rd group', () => {
    const link = genVlessLink({
      inbound: typed,
      address: 'example.test',
      port: typed.port,
      forceTls: 'same',
      remark: 'r',
      clientId: '11111111-2222-4333-8444-555555555555',
      flow: '' as never,
      externalProxy: {
        forceTls: 'same',
        dest: 'example.test',
        port: typed.port,
        remark: '',
        vlessRoute: '443',
      },
    });
    expect(link).toContain('vless://11111111-2222-01bb-8444-555555555555@');
  });

  it('leaves the UUID unchanged when no route is set', () => {
    const link = genVlessLink({
      inbound: typed,
      address: 'example.test',
      port: typed.port,
      forceTls: 'same',
      remark: 'r',
      clientId: '11111111-2222-4333-8444-555555555555',
      flow: '' as never,
      externalProxy: null,
    });
    expect(link).toContain('vless://11111111-2222-4333-8444-555555555555@');
  });
});

describe('genTrojanLink', () => {
  const fixtures = fixturesForProtocol('trojan');
  expect(fixtures.length, 'need at least one trojan full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ password: string }> } }).settings;
      const client = settings.clients[0];

      const link = genTrojanLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientPassword: client.password,
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('genHysteriaLink', () => {
  const fixtures = fixturesForProtocol('hysteria');
  expect(fixtures.length, 'need at least one hysteria full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients: Array<{ auth: string }> } }).settings;
      const client = settings.clients[0];

      const link = genHysteriaLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        remark: 'parity-test',
        clientAuth: client.auth,
      });
      expect(link).toMatchSnapshot();
    });
  }

  it('emits the UDP hop range as the v2rayN-compatible mport param', () => {
    const [, raw] = fixtures[0];
    const withHop = {
      ...raw,
      settings: { ...(raw.settings as Record<string, unknown>), version: 2 },
      streamSettings: {
        ...(raw.streamSettings as Record<string, unknown>),
        finalmask: { quicParams: { udpHop: { ports: '20000-50000', interval: '5-10' } } },
      },
    };
    const typed = InboundSchema.parse(withHop);
    const client = (raw.settings as { clients: Array<{ auth: string }> }).clients[0];

    const link = genHysteriaLink({
      inbound: typed,
      address: 'example.test',
      port: typed.port,
      remark: 'hop-test',
      clientAuth: client.auth,
    });

    expect(link.startsWith('hysteria2://')).toBe(true);
    expect(link).toContain(`@example.test:${typed.port}`);
    expect(link).toContain('mport=20000-50000');
    expect(link.endsWith('#hop-test')).toBe(true);
  });

  it('normalizes pinSHA256 to hex for base64, raw-hex and colon-hex pins (issue #4818)', () => {
    const [, raw] = fixtures[0];
    const base64Pin = 'yEfdI5XQl4wHgLggHEsomosoFZfUfCdfLXfT+W2N6cQ=';
    const hexPin = '84491c0312d9e70f519ce24659a2ca7d9c4ec59dc86417ece426945e0f939293';
    const colonPin =
      'C8:47:DD:23:95:D0:97:8C:07:80:B8:20:1C:4B:28:9A:8B:28:15:97:D4:7C:27:5F:2D:77:D3:F9:6D:8D:E9:C4';
    const stream = raw.streamSettings as Record<string, unknown>;
    const tls = stream.tlsSettings as Record<string, unknown>;
    const tlsClientSettings = tls.settings as Record<string, unknown>;
    const withPins = {
      ...raw,
      streamSettings: {
        ...stream,
        tlsSettings: {
          ...tls,
          settings: { ...tlsClientSettings, pinnedPeerCertSha256: [base64Pin, hexPin, colonPin] },
        },
      },
    };
    const typed = InboundSchema.parse(withPins);
    const client = (raw.settings as { clients: Array<{ auth: string }> }).clients[0];

    const link = genHysteriaLink({
      inbound: typed,
      address: 'example.test',
      port: typed.port,
      remark: 'pin-test',
      clientAuth: client.auth,
    });

    const pin = new URL(link).searchParams.get('pinSHA256');
    expect(pin).toBe(
      'c847dd2395d0978c0780b8201c4b289a8b281597d47c275f2d77d3f96d8de9c4,' +
        '84491c0312d9e70f519ce24659a2ca7d9c4ec59dc86417ece426945e0f939293,' +
        'c847dd2395d0978c0780b8201c4b289a8b281597d47c275f2d77d3f96d8de9c4',
    );
  });

  it('emits an external proxy pin as hex pinSHA256 (not pcs)', () => {
    const [, raw] = fixtures[0];
    const typed = InboundSchema.parse(raw);
    const client = (raw.settings as { clients: Array<{ auth: string }> }).clients[0];

    const link = genHysteriaLink({
      inbound: typed,
      address: 'edge.example.com',
      port: 8443,
      remark: 'ep-pin',
      clientAuth: client.auth,
      externalProxy: {
        forceTls: 'tls',
        dest: 'edge.example.com',
        port: 8443,
        remark: 'ep-pin',
        // base64 SHA-256 — must come out hex-normalized for Hysteria.
        pinnedPeerCertSha256: ['yEfdI5XQl4wHgLggHEsomosoFZfUfCdfLXfT+W2N6cQ='],
      },
    });

    const url = new URL(link);
    expect(url.searchParams.get('pinSHA256')).toBe(
      'c847dd2395d0978c0780b8201c4b289a8b281597d47c275f2d77d3f96d8de9c4',
    );
    expect(url.searchParams.has('pcs')).toBe(false);
  });
});

describe('genWireguardLink + genWireguardConfig', () => {
  const fixtures = fixturesForProtocol('wireguard');
  expect(fixtures.length, 'need at least one wireguard full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      if (typed.protocol !== 'wireguard') throw new Error('not a wireguard fixture');
      // InboundSchema is an intersection of two DUs, so TS can't auto-narrow
      // `settings` from `protocol`. The runtime guard above is the real
      // check; this cast just helps the type checker.
      const settings = typed.settings as WireguardInboundSettings;

      const link = genWireguardLink({
        settings,
        address: 'wg.example.test',
        port: typed.port,
        remark: 'wg-peer-1',
        peerIndex: 0,
      });
      const config = genWireguardConfig({
        settings,
        address: 'wg.example.test',
        port: typed.port,
        remark: 'wg-peer-1',
        peerIndex: 0,
      });
      expect({ link, config }).toMatchSnapshot();
    });
  }
});

describe('genWireguardLink + genWireguardConfig multi allowedIPs', () => {
  const settings = {
    secretKey: '',
    mtu: 1280,
    dns: '',
    peers: [
      {
        privateKey: 'cLI',
        allowedIPs: ['10.0.0.2/32', 'fd00::2/128'],
      },
    ],
  } as unknown as WireguardInboundSettings;

  it('joins every allowed IP into the share-link address param', () => {
    const link = genWireguardLink({
      settings,
      address: 'wg.example.test',
      port: 51820,
      remark: 'dual-stack',
      peerIndex: 0,
    });
    const u = new URL(link);
    expect(u.searchParams.get('address')).toBe('10.0.0.2/32,fd00::2/128');
  });

  it('joins every allowed IP into the .conf Address line', () => {
    const config = genWireguardConfig({
      settings,
      address: 'wg.example.test',
      port: 51820,
      remark: 'dual-stack',
      peerIndex: 0,
    });
    expect(config).toContain('Address = 10.0.0.2/32, fd00::2/128\n');
  });
});

// LUCX-HOOK: AWG — version-gated share-link + .conf generation. The emitted
// field set is gated by settings.awgVersion (the server ceiling): S3/S4 and
// I1-I5 are AWG v2+; HeaderProtectionKey is AWG3-only. An override below the
// ceiling clamps the .conf set (clients-page export selector); share-links use
// the ceiling.
import { genAwgConfig, genAwgLink } from '@/lib/xray/inbound-link';
import type { AwgInboundSettings } from '@/schemas/protocols/inbound/awg';

function awgSettings(version: '1.5' | '2' | '3' | '3.1'): AwgInboundSettings {
  return {
    privateKey: 'serverPrivKeyBase64',
    publicKey: '',
    address: '10.8.0.1/24',
    mtu: 1320,
    obfLevel: 3,
    mimicryProfile: 'quic',
    browserProfile: 'chrome',
    region: 'world',
    jc: 5,
    jmin: 50,
    jmax: 200,
    s1: 30,
    s2: 60,
    s3: 20,
    s4: 25,
    h1: '100000-500000',
    h2: '600000-900000',
    h3: '1000000-1500000',
    h4: '1600000-2000000',
    i1: '<b 0xaa>',
    i2: '<b 0xbb>',
    i3: '<b 0xcc>',
    i4: '<b 0xdd>',
    i5: '<b 0xee>',
    headerProtectionKey: 'aBcD...base64hpk==',
    awgVersion: version,
    contentPaddingAddition: '0',
    rekeyAfterTime: '0',
    rekeyTimeout: '0',
    rejectAfterTime: '0',
    keepaliveTimeout: '0',
    maxHandshakeAttempts: '0',
    randomTrailers: version === '3.1',
    disableCookies: false,
    routeThroughXray: true,
    outboundTag: '',
    clients: [
      {
        privateKey: 'clientPrivKeyBase64',
        publicKey: 'peerPub',
        preSharedKey: 'psk',
        allowedIPs: ['10.8.0.2/32'],
        keepAlive: '25',
        email: 'u',
        limitIp: 0,
        totalGB: 0,
        expiryTime: 0,
        enable: true,
        tgId: 0,
        subId: '',
        comment: '',
        reset: 0,
      },
    ] as AwgInboundSettings['clients'],
  };
}

describe('genAwgLink + genAwgConfig version gating', () => {
  it('v3 emits S3/S4, I1-I5, and headerprotectionkey', () => {
    const link = genAwgLink({
      settings: awgSettings('3'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const config = genAwgConfig({
      settings: awgSettings('3'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const u = new URL(link);
    expect(u.searchParams.get('s3')).toBe('20');
    expect(u.searchParams.get('s4')).toBe('25');
    expect(u.searchParams.get('i5')).toBe('<b 0xee>');
    expect(u.searchParams.get('headerprotectionkey')).toBe('aBcD...base64hpk==');
    expect(config).toContain('S3 = 20');
    expect(config).toContain('HeaderProtectionKey = aBcD...base64hpk==');
  });

  it('v2 emits S3/S4 and I1-I5 but NOT headerprotectionkey', () => {
    const link = genAwgLink({
      settings: awgSettings('2'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const config = genAwgConfig({
      settings: awgSettings('2'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const u = new URL(link);
    expect(u.searchParams.get('s3')).toBe('20');
    expect(u.searchParams.get('headerprotectionkey')).toBeNull();
    expect(config).toContain('S3 = 20');
    expect(config).not.toContain('HeaderProtectionKey');
  });

  it('v1.5 omits S3/S4, I1-I5, and headerprotectionkey', () => {
    const link = genAwgLink({
      settings: awgSettings('1.5'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const config = genAwgConfig({
      settings: awgSettings('1.5'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const u = new URL(link);
    expect(u.searchParams.get('s3')).toBeNull();
    expect(u.searchParams.get('i1')).toBeNull();
    expect(u.searchParams.get('headerprotectionkey')).toBeNull();
    expect(config).not.toContain('S3 =');
    expect(config).toContain('S1 = 30');
  });

  it('genAwgConfig clamps an override below the ceiling', () => {
    const config = genAwgConfig({
      settings: awgSettings('3'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
      awgVersionOverride: '1.5',
    });
    expect(config).not.toContain('S3 =');
    expect(config).not.toContain('HeaderProtectionKey');
    expect(config).toContain('Jc = 5');
  });

  it('does not treat form password as PresharedKey', () => {
    const settings = awgSettings('2');
    settings.clients[0] = {
      ...settings.clients[0],
      preSharedKey: '',
      password: 'vgmg2ms952ceemgc',
    };
    const config = genAwgConfig({
      settings,
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const link = genAwgLink({
      settings,
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    expect(config).not.toContain('PresharedKey');
    expect(new URL(link).searchParams.get('presharedkey')).toBeNull();
  });

  it('v3.1 emits RandomTrailers and keeps HPK', () => {
    const settings = awgSettings('3.1');
    const link = genAwgLink({ settings, address: 'wg.example.test', port: 51820, peerIndex: 0 });
    const config = genAwgConfig({
      settings,
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    const u = new URL(link);
    expect(u.searchParams.get('headerprotectionkey')).toBe('aBcD...base64hpk==');
    expect(u.searchParams.get('randomtrailers')).toBe('true');
    expect(u.searchParams.get('disablecookies')).toBeNull();
    expect(config).toContain('HeaderProtectionKey = aBcD...base64hpk==');
    expect(config).toContain('RandomTrailers = on');
    expect(config).not.toContain('DisableCookies');
  });

  it('v3 does NOT emit AdvancedSecurity (kernel ignores it, risks parse errors)', () => {
    const config = genAwgConfig({
      settings: awgSettings('3'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    expect(config).not.toContain('AdvancedSecurity');
  });

  it('v2 does NOT emit AdvancedSecurity', () => {
    const config = genAwgConfig({
      settings: awgSettings('2'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    expect(config).not.toContain('AdvancedSecurity');
  });

  it('v1.5 does NOT emit AdvancedSecurity', () => {
    const config = genAwgConfig({
      settings: awgSettings('1.5'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    expect(config).not.toContain('AdvancedSecurity');
  });

  it('v3 config does not emit zero-valued AWG3 device timers', () => {
    const config = genAwgConfig({
      settings: awgSettings('3'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
    });
    expect(config).not.toContain('ContentPaddingAddition');
    expect(config).not.toContain('RekeyAfterTime');
    expect(config).not.toContain('RekeyTimeout');
    expect(config).not.toContain('RejectAfterTime');
    expect(config).not.toContain('KeepaliveTimeout');
    expect(config).not.toContain('MaxHandshakeAttempts');
  });

  // Д3: a local inbound whose host lacks 3.1 tools must not enable
  // RandomTrailers — the server .conf drops it (awg.AwgVersionFieldsAllowed).
  it('local inbound + unsupported host strips RandomTrailers from genAwgConfig', () => {
    const config = genAwgConfig({
      settings: awgSettings('3.1'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
      nodeId: null,
      hostAwgSupport: { moduleAwg3: false, moduleAwg31: false },
    });
    expect(config).not.toContain('RandomTrailers');
  });

  it('node-hosted inbound keeps RandomTrailers in genAwgConfig regardless of this host', () => {
    const config = genAwgConfig({
      settings: awgSettings('3.1'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
      nodeId: 7,
      hostAwgSupport: { moduleAwg3: false, moduleAwg31: false },
    });
    expect(config).toContain('RandomTrailers = on');
  });

  it('local inbound + unsupported host strips randomtrailers from genAwgLink', () => {
    const link = genAwgLink({
      settings: awgSettings('3.1'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
      nodeId: null,
      hostAwgSupport: { moduleAwg3: false, moduleAwg31: false },
    });
    expect(new URL(link).searchParams.get('randomtrailers')).toBeNull();
  });

  it('node-hosted inbound keeps randomtrailers in genAwgLink regardless of this host', () => {
    const link = genAwgLink({
      settings: awgSettings('3.1'),
      address: 'wg.example.test',
      port: 51820,
      peerIndex: 0,
      nodeId: 7,
      hostAwgSupport: { moduleAwg3: false, moduleAwg31: false },
    });
    expect(new URL(link).searchParams.get('randomtrailers')).toBe('true');
  });
});

// Д3, round 2: the InboundsPage "Export" button goes through genInboundLinks,
// not genAwgConfigs directly — the gate must survive that extra hop too.
describe('genInboundLinks awg export path', () => {
  const inbound = { protocol: 'awg', port: 51820, settings: awgSettings('3.1') };

  it('local inbound + unsupported host strips RandomTrailers via genInboundLinks', () => {
    const block = genInboundLinks({
      inbound: inbound as never,
      hostOverride: 'wg.example.test',
      fallbackHostname: 'fallback.test',
      nodeId: null,
      hostAwgSupport: { moduleAwg3: false, moduleAwg31: false },
    });
    expect(block).not.toContain('RandomTrailers');
  });

  it('node-hosted inbound keeps RandomTrailers via genInboundLinks', () => {
    const block = genInboundLinks({
      inbound: inbound as never,
      hostOverride: 'wg.example.test',
      fallbackHostname: 'fallback.test',
      nodeId: 7,
      hostAwgSupport: { moduleAwg3: false, moduleAwg31: false },
    });
    expect(block).toContain('RandomTrailers = on');
  });
});
// END LUCX-HOOK
// LUCX-HOOK: AmneziaVPN 5.x drops the AWG 3.0 fields when importing a raw
// .conf, so genAmneziaWGLink emits the official Amnezia JSON container —
// vpn:// + Base64URL(qCompress(JSON)) — the same envelope the Go side builds
// in internal/awg/vpnuri. The app's importController parses that container
// natively and keeps every structured field.
interface DecodedAwgProtocol {
  isThirdPartyConfig?: boolean;
  transport_proto?: string;
  port?: string;
  protocol_version?: string;
  last_config?: string;
}

interface DecodedEnvelope {
  defaultContainer?: string;
  hostName?: string;
  description?: string;
  dns1?: string;
  dns2?: string;
  containers?: { container?: string; awg?: DecodedAwgProtocol }[];
}

function decodeEnvelope(link: string): DecodedEnvelope {
  const bytes = bytesFromBase64Url(link.slice('vpn://'.length));
  const inflated = inflateStored(bytes);
  if (!inflated) throw new Error('vpn:// payload is not a stored-block zlib envelope');
  return JSON.parse(new TextDecoder().decode(inflated)) as DecodedEnvelope;
}

describe('genAmneziaWGLink vpn:// scheme', () => {
  const settings = {
    server: {
      publicKey: 'serverPubKey==',
      mtu: 1420,
      primaryDns: '8.8.8.8',
      secondaryDns: '8.8.4.4',
      jc: 5,
      jmin: 10,
      jmax: 50,
      s1: 30,
      s2: 45,
      s3: 10,
      s4: 5,
      h1: '',
      h2: '',
      h3: '',
      h4: '',
      i1: '',
    },
    clients: [
      {
        email: 'peer-1',
        privateKey: 'clientPrivKey==',
        allowedIPs: ['10.8.1.2/32'],
        keepAlive: 25,
      },
    ],
  } as unknown as AmneziawgInboundSettings;

  const input = {
    settings,
    address: 'awg.example.test',
    port: 51820,
    remark: 'awg-peer-1',
    peerIndex: 0,
  };

  it('wraps the .conf text in the Amnezia JSON container; the async decoder recovers it byte-identical', async () => {
    const link = genAmneziaWGLink(input);
    expect(link.startsWith('vpn://')).toBe(true);

    const conf = await vpnConfFromLink(link);
    expect(conf).toBe(genAmneziaWGConfig(input));
    expect(conf).toContain('PrivateKey = clientPrivKey==\n');
    expect(conf).toContain('PublicKey = serverPubKey==\n');
    expect(conf).toContain('Endpoint = awg.example.test:51820');
    // No trailing newline: the text ends on its last set field whichever that
    // is, so the three emitters produce the same shape for the same client.
    expect(conf.endsWith('PersistentKeepalive = 25')).toBe(true);
  });

  it('carries the structured fields the AmneziaVPN import path reads', () => {
    const env = decodeEnvelope(genAmneziaWGLink(input));
    expect(env.defaultContainer).toBe('amnezia-awg');
    expect(env.hostName).toBe('awg.example.test');
    expect(env.description).toBe('awg-peer-1');
    expect(env.dns1).toBe('8.8.8.8');
    expect(env.dns2).toBe('8.8.4.4');

    const container = env.containers?.[0];
    expect(container?.container).toBe('amnezia-awg');
    const awg = container?.awg;
    expect(awg?.isThirdPartyConfig).toBe(true);
    expect(awg?.transport_proto).toBe('udp');
    expect(awg?.port).toBe('51820');
    // S3/S4 set, no AWG 3.0 keys — the app must generate a v2 client
    expect(awg?.protocol_version).toBe('2');

    const inner = JSON.parse(awg?.last_config ?? '{}') as Record<string, unknown>;
    expect(typeof inner.config).toBe('string');
    expect(inner.client_priv_key).toBe('clientPrivKey==');
    expect(inner.server_pub_key).toBe('serverPubKey==');
    expect(inner.client_ip).toBe('10.8.1.2/32');
    expect(inner.psk_key).toBeUndefined();
    expect(inner.hostName).toBe('awg.example.test');
    expect(inner.port).toBe(51820);
    expect(inner.Jc).toBe('5');
    expect(inner.S3).toBe('10');
    expect(inner.allowed_ips).toEqual(['0.0.0.0/0', '::/0']);
    expect(inner.persistent_keep_alive).toBe('25');
  });

  it('omits every unset 3.1 field — a lone HeaderProtectionKey line would break the handshake', async () => {
    const conf = await vpnConfFromLink(genAmneziaWGLink(input));
    for (const absent of [
      'I2',
      'HeaderProtectionKey',
      'ContentPaddingAddition',
      'RekeyAfterTime',
      'RekeyTimeout',
      'RejectAfterTime',
      'KeepaliveTimeout',
      'MaxHandshakeAttempts',
      'RandomTrailers',
      'DisableCookies',
    ]) {
      expect(conf).not.toContain(absent);
    }
  });

  it('returns an empty string when the peer index has no client', () => {
    expect(genAmneziaWGLink({ ...input, peerIndex: 5 })).toBe('');
  });

  // The sync decoder refuses the qCompress envelope (never UTF-8 it — that is
  // the client-card mojibake); the subscription page's async vpnConfFromLink
  // is the supported reverse path for copy/download/QR "Config" blocks.
  it('sync amneziawgConfigFromLink refuses the envelope; the async decoder round-trips it', async () => {
    const link = genAmneziaWGLink(input);
    expect(amneziawgConfigFromLink(link)).toBe('');
    expect(await vpnConfFromLink(link)).toBe(genAmneziaWGConfig(input));
  });
});

describe('amneziawgConfigFromLink edge cases', () => {
  it('returns an empty string for a non-vpn:// link', () => {
    expect(amneziawgConfigFromLink('wireguard://abc')).toBe('');
    expect(amneziawgConfigFromLink('')).toBe('');
  });

  it('returns an empty string for an unparseable vpn:// payload', () => {
    expect(amneziawgConfigFromLink('vpn://not-valid-base64url!!!')).toBe('');
  });

  // LUCX-HOOK: LucX vpn:// is qCompress(JSON); sync decoder must not UTF-8 it.
  it('does not UTF-8 decode a LucX qCompress vpn:// envelope', () => {
    const raw = Uint8Array.from([
      0, 0, 0, 16, 0x78, 0x9c, 0xff, 0x23, 0x20, 0x62, 0x61, 0x64, 0xfe, 0x00,
    ]);
    let bin = '';
    for (const b of raw) bin += String.fromCharCode(b);
    const link = `vpn://${btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')}`;
    const got = amneziawgConfigFromLink(link);
    expect(got).toBe('');
    expect(got).not.toMatch(/\uFFFD/);
  });
});

/*
 * The full AmneziaWG 3.1 parameter block, pinned line-by-line and in order:
 * the emitted client config must carry the identical block the Go server
 * emitter writes (internal/amneziawg.writeObfuscation) or the tunnel breaks.
 */
describe('genAmneziaWGConfig 3.1 parameters', () => {
  const settings = {
    server: {
      publicKey: 'serverPubKey==',
      jc: 4,
      jmin: 40,
      jmax: 100,
      s1: 30,
      s2: 90,
      s3: 20,
      s4: 10,
      h1: '10-2000',
      h2: '3000-5000',
      h3: '6000-8000',
      h4: '9000-11000',
      i1: '<r 64>',
      i2: '<r 80>',
      i3: '',
      i4: '',
      i5: '',
      headerProtectionKey: 'MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=',
      contentPaddingAddition: '16-48',
      rekeyAfterTime: '110-140',
      rekeyTimeout: '4-8',
      rejectAfterTime: '190-250',
      keepaliveTimeout: '9-15',
      maxHandshakeAttempts: '20-40',
      randomTrailers: true,
      disableCookies: true,
    },
    clients: [{ email: 'peer-1', privateKey: 'clientPrivKey==', allowedIPs: ['10.8.1.2/32'] }],
  } as unknown as AmneziawgInboundSettings;

  const input = {
    settings,
    address: 'awg.example.test',
    port: 51820,
    remark: 'awg-31',
    peerIndex: 0,
  };

  it('emits every 3.1 line in the shared emitter order and round-trips through vpn://', async () => {
    const cfg = genAmneziaWGConfig(input);
    const expectedOrder = [
      'Jc = 4',
      'H4 = 9000-11000',
      'I1 = <r 64>',
      'I2 = <r 80>',
      'HeaderProtectionKey = MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=',
      'ContentPaddingAddition = 16-48',
      'RekeyAfterTime = 110-140',
      'RekeyTimeout = 4-8',
      'RejectAfterTime = 190-250',
      'KeepaliveTimeout = 9-15',
      'MaxHandshakeAttempts = 20-40',
      'RandomTrailers = on',
      'DisableCookies = on',
      '[Peer]',
    ];
    let pos = -1;
    for (const line of expectedOrder) {
      const i = cfg.indexOf(line);
      expect(i, `missing or out-of-order: ${line}\n${cfg}`).toBeGreaterThan(pos);
      pos = i;
    }
    expect(cfg).not.toContain('I3');
    const link = genAmneziaWGLink(input);
    expect(decodeEnvelope(link).containers?.[0]?.awg?.protocol_version).toBe('3');
    expect(await vpnConfFromLink(link)).toBe(cfg);
  });
});

describe('resolveAddr precedence', () => {
  const baseInbound = {
    listen: '',
    port: 443,
    protocol: 'vless' as const,
  };

  it('prefers hostOverride over listen and fallback', () => {
    expect(
      resolveAddr(
        { ...baseInbound, listen: '10.0.0.1' } as never,
        'cdn.example.test',
        'fallback.test',
      ),
    ).toBe('cdn.example.test');
  });

  it('uses listen when override is empty and listen is explicit', () => {
    expect(resolveAddr({ ...baseInbound, listen: '10.0.0.1' } as never, '', 'fallback.test')).toBe(
      '10.0.0.1',
    );
  });

  it('skips listen when it is 0.0.0.0 and falls through to fallbackHostname', () => {
    expect(resolveAddr({ ...baseInbound, listen: '0.0.0.0' } as never, '', 'fallback.test')).toBe(
      'fallback.test',
    );
  });

  it('skips a unix socket path listen and falls through to fallbackHostname', () => {
    expect(
      resolveAddr({ ...baseInbound, listen: '/run/xray/in.sock' } as never, '', 'fallback.test'),
    ).toBe('fallback.test');
    expect(
      resolveAddr({ ...baseInbound, listen: '@xray-abstract' } as never, '', 'fallback.test'),
    ).toBe('fallback.test');
  });

  it('falls through to fallbackHostname when listen is empty', () => {
    expect(resolveAddr(baseInbound as never, '', 'fallback.test')).toBe('fallback.test');
  });

  it('uses listen strategy with a shareable IPv6 listen before node override', () => {
    expect(
      resolveAddr(
        {
          ...baseInbound,
          listen: '[2001:db8::1]',
          shareAddrStrategy: 'listen',
          shareAddr: '',
        } as never,
        'node.example.test',
        'fallback.test',
      ),
    ).toBe('[2001:db8::1]');
  });

  it('uses listen strategy to prefer listen and fall back to node override', () => {
    expect(
      resolveAddr(
        { ...baseInbound, listen: '10.0.0.1', shareAddrStrategy: 'listen', shareAddr: '' } as never,
        'node.example.test',
        'fallback.test',
      ),
    ).toBe('10.0.0.1');
    expect(
      resolveAddr(
        { ...baseInbound, listen: '0.0.0.0', shareAddrStrategy: 'listen', shareAddr: '' } as never,
        'node.example.test',
        'fallback.test',
      ),
    ).toBe('node.example.test');
    expect(
      resolveAddr(
        {
          ...baseInbound,
          listen: 'localhost',
          shareAddrStrategy: 'listen',
          shareAddr: '',
        } as never,
        'node.example.test',
        'fallback.test',
      ),
    ).toBe('node.example.test');
  });

  it('uses custom strategy address before node override', () => {
    expect(
      resolveAddr(
        {
          ...baseInbound,
          listen: '10.0.0.1',
          shareAddrStrategy: 'custom',
          shareAddr: 'edge.example.test',
        } as never,
        'node.example.test',
        'fallback.test',
      ),
    ).toBe('edge.example.test');
  });

  it('normalizes a bare IPv6 custom strategy address', () => {
    expect(
      resolveAddr(
        {
          ...baseInbound,
          listen: '10.0.0.1',
          shareAddrStrategy: 'custom',
          shareAddr: '2001:db8::2',
        } as never,
        'node.example.test',
        'fallback.test',
      ),
    ).toBe('[2001:db8::2]');
  });

  it('ignores invalid custom strategy addresses and falls back to node override', () => {
    for (const shareAddr of [
      'https://edge.example.test',
      'edge.example.test:8443',
      '[2001:db8::2]:8443',
      'bad host',
    ]) {
      expect(
        resolveAddr(
          { ...baseInbound, listen: '10.0.0.1', shareAddrStrategy: 'custom', shareAddr } as never,
          'node.example.test',
          'fallback.test',
        ),
      ).toBe('node.example.test');
    }
  });
});

// #4829: reaching the panel through an SSH tunnel (127.0.0.1/localhost) must not
// leak the loopback host into share/QR links; a configured public host wins.
describe('preferPublicHost (loopback fallback)', () => {
  it('keeps a routable browser host as-is even when a public host is configured', () => {
    expect(preferPublicHost('panel.example.com', 'sub.example.com')).toBe('panel.example.com');
    expect(preferPublicHost('203.0.113.7', 'sub.example.com')).toBe('203.0.113.7');
  });

  it('substitutes the public host for loopback browser hosts', () => {
    for (const loop of ['127.0.0.1', 'localhost', '::1', '[::1]', '127.5.6.7']) {
      expect(preferPublicHost(loop, 'sub.example.com')).toBe('sub.example.com');
    }
  });

  it('leaves loopback untouched when no public host is configured', () => {
    expect(preferPublicHost('127.0.0.1', '')).toBe('127.0.0.1');
    expect(preferPublicHost('localhost', '')).toBe('localhost');
  });

  it('an explicit per-inbound listen still wins over the loopback fallback', () => {
    const inbound = { listen: '203.0.113.9', port: 443, protocol: 'vless' as const };
    expect(
      resolveAddr(inbound as never, '', preferPublicHost('127.0.0.1', 'sub.example.com')),
    ).toBe('203.0.113.9');
  });
});

describe('genInboundLinks orchestrator', () => {
  // Every full-inbound fixture should produce the same \r\n-joined link
  // block at this baseline.
  const fixtures = Object.entries(fullFixtures)
    .map(([path, raw]): [string, Record<string, unknown>] => [
      fixtureName(path),
      raw as Record<string, unknown>,
    ])
    .sort(([a], [b]) => a.localeCompare(b));

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const block = genInboundLinks({
        inbound: typed,
        remark: 'parity-test',
        hostOverride: 'override.test',
        fallbackHostname: 'fallback.test',
      });
      expect(block).toMatchSnapshot();
    });
  }
});

describe('genShadowsocksLink', () => {
  const fixtures = fixturesForProtocol('shadowsocks');
  expect(fixtures.length, 'need at least one shadowsocks full-inbound fixture').toBeGreaterThan(0);

  for (const [name, raw] of fixtures) {
    it(`${name}: byte-stable`, () => {
      const typed = InboundSchema.parse(raw);
      const settings = (raw as { settings: { clients?: Array<{ password: string }> } }).settings;
      const client = settings.clients?.[0];

      const link = genShadowsocksLink({
        inbound: typed,
        address: 'example.test',
        port: typed.port,
        forceTls: 'same',
        remark: 'parity-test',
        clientPassword: client?.password ?? '',
        externalProxy: null,
      });
      expect(link).toMatchSnapshot();
    });
  }
});

describe('IPv6 bracket wrapping in share-link authority', () => {
  it('genVlessLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('vless')[0];
    const typed = InboundSchema.parse(raw);
    const clientId = (raw as { settings: { clients: Array<{ id: string }> } }).settings.clients[0]
      .id;

    const link = genVlessLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientId,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genTrojanLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('trojan')[0];
    const typed = InboundSchema.parse(raw);
    const clientPassword = (raw as { settings: { clients: Array<{ password: string }> } }).settings
      .clients[0].password;

    const link = genTrojanLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientPassword,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genShadowsocksLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('shadowsocks')[0];
    const typed = InboundSchema.parse(raw);
    const clientPassword =
      (raw as { settings: { clients?: Array<{ password: string }> } }).settings.clients?.[0]
        ?.password ?? '';

    const link = genShadowsocksLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientPassword,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genHysteriaLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('hysteria')[0];
    const typed = InboundSchema.parse(raw);
    const clientAuth = (raw as { settings: { clients: Array<{ auth: string }> } }).settings
      .clients[0].auth;

    const link = genHysteriaLink({
      inbound: typed,
      address: '2001:db8::1',
      port: 443,
      clientAuth,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('genWireguardLink brackets a bare IPv6 address', () => {
    const [, raw] = fixturesForProtocol('wireguard')[0];
    const typed = InboundSchema.parse(raw);
    if (typed.protocol !== 'wireguard') throw new Error('not a wireguard fixture');
    const settings = typed.settings as WireguardInboundSettings;

    const link = genWireguardLink({
      settings,
      address: '2001:db8::1',
      port: 443,
      peerIndex: 0,
    });
    expect(new URL(link).host).toBe('[2001:db8::1]:443');
  });

  it('does not bracket IPv4 addresses or hostnames', () => {
    const [, raw] = fixturesForProtocol('vless')[0];
    const typed = InboundSchema.parse(raw);
    const clientId = (raw as { settings: { clients: Array<{ id: string }> } }).settings.clients[0]
      .id;

    const v4 = genVlessLink({ inbound: typed, address: '203.0.113.7', port: 443, clientId });
    expect(new URL(v4).host).toBe('203.0.113.7:443');

    const host = genVlessLink({ inbound: typed, address: 'example.test', port: 443, clientId });
    expect(new URL(host).host).toBe('example.test:443');
  });
});

describe('external proxy pinned cert (pcs)', () => {
  const [, raw] = fixturesForProtocol('vless').find(([name]) => name === 'vless-ws-tls')!;
  const typed = InboundSchema.parse(raw);
  const clientId = (raw as { settings: { clients: Array<{ id: string }> } }).settings.clients[0].id;

  it('emits the external proxy pin list as pcs when forcing TLS', () => {
    const link = genVlessLink({
      inbound: typed,
      address: 'edge.example.com',
      port: 8443,
      forceTls: 'tls',
      remark: 'ep-pin',
      clientId,
      externalProxy: {
        forceTls: 'tls',
        dest: 'edge.example.com',
        port: 8443,
        remark: 'ep-pin',
        pinnedPeerCertSha256: ['aa11', 'bb22'],
      },
    });

    expect(new URL(link).searchParams.get('pcs')).toBe('aa11,bb22');
  });

  it('omits pcs when the external proxy forces security off', () => {
    const link = genVlessLink({
      inbound: typed,
      address: 'edge.example.com',
      port: 8080,
      forceTls: 'none',
      remark: 'ep-none',
      clientId,
      externalProxy: {
        forceTls: 'none',
        dest: 'edge.example.com',
        port: 8080,
        remark: 'ep-none',
        pinnedPeerCertSha256: ['aa11'],
      },
    });

    expect(new URL(link).searchParams.has('pcs')).toBe(false);
  });
});

// #5322: the panel copy-link must carry XTLS Vision `flow` for VLESS+XHTTP
// when VLESS encryption (vlessenc) is on, matching the form's flow display
// and the backend subscription. Gating is via canEnableTlsFlow.
describe('genVlessLink flow gating (#5322)', () => {
  function vlessXhttp(encryption: string) {
    return InboundSchema.parse({
      id: 1,
      up: 0,
      down: 0,
      total: 0,
      remark: 'vlessenc',
      enable: true,
      expiryTime: 0,
      listen: '',
      port: 443,
      tag: 'inbound-vless-xhttp',
      sniffing: {
        enabled: false,
        destOverride: [],
        metadataOnly: false,
        routeOnly: false,
        ipsExcluded: [],
        domainsExcluded: [],
      },
      protocol: 'vless',
      settings: {
        clients: [
          {
            id: '11111111-2222-3333-4444-555555555555',
            email: 'a@example.test',
            flow: 'xtls-rprx-vision',
            limitIp: 0,
            totalGB: 0,
            expiryTime: 0,
            enable: true,
            tgId: 0,
            subId: 's1',
            comment: '',
            reset: 0,
          },
        ],
        decryption: 'none',
        encryption,
        fallbacks: [],
      },
      streamSettings: {
        network: 'xhttp',
        xhttpSettings: {},
        security: 'none',
      },
    });
  }

  const clientId = '11111111-2222-3333-4444-555555555555';

  it('emits flow for VLESS+XHTTP when vless encryption is enabled', () => {
    const link = genVlessLink({
      inbound: vlessXhttp('mlkem768x25519plus.native.0rtt.SGVsbG8'),
      address: 'example.test',
      port: 443,
      clientId,
      flow: 'xtls-rprx-vision',
    });
    expect(new URL(link).searchParams.get('flow')).toBe('xtls-rprx-vision');
  });

  it('omits flow for VLESS+XHTTP without vless encryption', () => {
    const link = genVlessLink({
      inbound: vlessXhttp('none'),
      address: 'example.test',
      port: 443,
      clientId,
      flow: 'xtls-rprx-vision',
    });
    expect(new URL(link).searchParams.has('flow')).toBe(false);
  });

  it('still emits flow for classic TCP+REALITY Vision', () => {
    const [, raw] = fixturesForProtocol('vless').find(([name]) => name === 'vless-tcp-reality')!;
    const typed = InboundSchema.parse(raw);
    const link = genVlessLink({
      inbound: typed,
      address: 'example.test',
      port: 443,
      clientId: (raw as { settings: { clients: Array<{ id: string }> } }).settings.clients[0].id,
      flow: 'xtls-rprx-vision',
    });
    expect(new URL(link).searchParams.get('flow')).toBe('xtls-rprx-vision');
  });
});

describe('genVlessLink XHTTP extra compatibility', () => {
  it('emits both sessionID and legacy session keys in XHTTP extra', () => {
    const typed = InboundSchema.parse({
      id: 1,
      up: 0,
      down: 0,
      total: 0,
      remark: 'xhttp-session',
      enable: true,
      expiryTime: 0,
      listen: '',
      port: 443,
      tag: 'inbound-vless-xhttp',
      sniffing: {
        enabled: false,
        destOverride: [],
        metadataOnly: false,
        routeOnly: false,
        ipsExcluded: [],
        domainsExcluded: [],
      },
      protocol: 'vless',
      settings: {
        clients: [
          {
            id: '11111111-2222-3333-4444-555555555555',
            email: 'a@example.test',
            flow: '',
            limitIp: 0,
            totalGB: 0,
            expiryTime: 0,
            enable: true,
            tgId: 0,
            subId: 's1',
            comment: '',
            reset: 0,
          },
        ],
        decryption: 'none',
        encryption: 'none',
        fallbacks: [],
      },
      streamSettings: {
        network: 'xhttp',
        security: 'none',
        xhttpSettings: {
          path: '/sp',
          host: 'edge.example.test',
          mode: 'auto',
          sessionIDPlacement: 'header',
          sessionIDKey: 'X-Session',
        },
      },
    });

    const link = genVlessLink({
      inbound: typed,
      address: 'example.test',
      port: 443,
      clientId: '11111111-2222-3333-4444-555555555555',
    });
    const extra = JSON.parse(new URL(link).searchParams.get('extra') ?? '{}') as Record<
      string,
      unknown
    >;

    expect(extra.sessionIDPlacement).toBe('header');
    expect(extra.sessionIDKey).toBe('X-Session');
    expect(extra.sessionPlacement).toBe('header');
    expect(extra.sessionKey).toBe('X-Session');
  });
});

describe('genAnytlsLink', () => {
  const inbound = {
    protocol: 'anytls',
    port: 8443,
    settings: { password: 'hunter2', sni: 'vpn.example.com' },
  } as Parameters<typeof genAnytlsLink>[0]['inbound'];

  it('emits sni without insecure', () => {
    const link = genAnytlsLink({ inbound, address: 'node.example', remark: 'home' });
    expect(link).toContain('anytls://hunter2@node.example:8443/');
    expect(link).toContain('sni=vpn.example.com');
    expect(link).not.toContain('insecure=');
    expect(link).toContain('#home');
  });

  it('returns empty without sni', () => {
    const noSni = {
      ...inbound,
      settings: { password: 'hunter2', sni: '' },
    } as typeof inbound;
    expect(genAnytlsLink({ inbound: noSni, address: 'node.example' })).toBe('');
  });
});

describe('genTproxyLink', () => {
  it('emits t.me/webproxy without a port', () => {
    const inbound = {
      protocol: 'tproxy',
      port: 443,
      settings: { hostname: 'proxy.example.com', secret: '000102030405060708090a0b0c0d0e0f' },
    } as Parameters<typeof genTproxyLink>[0]['inbound'];
    expect(genTproxyLink({ inbound })).toBe(
      'https://t.me/webproxy?server=proxy.example.com&secret=000102030405060708090a0b0c0d0e0f',
    );
  });
});
