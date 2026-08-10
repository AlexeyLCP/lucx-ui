import { describe, expect, it } from 'vitest';

import { isAmneziaVpnUrl } from '@/lib/sub/fetchBody';

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
