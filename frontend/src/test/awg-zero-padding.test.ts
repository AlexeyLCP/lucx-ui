import { describe, it, expect } from 'vitest';

import { genAwgConfig, genAwgLink, genAmneziaWGConfig } from '@/lib/xray/inbound-link';
import { buildAmneziaWGClientConfig } from '@/pages/clients/amneziawgConfig';
import type { AwgInboundSettings } from '@/schemas/protocols/inbound/awg';
import type { AmneziawgInboundSettings } from '@/schemas/protocols/inbound/amneziawg';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

// S1-S4 = 0 means "do not pad", not "unset": a client that finds no line
// substitutes its own non-zero default and the handshake sizes diverge.
function sLines(conf: string): string[] {
  return conf.split('\n').filter((line) => /^S[1-4] = /.test(line.trim()));
}

function awgZeroSettings(version: '1.5' | '2'): AwgInboundSettings {
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
    s1: 0,
    s2: 0,
    s3: 0,
    s4: 0,
    h1: '',
    h2: '',
    h3: '',
    h4: '',
    i1: '',
    i2: '',
    i3: '',
    i4: '',
    i5: '',
    headerProtectionKey: '',
    awgVersion: version,
    contentPaddingAddition: '0',
    rekeyAfterTime: '0',
    rekeyTimeout: '0',
    rejectAfterTime: '0',
    keepaliveTimeout: '0',
    maxHandshakeAttempts: '0',
    randomTrailers: false,
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

describe('AWG S1-S4 = 0 survives into every artifact', () => {
  const address = 'wg.example.test';
  const port = 51820;

  it('the client export carries all four zeros at v2', () => {
    const config = genAwgConfig({ settings: awgZeroSettings('2'), address, port, peerIndex: 0 });
    expect(sLines(config)).toEqual(['S1 = 0', 'S2 = 0', 'S3 = 0', 'S4 = 0']);
  });

  it('the vpn:// link carries all four zeros at v2', () => {
    const link = genAwgLink({ settings: awgZeroSettings('2'), address, port, peerIndex: 0 });
    const u = new URL(link);
    expect(u.searchParams.get('s1')).toBe('0');
    expect(u.searchParams.get('s2')).toBe('0');
    expect(u.searchParams.get('s3')).toBe('0');
    expect(u.searchParams.get('s4')).toBe('0');
  });

  it('link and export agree on the S block at v2', () => {
    const settings = awgZeroSettings('2');
    const u = new URL(genAwgLink({ settings, address, port, peerIndex: 0 }));
    const fromLink = (['s1', 's2', 's3', 's4'] as const).map(
      (k) => `${k.toUpperCase()} = ${u.searchParams.get(k)}`,
    );
    expect(fromLink).toEqual(sLines(genAwgConfig({ settings, address, port, peerIndex: 0 })));
  });

  it('v1.5 keeps the zeros it owns and still drops S3/S4', () => {
    const settings = awgZeroSettings('1.5');
    const config = genAwgConfig({ settings, address, port, peerIndex: 0 });
    expect(sLines(config)).toEqual(['S1 = 0', 'S2 = 0']);
    const u = new URL(genAwgLink({ settings, address, port, peerIndex: 0 }));
    expect(u.searchParams.get('s1')).toBe('0');
    expect(u.searchParams.get('s2')).toBe('0');
    expect(u.searchParams.has('s3')).toBe(false);
    expect(u.searchParams.has('s4')).toBe(false);
  });
});

describe('AmneziaWG S1-S4 = 0 survives into every artifact', () => {
  const server = {
    publicKey: 'serverPubKey==',
    primaryDns: '8.8.8.8',
    secondaryDns: '',
    mtu: 1420,
    jc: 4,
    jmin: 40,
    jmax: 100,
    s1: 0,
    s2: 0,
    s3: 0,
    s4: 0,
    h1: '',
    h2: '',
    h3: '',
    h4: '',
  };
  const settings = {
    server,
    clients: [
      {
        email: 'peer-1',
        privateKey: 'clientPrivKey==',
        allowedIPs: ['10.8.1.2/32'],
        preSharedKey: 'psk==',
        keepAlive: 25,
      },
    ],
  } as unknown as AmneziawgInboundSettings;

  const client = {
    email: 'peer-1',
    privateKey: 'clientPrivKey==',
    allowedIPs: '10.8.1.2/32',
    preSharedKey: 'psk==',
    keepAlive: 25,
  } as unknown as ClientRecord;
  const inbound = {
    id: 1,
    tag: 'awg-1',
    remark: 'awg',
    port: 51820,
    protocol: 'amneziawg',
    awgServer: server,
  } as unknown as InboundOption;

  // AmneziaWG settings carry no awgVersion, so S3/S4 are unconditional here.
  const want = ['S1 = 0', 'S2 = 0', 'S3 = 0', 'S4 = 0'];

  it('the share-link emitter carries all four zeros', () => {
    const conf = genAmneziaWGConfig({
      settings,
      address: 'awg.example.test',
      port: 51820,
      remark: 'awg-peer-1',
      peerIndex: 0,
    });
    expect(sLines(conf)).toEqual(want);
  });

  it('the clients-page emitter agrees with it', () => {
    const conf = buildAmneziaWGClientConfig(client, inbound, 'awg.example.test');
    expect(sLines(conf)).toEqual(want);
  });
});
