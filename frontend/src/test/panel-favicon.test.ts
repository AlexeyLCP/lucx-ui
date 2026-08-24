import { describe, expect, it } from 'vitest';

import { panelFaviconHref } from '@/lib/panelFavicon';

describe('panelFaviconHref', () => {
  it('returns empty for a blank value', () => {
    expect(panelFaviconHref('  ')).toBe('');
  });

  it('wraps an emoji as an svg data URI', () => {
    const href = panelFaviconHref('🐰');
    expect(href.startsWith('data:image/svg+xml;base64,')).toBe(true);
    expect(href.length).toBeGreaterThan(40);
  });

  it('rejects a javascript URI', () => {
    expect(panelFaviconHref('javascript:alert(1)')).toBe('');
  });
});
