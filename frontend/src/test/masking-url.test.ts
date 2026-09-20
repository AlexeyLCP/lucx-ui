import { describe, expect, it } from 'vitest';

import { maskingPanelURL } from '@/pages/masking/maskingUrl';

describe('maskingPanelURL', () => {
  it('hides 443 on https and 80 on http', () => {
    expect(maskingPanelURL('shop.example', 443, true, '/secret/')).toBe(
      'https://shop.example/secret/panel/masking',
    );
    expect(maskingPanelURL('shop.example', 80, false, '/secret/')).toBe(
      'http://shop.example/secret/panel/masking',
    );
  });

  it('keeps the panel port and slashes the base path', () => {
    expect(maskingPanelURL('shop.example', 2904, true, '/secret')).toBe(
      'https://shop.example:2904/secret/panel/masking',
    );
  });
});
