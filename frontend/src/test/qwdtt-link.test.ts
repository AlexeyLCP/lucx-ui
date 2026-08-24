import { describe, it, expect } from 'vitest';
import { genQwdttLink, genOlcrtcLink, genInboundLinks, genLink } from '@/lib/xray/inbound-link';
import type { Inbound } from '@/schemas/api/inbound';

function qwdttInbound(over: Record<string, unknown> = {}): Inbound {
  return {
    protocol: 'qwdtt',
    port: 56000,
    listen: '',
    settings: {
      listenAddr: '0.0.0.0:56000',
      wgPort: 56001,
      password: 'secret',
      dns: '8.8.8.8',
      subHost: '1.2.3.4:56000',
      vkHashes: 'h1,h2',
      workers: 16,
      clientPort: 9000,
      ...((over.settings as object) || {}),
    },
    streamSettings: {},
    sniffing: {},
    ...over,
  } as unknown as Inbound;
}

describe('genQwdttLink', () => {
  it('builds qwdtt:// from subHost', () => {
    const link = genQwdttLink({ inbound: qwdttInbound(), remark: 'Home' });
    expect(link.startsWith('qwdtt://config?')).toBe(true);
    expect(link).toContain('peer=1.2.3.4%3A56000');
    expect(link).toContain('pass=secret');
    expect(link).toContain('name=Home');
    expect(link).toContain('hashes=h1%2Ch2');
    expect(link.includes('\n')).toBe(false);
    expect(link).not.toMatch(/(?:^|[\r\n])wdtt:\/\//);
  });

  it('falls back to address:port when subHost empty', () => {
    const ib = qwdttInbound({
      settings: { subHost: '', password: 'x', listenAddr: '0.0.0.0:56000' },
    });
    const link = genQwdttLink({ inbound: ib, address: '9.9.9.9', remark: 'r' });
    expect(link).toContain('peer=9.9.9.9%3A56000');
  });

  it('returns empty without password', () => {
    const ib = qwdttInbound({ settings: { password: '', subHost: '1.1.1.1:56000' } });
    expect(genQwdttLink({ inbound: ib })).toBe('');
  });

  it('genInboundLinks and genLink dispatch', () => {
    const ib = qwdttInbound();
    expect(genInboundLinks({ inbound: ib, remark: 'e2e', fallbackHostname: 'x' })).toContain(
      'qwdtt://',
    );
    expect(genLink({ inbound: ib, address: '1.2.3.4', client: {}, remark: 'e2e' })).toContain(
      'qwdtt://',
    );
  });
});

describe('genOlcrtcLink', () => {
  it('builds olcrtc://', () => {
    const ib = {
      protocol: 'olcrtc',
      port: 0,
      listen: '',
      settings: {
        provider: 'jitsi',
        roomId: 'https://meet.jit.si/r',
        cryptoKey: 'a'.repeat(64),
        transport: 'datachannel',
      },
      streamSettings: {},
      sniffing: {},
    } as unknown as Inbound;
    const link = genOlcrtcLink({ inbound: ib });
    expect(link).toBe(`olcrtc://jitsi?datachannel@https://meet.jit.si/r#${'a'.repeat(64)}`);
  });
});
