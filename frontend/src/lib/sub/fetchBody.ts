// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

/**
 * Fetch a subscription endpoint body for clipboard/QR.
 *
 * The panel stores Amnezia rows as HTTPS URLs (`/awg/<id>?format=vpn`). The
 * tag says "vpn://" because that is what the response body contains — copying
 * the HTTPS URL itself confuses users (and Amnezia QR scanners). This helper
 * resolves the body so Copy/QR hand out the real `vpn://…` / `.conf` payload.
 *
 * Prefer `view=raw` so intermediate proxies that rewrite Accept still get the
 * plain body, not an HTML shell.
 */
export async function fetchSubscriptionBody(url: string): Promise<string> {
  const u = url.trim();
  if (!u) throw new Error('empty subscription url');
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

/** True when the URL is the Amnezia vpn:// body endpoint. */
export function isAmneziaVpnUrl(url: string): boolean {
  try {
    const u = new URL(url, typeof window !== 'undefined' ? window.location.origin : 'http://local');
    return u.searchParams.get('format')?.toLowerCase() === 'vpn' || /[?&]format=vpn(?:&|$)/i.test(url);
  } catch {
    return /[?&]format=vpn(?:&|$)/i.test(url);
  }
}
