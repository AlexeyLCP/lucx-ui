import { deflateSync } from 'node:zlib';
import { describe, expect, it } from 'vitest';

import {
  bytesFromBase64Url,
  inflateStored,
  isQCompress,
  vpnConfFromLink,
  vpnUriFromConf,
} from '@/lib/awg/vpnuri';

const sampleConf = `[Interface]
PrivateKey = CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=
Address = 10.200.0.2/32
DNS = 1.1.1.1, 1.0.0.1
MTU = 1320
Jc = 4

# alice
[Peer]
PublicKey = DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 1.2.3.4:51820
`;

function qCompress(data: Uint8Array): Uint8Array {
  const z = deflateSync(data);
  const out = new Uint8Array(4 + z.length);
  new DataView(out.buffer).setUint32(0, data.length);
  out.set(z, 4);
  return out;
}

function vpnUriFromBytes(bytes: Uint8Array): string {
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return `vpn://${btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')}`;
}

describe('vpnConfFromLink', () => {
  it('returns plain .conf from an upstream vpn:// payload', async () => {
    const link = vpnUriFromBytes(new TextEncoder().encode(sampleConf));
    await expect(vpnConfFromLink(link)).resolves.toContain('Endpoint = 1.2.3.4:51820');
  });

  it('inflates a LucX qCompress JSON envelope to the inner .conf', async () => {
    const inner = {
      config: sampleConf,
      client_priv_key: 'CKLAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEE=',
      Jc: '4',
    };
    const env = {
      defaultContainer: 'amnezia-awg',
      containers: [
        {
          container: 'amnezia-awg',
          awg: { last_config: JSON.stringify(inner), isThirdPartyConfig: true },
        },
      ],
    };
    const link = vpnUriFromBytes(qCompress(new TextEncoder().encode(JSON.stringify(env))));
    const conf = await vpnConfFromLink(link);
    expect(conf).toContain('[Interface]');
    expect(conf).toContain('Jc = 4');
    expect(conf).toContain('Endpoint = 1.2.3.4:51820');
    expect(conf).not.toMatch(/\uFFFD/);
  });

  it('returns empty for a non-vpn:// link', async () => {
    await expect(vpnConfFromLink('amneziawg://x')).resolves.toBe('');
  });

  // Cross-language fixture: this exact URI was produced by the Go encoder
  // internal/awg/vpnuri.EncodeConf(sampleConf). If the TS decoder stops
  // understanding the Go side (or vice versa), subscriptions and panel
  // exports diverge — AmneziaVPN would import one but not the other.
  const goEncodedUri =
    'vpn://AAADOHiclFJfb9o-FP0q0f29-pfmD1snSzxQilr-FLFRKm1cFLmJKV7DTWQ7dID47pMdRLWHSWv8Ep1z7tHxuT5CXpEViqQ2wJdHEG8vwI-gzONG6WImtN33K1qrF-BWN5JBKYzN8jMER4RRjsAROggMQZRl9SaLTNUGgS8RotCfq8jTnLufFUPISyXJZqr2w3EUJpETJldpgvDO11rtsle596r-eNL7p28w6LYmPqafXQ7JSr0WuVwh0kyrnbByLPdBN_iQLVKvKLQ0JugGf6ZGup3OHRr6w4LYXTyMEenhceGINIkQaZQH3aCDSIj0XyBKlUtEWs6k1D5a81yqvE12ezf_PswH497itXd9vznczX_-mvxonj5f33xLJ3K_iG4eJqPD9On-axeRem35w5kLdymeBb51pAEVdaXIuihhEqZhh3-KvyTtZjaVsVOxlb6sM-2JrW1aLD0r60pbhHaUIRipd1JndfN82dOHYsMJmPcEDt4TGFgtyDgsq3VlK-DQFDWc2PtjBQ5iS_KgxP_uxZ5WDAq5Fk1p-3-ROIHJtaqtqshxrnhgUJCJgcN5ay2QeMD1FwO7VOPBJEzDDpx-DwDRQfdV';

  it('decodes the Go-encoded envelope (cross-language parity)', async () => {
    const conf = await vpnConfFromLink(goEncodedUri);
    expect(conf).toContain('[Interface]');
    expect(conf).toContain('Jc = 4');
    expect(conf).toContain('Endpoint = 1.2.3.4:51820');
    expect(conf).not.toMatch(/\uFFFD/);
  });
});

describe('vpnUriFromConf (panel-side envelope encoder)', () => {
  it('round-trips its own output through the async decoder', async () => {
    const link = vpnUriFromConf(sampleConf);
    expect(link.startsWith('vpn://')).toBe(true);
    await expect(vpnConfFromLink(link)).resolves.toBe(sampleConf.trim());
  });

  it('rejects payloads without [Interface]/[Peer]', () => {
    expect(vpnUriFromConf('no sections here')).toBe('');
    expect(vpnUriFromConf('')).toBe('');
  });

  // Link labels inflate panel-minted envelopes synchronously (stored-block
  // zlib); Go/sub-service envelopes use real deflate and return null there.
  it('emits stored-block zlib that inflateStored reads synchronously', () => {
    const link = vpnUriFromConf(sampleConf);
    const bytes = bytesFromBase64Url(link.slice('vpn://'.length));
    expect(isQCompress(bytes)).toBe(true);
    const inflated = inflateStored(bytes);
    expect(inflated).not.toBeNull();
    const env = JSON.parse(new TextDecoder().decode(inflated as Uint8Array)) as {
      defaultContainer?: string;
      hostName?: string;
      description?: string;
      containers?: { awg?: { port?: string; protocol_version?: string } }[];
    };
    expect(env.defaultContainer).toBe('amnezia-awg');
    expect(env.hostName).toBe('1.2.3.4');
    expect(env.description).toBe('alice');
    expect(env.containers?.[0]?.awg?.port).toBe('51820');
    expect(env.containers?.[0]?.awg?.protocol_version).toBeUndefined();
  });

  it('marks AWG3 configs with protocol_version 3', () => {
    const v3 = sampleConf.replace('Jc = 4', 'Jc = 4\nHeaderProtectionKey = AAAA');
    const bytes = bytesFromBase64Url(vpnUriFromConf(v3).slice('vpn://'.length));
    const env = JSON.parse(new TextDecoder().decode(inflateStored(bytes) as Uint8Array)) as {
      containers?: { awg?: { protocol_version?: string } }[];
    };
    expect(env.containers?.[0]?.awg?.protocol_version).toBe('3');
  });
});
