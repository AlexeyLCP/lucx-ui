import { afterEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from './test-utils';
import QrCodeModal from '@/pages/inbounds/qr/QrCodeModal';
import { Status } from '@/models/status';
import type { AwgInboundSettings } from '@/schemas/protocols/inbound/awg';

const { statusHook, genAwgConfigsSpy } = vi.hoisted(() => ({
  statusHook: vi.fn(),
  genAwgConfigsSpy: vi.fn(),
}));

vi.mock('@/api/queries/useStatusQuery', () => ({
  useStatusQuery: statusHook,
}));

vi.mock('@/lib/xray/inbound-link', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/xray/inbound-link')>();
  return {
    ...actual,
    genAwgConfigs: (input: Parameters<typeof actual.genAwgConfigs>[0]) => {
      genAwgConfigsSpy(input);
      return actual.genAwgConfigs(input);
    },
  };
});

afterEach(() => {
  vi.clearAllMocks();
});

function awgSettings(): AwgInboundSettings {
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
    i1: '',
    i2: '',
    i3: '',
    i4: '',
    i5: '',
    headerProtectionKey: 'aBcD...base64hpk==',
    awgVersion: '3.1',
    contentPaddingAddition: '0',
    rekeyAfterTime: '0',
    rekeyTimeout: '0',
    rejectAfterTime: '0',
    keepaliveTimeout: '0',
    maxHandshakeAttempts: '0',
    randomTrailers: true,
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

function localAwgDbInbound() {
  return {
    protocol: 'awg',
    port: 51820,
    listen: '',
    settings: awgSettings() as unknown as Record<string, unknown>,
    streamSettings: {},
    sniffing: {},
    remark: 'test',
    nodeId: null,
  };
}

// D3 finding: before the status poll resolves, moduleAwg3/31 default to false;
// gating a LOCAL inbound on that default silently strips 3.1-only fields.
describe('QrCodeModal AWG host-capability gate', () => {
  it('skips the host-capability gate until status has been fetched at least once', () => {
    statusHook.mockReturnValue({
      status: new Status(),
      fetched: false,
      fetchError: '',
      refresh: async () => {},
    });
    renderWithProviders(<QrCodeModal open onClose={() => {}} dbInbound={localAwgDbInbound()} />);
    expect(genAwgConfigsSpy).toHaveBeenCalledTimes(1);
    expect(genAwgConfigsSpy.mock.calls[0][0].nodeId).toBeUndefined();
  });

  it('gates on the real host capability once status has resolved', () => {
    statusHook.mockReturnValue({
      status: new Status(),
      fetched: true,
      fetchError: '',
      refresh: async () => {},
    });
    renderWithProviders(<QrCodeModal open onClose={() => {}} dbInbound={localAwgDbInbound()} />);
    expect(genAwgConfigsSpy.mock.calls[0][0].nodeId).toBeNull();
  });
});
