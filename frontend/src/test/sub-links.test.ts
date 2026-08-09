import { describe, expect, it } from 'vitest';

import { buildSubLinks } from '@/lib/sub/links';

describe('buildSubLinks', () => {
  it('builds all enabled formats', () => {
    const L = buildSubLinks({
      enable: true,
      subURI: 'https://h/sub/',
      subJsonEnable: true,
      subJsonURI: 'https://h/json/',
      subClashEnable: true,
      subClashURI: 'https://h/clash/',
      subAwgEnable: true,
      subAwgURI: 'https://h/awg/',
    }, 'abc');
    expect(L.sub).toBe('https://h/sub/abc');
    expect(L.json).toBe('https://h/json/abc');
    expect(L.clash).toBe('https://h/clash/abc');
    expect(L.amnezia).toBe('https://h/awg/abc');
    expect(L.amneziaVpn).toBe('https://h/awg/abc?format=vpn');
  });

  it('omits disabled formats', () => {
    const L = buildSubLinks({
      enable: true,
      subURI: 'https://h/sub/',
      subAwgEnable: false,
      subAwgURI: 'https://h/awg/',
    }, 'x');
    expect(L.sub).toBe('https://h/sub/x');
    expect(L.amnezia).toBe('');
    expect(L.amneziaVpn).toBe('');
  });
});
