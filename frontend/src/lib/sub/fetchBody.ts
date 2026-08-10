// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

/**
 * Resolve Amnezia subscription body (vpn:// lines or .conf) for panel Copy/QR.
 *
 * The panel stores Amnezia rows as public HTTPS URLs on the subscription port
 * (`https://host:2096/awg/<id>?format=vpn`). Fetching that URL from the panel
 * origin is cross-origin and fails CORS — Copy then shows "something went
 * wrong". Prefer the same-origin panel proxy:
 *   GET /panel/api/clients/awgBody/:subId?format=vpn|conf
 * which uses the same SubAwgService builder as the public endpoint.
 */

/** Extract subId from an /awg/<subId> subscription URL (any host/port). */
export function extractAwgSubId(url: string): string | null {
  const u = url.trim();
  if (!u) return null;
  try {
    const parsed = new URL(u, typeof window !== 'undefined' ? window.location.origin : 'http://local');
    const m = parsed.pathname.match(/\/awg\/([^/]+)\/?$/i);
    if (m?.[1]) return decodeURIComponent(m[1]);
  } catch {
    /* fall through */
  }
  const m = u.match(/\/awg\/([^/?#]+)/i);
  return m?.[1] ? decodeURIComponent(m[1]) : null;
}

/** True when the URL is the Amnezia vpn:// body endpoint. */
export function isAmneziaVpnUrl(url: string): boolean {
  try {
    const u = new URL(url, typeof window !== 'undefined' ? window.location.origin : 'http://local');
    return u.searchParams.get('format')?.toLowerCase() === 'vpn' || /[?&]format=vpn(?:&|$)/i.test(url);
  } catch {
    return /[?&]format=vpn(?:&|$)/i.test(url);
  }
}

/** True when the URL is the Amnezia .conf subscription endpoint. */
export function isAmneziaConfUrl(url: string): boolean {
  const subId = extractAwgSubId(url);
  if (!subId) return false;
  return !isAmneziaVpnUrl(url);
}

export async function fetchSubscriptionBody(url: string): Promise<string> {
  const u = url.trim();
  if (!u) throw new Error('empty subscription url');

  const subId = extractAwgSubId(u);
  if (subId) {
    const format = isAmneziaVpnUrl(u) ? 'vpn' : 'conf';
    const res = await fetch(`/panel/api/clients/awgBody/${encodeURIComponent(subId)}?format=${format}`, {
      method: 'GET',
      credentials: 'same-origin',
      cache: 'no-store',
      headers: {
        Accept: 'text/plain,*/*',
        'X-Requested-With': 'XMLHttpRequest',
      },
    });
    if (!res.ok) {
      // Fall through to direct fetch (same-origin reverse-proxy setups).
      if (res.status !== 404 && res.status !== 401) {
        const hint = await res.text().catch(() => '');
        throw new Error(`panel awgBody failed: HTTP ${res.status}${hint ? ` ${hint.slice(0, 120)}` : ''}`);
      }
    } else {
      const text = (await res.text()).replace(/^\uFEFF/, '').trim();
      if (text && !text.startsWith('{')) {
        // json error envelope starts with { — treat as failure
        return text;
      }
      if (text.startsWith('{')) {
        throw new Error('panel awgBody returned JSON error');
      }
      if (text) return text;
    }
  }

  // Direct public endpoint (same origin or CORS-open).
  const sep = u.includes('?') ? '&' : '?';
  const target = u.includes('view=raw') ? u : `${u}${sep}view=raw`;
  const res = await fetch(target, {
    method: 'GET',
    headers: { Accept: 'text/plain,*/*' },
    credentials: 'omit',
    cache: 'no-store',
  });
  if (!res.ok) {
    throw new Error(`subscription fetch failed: HTTP ${res.status}`);
  }
  const text = (await res.text()).replace(/^\uFEFF/, '').trim();
  if (!text) throw new Error('subscription body is empty');
  return text;
}
