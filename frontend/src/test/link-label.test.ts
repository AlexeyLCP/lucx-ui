import { describe, it, expect } from 'vitest';

import { parseLinkParts, linkMetaText } from '@/lib/xray/link-label';
import { genAmneziaWGLink } from '@/lib/xray/inbound-link';
import type { AmneziawgInboundSettings } from '@/schemas/protocols/inbound/amneziawg';

// The panel shows the subscription's remark verbatim. Per-client traffic/expiry
// info is rendered only into the body a client app imports (backend, first link
// only), so the panel's display links are already clean — nothing is stripped.
describe('link-label parseLinkParts', () => {
  const linkWith = (remark: string) =>
    `vless://uid@host.example.com:443?type=tcp&security=tls#${encodeURIComponent(remark)}`;

  it('parses protocol / network / security and keeps the remark verbatim', () => {
    const parts = parseLinkParts(linkWith('Germany-john@example.com'));
    expect(parts?.protocol).toBe('Vless');
    expect(parts?.network).toBe('TCP');
    expect(parts?.security).toBe('TLS');
    expect(parts?.remark).toBe('Germany-john@example.com');
    expect(parts?.port).toBe('443');
  });

  it('linkMetaText joins the remark with the port', () => {
    const parts = parseLinkParts(linkWith('Germany-john@example.com'));
    expect(parts && linkMetaText(parts)).toBe('Germany-john@example.com:443');
  });

  it('returns null for an unparseable scheme', () => {
    expect(parseLinkParts('not-a-link')).toBeNull();
  });

  // MTProto share links are tg://proxy deep links whose port rides in a query
  // param, not the URL authority; they carry no transport and use FakeTLS.
  it('labels an mtproto tg://proxy link with its query-param port and FakeTLS', () => {
    const parts = parseLinkParts(
      'tg://proxy?server=host.example.com&port=8443&secret=ee00#mt-inbound',
    );
    expect(parts?.protocol).toBe('MTProto');
    expect(parts?.network).toBe('');
    expect(parts?.security).toBe('FAKETLS');
    expect(parts?.port).toBe('8443');
    expect(parts?.remark).toBe('mt-inbound');
    expect(parts && linkMetaText(parts)).toBe('mt-inbound:8443');
  });

  // LUCX-HOOK: naive+https:// uses a compound scheme; must not fall back to "LINK".
  it('labels a naive+https share link as Naive with HTTPS and #remark', () => {
    const parts = parseLinkParts('naive+https://nxabc:pass@n.example.com:8443#alice@example.com');
    expect(parts?.protocol).toBe('Naive');
    expect(parts?.security).toBe('HTTPS');
    expect(parts?.remark).toBe('alice@example.com');
    expect(parts?.port).toBe('8443');
    expect(parts && linkMetaText(parts)).toBe('alice@example.com:8443');
  });

  it('omits default HTTPS port 443 from naive link meta', () => {
    const parts = parseLinkParts('naive+https://u:p@n.example.com#client1');
    expect(parts?.protocol).toBe('Naive');
    expect(parts?.port).toBe('');
    expect(parts && linkMetaText(parts)).toBe('client1');
  });

  it('labels olcrtc and qwdtt share URIs', () => {
    expect(parseLinkParts('olcrtc://jitsi?datachannel@https://meet.jit.si/r#aabb')?.protocol).toBe(
      'olcRTC',
    );
    expect(parseLinkParts('qwdtt://config?name=Home&peer=1.2.3.4%3A56000&pass=x')?.protocol).toBe(
      'qWDTT',
    );
  });

  it('labels an AmneziaWG vpn:// link with its decoded remark and endpoint port', () => {
    const settings = {
      server: {
        publicKey: 'serverPubKey==',
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
      clients: [{ email: 'peer-1', privateKey: 'clientPrivKey==', allowedIPs: ['10.8.1.2/32'] }],
    } as unknown as AmneziawgInboundSettings;

    const link = genAmneziaWGLink({
      settings,
      address: 'awg.example.test',
      port: 36541,
      remark: 'wg-Майфун',
      peerIndex: 0,
    });

    const parts = parseLinkParts(link);
    expect(parts?.protocol).toBe('AmneziaWG');
    expect(parts?.remark).toBe('wg-Майфун');
    expect(parts?.port).toBe('36541');
    expect(parts && linkMetaText(parts)).toBe('wg-Майфун:36541');
  });

  it('does not treat a LucX qCompress vpn:// envelope as a .conf remark', () => {
    const raw = Uint8Array.from([
      0, 0, 0, 16, 0x78, 0x9c, 0xff, 0x23, 0x20, 0x62, 0x61, 0x64, 0xfe, 0x00,
    ]);
    let bin = '';
    for (const b of raw) bin += String.fromCharCode(b);
    const link = `vpn://${btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')}`;
    const parts = parseLinkParts(link);
    expect(parts?.protocol).toBe('AmneziaWG');
    expect(parts?.remark).toBe('');
    expect(parts?.port).toBe('');
    expect(parts?.remark).not.toMatch(/\uFFFD/);
    expect(parts && linkMetaText(parts)).toBe('');
  });
});
