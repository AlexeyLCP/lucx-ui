// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

/**
 * Resolve Amnezia subscription body (vpn:// lines or .conf) for panel Copy/QR.
 *
 * The panel stores Amnezia rows as public HTTPS URLs on the subscription port
 * (`https://host:2096/awg/<id>?format=vpn`). Fetching that URL from the panel
 * origin is cross-origin and fails CORS — Copy then shows "something went
 * wrong". Prefer the same-origin panel proxy via HttpUtil (honours
 * webBasePath / session / XHR headers):
 *   GET /panel/api/clients/awgBody/:subId?format=vpn|conf
 * → JSON { success, obj: { body, format } }
 */

import { HttpUtil } from '@/utils';

/** Extract subId from an /awg/<subId> subscription URL (any host/port). */
export function extractAwgSubId(url: string): string | null {
  const u = url.trim();
  if (!u) return null;
  try {
    const parsed = new URL(
      u,
      typeof window !== 'undefined' ? window.location.origin : 'http://local',
    );
    const m = parsed.pathname.match(/\/awg\/([^/]+)\/?$/i);
    if (m?.[1]) return decodeURIComponent(m[1]);
  } catch {
    /* fall through */
  }
  const m = u.match(/\/awg\/([^/?#]+)/i);
  return m?.[1] ? decodeURIComponent(m[1]) : null;
}

export function extractAwgInboundId(url: string): string | null {
  try {
    const parsed = new URL(
      url,
      typeof window !== 'undefined' ? window.location.origin : 'http://local',
    );
    const id = parsed.searchParams.get('inboundId');
    if (id && /^\d+$/.test(id) && Number(id) > 0) return id;
  } catch {
    /* fall through */
  }
  const m = url.match(/[?&]inboundId=(\d+)/i);
  return m?.[1] && Number(m[1]) > 0 ? m[1] : null;
}

/** True when the URL is the Amnezia vpn:// body endpoint. */
export function isAmneziaVpnUrl(url: string): boolean {
  try {
    const u = new URL(url, typeof window !== 'undefined' ? window.location.origin : 'http://local');
    return (
      u.searchParams.get('format')?.toLowerCase() === 'vpn' || /[?&]format=vpn(?:&|$)/i.test(url)
    );
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

type AwgBodyObj = { body?: unknown; format?: unknown };

/**
 * Same-origin body for any non-AWG subscription route (sub/json/clash) via
 * GET /panel/api/clients/subBody?url=… — the public sub port has no CORS
 * headers, so a browser fetch from the panel origin fails "Failed to fetch".
 * The backend loops back to the LOCAL sub server; body is byte-identical to
 * what VPN apps receive (base64 / JSON / YAML).
 */
export async function fetchSubBodyViaProxy(url: string): Promise<string> {
  const msg = await HttpUtil.get<AwgBodyObj>(
    '/panel/api/clients/subBody',
    { url },
    { silent: true },
  );
  if (msg.success) {
    const body =
      typeof msg.obj?.body === 'string' ? msg.obj.body.replace(/^\uFEFF/, '').trim() : '';
    if (body) return body;
    throw new Error('panel subBody returned empty body');
  }
  throw new Error(msg.msg || 'panel subBody failed');
}

export async function fetchSubscriptionBody(url: string): Promise<string> {
  const u = url.trim();
  if (!u) throw new Error('empty subscription url');

  const subId = extractAwgSubId(u);
  if (subId) {
    const format = isAmneziaVpnUrl(u) ? 'vpn' : 'conf';
    const inboundId = extractAwgInboundId(u);
    const query: Record<string, string> = { format };
    if (inboundId) query.inboundId = inboundId;
    const msg = await HttpUtil.get<AwgBodyObj>(
      `/panel/api/clients/awgBody/${encodeURIComponent(subId)}`,
      query,
      { silent: true },
    );
    if (msg.success) {
      const body =
        typeof msg.obj?.body === 'string' ? msg.obj.body.replace(/^\uFEFF/, '').trim() : '';
      if (body) return body;
      throw new Error('panel awgBody returned empty body');
    }
    // Fall through to public URL only when the panel proxy route is missing
    // (old binary → 404). Any real backend error — "no AWG configs" or the
    // specific BuildAwgClientConf reason — is rethrown so the caller can show
    // it instead of a generic "something went wrong" (issue #47).
    if (msg.msg && !/not found|404/i.test(msg.msg)) {
      throw new Error(msg.msg);
    }
  }

  // LUCX-HOOK: sub/json/clash rows go through the same-origin panel proxy;
  // a proxy failure (sub service off) still falls through to a direct fetch.
  try {
    return await fetchSubBodyViaProxy(u);
  } catch {
    /* fall through to direct public fetch */
  }
  // END LUCX-HOOK

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
