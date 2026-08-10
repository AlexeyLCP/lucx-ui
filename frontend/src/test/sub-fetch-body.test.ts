import { describe, expect, it } from 'vitest';

import { extractAwgSubId, isAmneziaConfUrl, isAmneziaVpnUrl } from '@/lib/sub/fetchBody';

describe('isAmneziaVpnUrl', () => {
  it('detects format=vpn query', () => {
    expect(isAmneziaVpnUrl('https://h.example/awg/abc?format=vpn')).toBe(true);
    expect(isAmneziaVpnUrl('https://h.example/awg/abc?foo=1&format=vpn')).toBe(true);
    expect(isAmneziaVpnUrl('https://h.example/awg/abc?format=VPN')).toBe(true);
  });

  it('rejects conf / plain sub urls', () => {
    expect(isAmneziaVpnUrl('https://h.example/awg/abc')).toBe(false);
    expect(isAmneziaVpnUrl('https://h.example/awg/abc?format=conf')).toBe(false);
    expect(isAmneziaVpnUrl('https://h.example/sub/abc')).toBe(false);
    expect(isAmneziaVpnUrl('vpn://already-encoded')).toBe(false);
  });
});

describe('extractAwgSubId', () => {
  it('parses subId from /awg/ path', () => {
    expect(extractAwgSubId('https://h:2096/awg/d692e589-8480-4c07-a375-4ccb71535a47?format=vpn')).toBe(
      'd692e589-8480-4c07-a375-4ccb71535a47',
    );
    expect(extractAwgSubId('https://h/awg/abc')).toBe('abc');
  });

  it('returns null for non-awg urls', () => {
    expect(extractAwgSubId('https://h/sub/abc')).toBeNull();
    expect(extractAwgSubId('')).toBeNull();
  });
});

describe('isAmneziaConfUrl', () => {
  it('matches awg without format=vpn', () => {
    expect(isAmneziaConfUrl('https://h/awg/abc')).toBe(true);
    expect(isAmneziaConfUrl('https://h/awg/abc?format=vpn')).toBe(false);
  });
});
