import { describe, it, expect } from 'vitest';
import { genCsqttLink, genInboundLinks, genLink } from '@/lib/xray/inbound-link';
import type { Inbound } from '@/schemas/api/inbound';

function csqttInbound(over: Record<string, unknown> = {}): Inbound {
  return {
    protocol: 'csqtt',
    port: 46000,
    listen: '',
    settings: {
      listenAddr: '0.0.0.0:46000',
      password: 'secret',
      subHost: '1.2.3.4',
      vkHashes: 'h1,h2',
      ...((over.settings as object) || {}),
    },
    streamSettings: {},
    sniffing: {},
    ...over,
  } as unknown as Inbound;
}

describe('genCsqttLink', () => {
  it('builds csqtt://connect?v=2', () => {
    const link = genCsqttLink({ inbound: csqttInbound() });
    expect(link.startsWith('csqtt://connect?')).toBe(true);
    expect(link).toContain('v=2');
    expect(link).toContain('host=1.2.3.4');
    expect(link).toContain('peer=46000');
    expect(link).toContain('password=secret');
    expect(link).toContain('hashes=h1%2Bh2');
    expect(link.includes('\n')).toBe(false);
    expect(link).not.toContain('qwdtt://');
  });

  it('falls back to address when subHost empty', () => {
    const ib = csqttInbound({
      settings: { subHost: '', password: 'x', listenAddr: '0.0.0.0:46000' },
    });
    const link = genCsqttLink({ inbound: ib, address: '9.9.9.9' });
    expect(link).toContain('host=9.9.9.9');
  });

  it('returns empty without password', () => {
    const ib = csqttInbound({ settings: { password: '', subHost: '1.1.1.1' } });
    expect(genCsqttLink({ inbound: ib })).toBe('');
  });

  it('genInboundLinks and genLink dispatch', () => {
    const ib = csqttInbound();
    expect(genInboundLinks({ inbound: ib, remark: 'e2e', fallbackHostname: 'x' })).toContain(
      'csqtt://',
    );
    expect(genLink({ inbound: ib, address: '1.2.3.4', client: {}, remark: 'e2e' })).toContain(
      'csqtt://',
    );
  });
});
