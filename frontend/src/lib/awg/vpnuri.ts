// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

export function isQCompress(bytes: Uint8Array): boolean {
  if (bytes.length < 6) return false;
  return bytes[4] === 0x78 && [0x01, 0x5e, 0x9c, 0xda].includes(bytes[5]);
}

export function bytesFromBase64Url(value: string): Uint8Array {
  const b64 = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

export async function vpnConfFromLink(link: string): Promise<string> {
  const trimmed = link.trim();
  if (!trimmed.startsWith('vpn://')) return '';
  let bytes: Uint8Array;
  try {
    bytes = bytesFromBase64Url(trimmed.slice('vpn://'.length));
  } catch {
    return '';
  }
  let payload = bytes;
  if (isQCompress(bytes)) {
    try {
      payload = await inflateZlib(bytes.subarray(4));
    } catch {
      return '';
    }
  }
  return confFromPayload(payload);
}

async function inflateZlib(data: Uint8Array): Promise<Uint8Array> {
  const ds = new DecompressionStream('deflate');
  const writer = ds.writable.getWriter();
  await writer.write(data as BufferSource);
  await writer.close();
  return new Uint8Array(await new Response(ds.readable).arrayBuffer());
}

function confFromPayload(payload: Uint8Array): string {
  const trimmed = new TextDecoder().decode(payload).trim();
  if (trimmed.startsWith('{')) return confFromJson(trimmed);
  if (trimmed.includes('[Interface]') && trimmed.includes('[Peer]')) return trimmed;
  return '';
}

function confFromJson(s: string): string {
  let env: unknown;
  try {
    env = JSON.parse(s);
  } catch {
    return '';
  }
  if (!env || typeof env !== 'object') return '';
  const containers = (env as { containers?: unknown }).containers;
  if (!Array.isArray(containers) || containers.length === 0) return '';
  const last = containers[containers.length - 1];
  if (!last || typeof last !== 'object') return '';
  const rec = last as Record<string, unknown>;
  const proto = rec.awg ?? rec.wireguard;
  if (!proto || typeof proto !== 'object') return '';
  const lc = (proto as { last_config?: unknown }).last_config;
  if (typeof lc === 'string') {
    const t = lc.trim();
    if (t.startsWith('{')) {
      try {
        const inner = JSON.parse(t) as { config?: unknown };
        if (typeof inner.config === 'string' && inner.config.includes('[Interface]')) {
          return inner.config.trim();
        }
      } catch {
        /* last_config is raw .conf */
      }
    }
    if (t.includes('[Interface]')) return t;
  }
  if (lc && typeof lc === 'object') {
    const c = (lc as { config?: unknown }).config;
    if (typeof c === 'string' && c.includes('[Interface]')) return c.trim();
  }
  return '';
}
