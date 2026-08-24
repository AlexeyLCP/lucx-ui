const MAX_RAW = 16 * 1024;
const MAX_EMOJI = 16;
const DATA_IMAGE =
  /^data:image\/(svg\+xml|png|x-icon|vnd\.microsoft\.icon|webp|gif|jpeg|jpg);base64,[A-Za-z0-9+/=]+$/i;
const BASE64 = /^[A-Za-z0-9+/]+=*$/;

export function panelFaviconHref(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed || trimmed.length > MAX_RAW) return '';
  let value = trimmed;
  if (value.toLowerCase().includes('<link')) {
    const href = value.match(/href\s*=\s*["']([^"']+)["']/i)?.[1]?.trim();
    if (href) value = href;
  }
  if (value.toLowerCase().startsWith('data:')) {
    return DATA_IMAGE.test(value) ? value : '';
  }
  const compact = value.replace(/[\n\r ]/g, '');
  if (compact.length >= 24 && compact.length % 4 === 0 && BASE64.test(compact)) {
    return `data:image/svg+xml;base64,${compact}`;
  }
  const runes = [...value];
  if (runes.length > MAX_EMOJI) return '';
  if (
    runes.some((ch) => {
      const c = ch.codePointAt(0) ?? 0;
      return c < 0x20 || c === 0x7f || ch === '<' || ch === '>' || ch === '"' || ch === "'";
    })
  ) {
    return '';
  }
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">${escapeXml(value)}</text></svg>`;
  return `data:image/svg+xml;base64,${btoa(unescape(encodeURIComponent(svg)))}`;
}

export function applyPanelFavicon(raw: string): void {
  const href = panelFaviconHref(raw);
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!href) {
    link?.remove();
    return;
  }
  if (!link) {
    link = document.createElement('link');
    link.rel = 'icon';
    document.head.appendChild(link);
  }
  link.href = href;
}

function escapeXml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}
