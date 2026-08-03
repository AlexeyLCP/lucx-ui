import { describe, expect, it } from 'vitest';

import { maskSubnet, subnetsOverlap } from '@/lib/awg/subnet';

describe('maskSubnet', () => {
  it('reduces a host address to its network prefix', () => {
    expect(maskSubnet('10.8.0.1/24')).toBe('10.8.0.0/24');
    expect(maskSubnet('10.8.1.5/24')).toBe('10.8.1.0/24');
    expect(maskSubnet('192.168.5.100/16')).toBe('192.168.0.0/16');
  });

  it('handles /32 (single host)', () => {
    expect(maskSubnet('10.8.0.1/32')).toBe('10.8.0.1/32');
  });

  it('handles /0 (default route)', () => {
    expect(maskSubnet('0.0.0.0/0')).toBe('0.0.0.0/0');
  });

  it('returns null for invalid input', () => {
    expect(maskSubnet('')).toBeNull();
    expect(maskSubnet('not-an-ip')).toBeNull();
    expect(maskSubnet('10.8.0.1')).toBeNull();
    expect(maskSubnet('10.8.0.1/')).toBeNull();
    expect(maskSubnet('10.8.0.1/33')).toBeNull();
    expect(maskSubnet('10.8.256.1/24')).toBeNull();
    expect(maskSubnet('10.8.0.1/24/extra')).toBeNull();
  });

  it('trims whitespace', () => {
    expect(maskSubnet('  10.8.0.1/24  ')).toBe('10.8.0.0/24');
  });
});

describe('subnetsOverlap', () => {
  it('detects same /24 overlap', () => {
    expect(subnetsOverlap('10.8.0.1/24', '10.8.0.5/24')).toBe(true);
  });

  it('detects overlap with different prefix lengths (wider contains narrower)', () => {
    expect(subnetsOverlap('10.8.0.0/16', '10.8.5.1/24')).toBe(true);
    expect(subnetsOverlap('10.8.5.1/24', '10.8.0.0/16')).toBe(true);
  });

  it('detects non-overlapping /24s in same /16', () => {
    expect(subnetsOverlap('10.8.0.1/24', '10.8.1.1/24')).toBe(false);
    expect(subnetsOverlap('10.8.0.1/24', '10.9.0.1/24')).toBe(false);
  });

  it('returns false when either input is invalid', () => {
    expect(subnetsOverlap('', '10.8.0.1/24')).toBe(false);
    expect(subnetsOverlap('10.8.0.1/24', 'garbage')).toBe(false);
    expect(subnetsOverlap(null as unknown as string, '10.8.0.1/24')).toBe(false);
  });

  it('treats identical subnets as overlapping', () => {
    expect(subnetsOverlap('10.8.0.1/24', '10.8.0.1/24')).toBe(true);
  });

  it('handles the exact dup-subnet bug from the field (awg2 + awg4 both 10.8.0.1/24)', () => {
    expect(subnetsOverlap('10.8.0.1/24', '10.8.0.1/24')).toBe(true);
  });
});
