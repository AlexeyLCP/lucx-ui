import { deflateSync } from 'node:zlib';
import { describe, expect, it } from 'vitest';

import { vpnConfFromLink } from '@/lib/awg/vpnuri';

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
});
