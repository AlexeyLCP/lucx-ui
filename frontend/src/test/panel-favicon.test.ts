/* @vitest-environment jsdom */
import { describe, expect, it } from 'vitest';

import { applyPanelFavicon, panelFaviconHref } from '@/lib/panelFavicon';

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

describe('applyPanelFavicon', () => {
  function resetHead() {
    document.head.innerHTML = '';
  }
  function defaultLink() {
    const link = document.createElement('link');
    link.rel = 'icon';
    link.type = 'image/svg+xml';
    link.href = './icons/lucx.svg';
    document.head.appendChild(link);
    return link;
  }

  it('keeps the installed default link when no custom favicon is set', () => {
    resetHead();
    const link = defaultLink();
    applyPanelFavicon('');
    expect(document.querySelector('link[rel="icon"]')).toBe(link);
    expect(link.getAttribute('href')).toBe('./icons/lucx.svg');
  });

  // H6: the LucX default favicon must survive operator-side favicon clearing.
  it('applies a custom favicon in its own link, removing only that one', () => {
    resetHead();
    const link = defaultLink();
    applyPanelFavicon('🐰');
    const managed = document.querySelector<HTMLLinkElement>('link[data-panel-favicon]');
    expect(managed && managed !== link).toBe(true);
    expect(managed?.getAttribute('href')?.startsWith('data:image')).toBe(true);
    expect(link.getAttribute('href')).toBe('./icons/lucx.svg');

    applyPanelFavicon('');
    expect(document.querySelector('link[data-panel-favicon]')).toBeNull();
    expect(document.querySelector('link[rel="icon"]')).toBe(link);
  });

  it('updates the managed link in place on successive edits', () => {
    resetHead();
    applyPanelFavicon('a');
    const managed = document.querySelector<HTMLLinkElement>('link[data-panel-favicon]');
    applyPanelFavicon('b');
    expect(document.querySelectorAll('link[data-panel-favicon]')).toHaveLength(1);
    expect(document.querySelector('link[rel="icon"]')?.getAttribute('href')).toBe(
      managed?.getAttribute('href'),
    );
  });
});
