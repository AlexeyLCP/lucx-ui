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

function toBase64Url(bytes: Uint8Array): string {
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// Qt qCompress framing: 4-byte big-endian uncompressed length + zlib stream.
// The deflate body uses stored (uncompressed) blocks — the most basic valid
// zlib form, accepted by Qt's qUncompress and every inflate implementation,
// which keeps this encoder synchronous (CompressionStream is async-only).
function qCompress(data: Uint8Array): Uint8Array {
  const blocks: number[] = [];
  let offset = 0;
  do {
    const len = Math.min(data.length - offset, 65535);
    const final = offset + len >= data.length;
    const nlen = len ^ 0xffff;
    blocks.push(
      final ? 0x01 : 0x00,
      len & 0xff,
      (len >> 8) & 0xff,
      nlen & 0xff,
      (nlen >> 8) & 0xff,
    );
    for (let i = 0; i < len; i++) blocks.push(data[offset + i]);
    offset += len;
  } while (offset < data.length);

  let a = 1;
  let b = 0;
  for (const byte of data) {
    a = (a + byte) % 65521;
    b = (b + a) % 65521;
  }
  const adler = ((b << 16) | a) >>> 0;

  const out = new Uint8Array(4 + 2 + blocks.length + 4);
  const view = new DataView(out.buffer);
  view.setUint32(0, data.length, false);
  out[4] = 0x78;
  out[5] = 0x01;
  out.set(blocks, 6);
  view.setUint32(out.length - 4, adler, false);
  return out;
}

// Amnezia last_config field names that match .conf keys 1:1 (same table as
// internal/awg/vpnuri on the Go side and Amnezia's own awgProtocolKeys).
const AWG_CONF_KEYS: Record<string, string> = {
  jc: 'Jc',
  jmin: 'Jmin',
  jmax: 'Jmax',
  s1: 'S1',
  s2: 'S2',
  s3: 'S3',
  s4: 'S4',
  h1: 'H1',
  h2: 'H2',
  h3: 'H3',
  h4: 'H4',
  i1: 'I1',
  i2: 'I2',
  i3: 'I3',
  i4: 'I4',
  i5: 'I5',
  headerprotectionkey: 'HeaderProtectionKey',
  contentpaddingaddition: 'ContentPaddingAddition',
  rekeyaftertime: 'RekeyAfterTime',
  rekeytimeout: 'RekeyTimeout',
  rejectaftertime: 'RejectAfterTime',
  keepalivetimeout: 'KeepaliveTimeout',
  maxhandshakeattempts: 'MaxHandshakeAttempts',
  randomtrailers: 'RandomTrailers',
  disablecookies: 'DisableCookies',
};

interface ConfMeta {
  host: string;
  port: number;
  dns1: string;
  dns2: string;
  desc: string;
  inner: Record<string, unknown>;
}

function splitHostPort(s: string): { host: string; port: number } {
  if (s.startsWith('[')) {
    const end = s.lastIndexOf(']');
    if (end < 0) return { host: s, port: 0 };
    const rest = s.slice(end + 1);
    if (!rest.startsWith(':')) return { host: s.slice(1, end), port: 0 };
    const p = Number(rest.slice(1));
    return { host: s.slice(1, end), port: Number.isInteger(p) && p > 0 ? p : 0 };
  }
  const i = s.lastIndexOf(':');
  if (i < 0) return { host: s, port: 0 };
  const p = Number(s.slice(i + 1));
  return { host: s.slice(0, i), port: Number.isInteger(p) && p > 0 ? p : 0 };
}

function parseConf(conf: string): ConfMeta {
  const meta: ConfMeta = {
    host: '',
    port: 0,
    dns1: '',
    dns2: '',
    desc: '',
    inner: { config: conf },
  };
  for (const raw of conf.split('\n')) {
    const line = raw.trim();
    if (!line) continue;
    if (line.startsWith('#')) {
      if (!meta.desc) meta.desc = line.slice(1).trim();
      continue;
    }
    if (line.startsWith('[')) continue;
    const eq = line.indexOf('=');
    if (eq <= 0) continue;
    const key = line.slice(0, eq).trim();
    const val = line.slice(eq + 1).trim();
    if (!val) continue;
    switch (key.toLowerCase()) {
      case 'endpoint':
        if (!meta.host) {
          const { host, port } = splitHostPort(val);
          meta.host = host;
          meta.port = port;
          if (host) meta.inner.hostName = host;
          if (port > 0) meta.inner.port = port;
        }
        break;
      case 'dns': {
        const parts = val.split(',').map((p) => p.trim());
        if (!meta.dns1 && parts.length > 0) meta.dns1 = parts[0];
        if (!meta.dns2 && parts.length > 1) meta.dns2 = parts[1];
        break;
      }
      case 'privatekey':
        meta.inner.client_priv_key = val;
        break;
      case 'address':
        meta.inner.client_ip = val;
        break;
      case 'publickey':
        meta.inner.server_pub_key = val;
        break;
      case 'presharedkey':
        meta.inner.psk_key = val;
        break;
      case 'mtu':
        meta.inner.mtu = val;
        break;
      case 'persistentkeepalive':
        meta.inner.persistent_keep_alive = val;
        break;
      case 'allowedips': {
        const ips = val
          .split(',')
          .map((p) => p.trim())
          .filter((p) => p !== '');
        if (ips.length > 0) meta.inner.allowed_ips = ips;
        break;
      }
      default: {
        const jsonKey = AWG_CONF_KEYS[key.toLowerCase()];
        if (jsonKey) meta.inner[jsonKey] = val;
      }
    }
  }
  return meta;
}

function nonEmpty(inner: Record<string, unknown>, key: string): boolean {
  const v = inner[key];
  return typeof v === 'string' && v.trim() !== '';
}

function atoiPositive(inner: Record<string, unknown>, key: string): boolean {
  const v = inner[key];
  if (typeof v !== 'string') return false;
  const n = Number(v.trim());
  return Number.isInteger(n) && n > 0;
}

// AmneziaVPN picks the protocol generation off protocol_version; without it
// older app builds fall back to v1 obfuscation and the AWG3 handshake dies.
function protocolVersion(inner: Record<string, unknown>): string {
  const awg3 = [
    'HeaderProtectionKey',
    'ContentPaddingAddition',
    'RekeyAfterTime',
    'RekeyTimeout',
    'RejectAfterTime',
    'KeepaliveTimeout',
    'MaxHandshakeAttempts',
  ];
  if (awg3.some((k) => nonEmpty(inner, k))) return '3';
  if (atoiPositive(inner, 'S3') || atoiPositive(inner, 'S4')) return '2';
  if (['H1', 'H2', 'H3', 'H4'].some((k) => nonEmpty(inner, k) && String(inner[k]).includes('-'))) {
    return '2';
  }
  if (nonEmpty(inner, 'I1')) return '2';
  return '';
}

// Synchronous inflate for the stored-block zlib streams vpnUriFromConf emits.
// Real deflate streams (the Go sub service uses zlib compression) return null;
// callers that must decode those use the async DecompressionStream path.
export function inflateStored(bytes: Uint8Array): Uint8Array | null {
  if (bytes.length < 6 || bytes[4] !== 0x78) return null;
  let i = 6;
  const out: number[] = [];
  for (;;) {
    if (i + 5 > bytes.length) return null;
    const header = bytes[i];
    if ((header & 0x06) !== 0) return null;
    const len = bytes[i + 1] | (bytes[i + 2] << 8);
    const nlen = bytes[i + 3] | (bytes[i + 4] << 8);
    if ((len ^ nlen) !== 0xffff) return null;
    i += 5;
    if (i + len > bytes.length) return null;
    for (let k = 0; k < len; k++) out.push(bytes[i + k]);
    i += len;
    if (header & 0x01) break;
  }
  return Uint8Array.from(out);
}

// vpnUriFromConf builds an AmneziaVPN vpn:// share link from an awg-quick
// client .conf, mirroring internal/awg/vpnuri.EncodeConf: vpn:// +
// Base64URL(qCompress(Amnezia JSON container)). AmneziaVPN's import parses
// the container natively and keeps every AWG 3.0 field (a raw-.conf import
// drops HeaderProtectionKey and friends on 5.x).
export function vpnUriFromConf(conf: string): string {
  const trimmed = conf.trim();
  if (!trimmed || !trimmed.includes('[Interface]') || !trimmed.includes('[Peer]')) return '';
  const meta = parseConf(trimmed);

  const awg: Record<string, unknown> = {
    isThirdPartyConfig: true,
    last_config: JSON.stringify(meta.inner),
    transport_proto: 'udp',
  };
  if (meta.port > 0) awg.port = String(meta.port);
  const pv = protocolVersion(meta.inner);
  if (pv) awg.protocol_version = pv;

  const env: Record<string, unknown> = {
    containers: [{ container: 'amnezia-awg', awg }],
    defaultContainer: 'amnezia-awg',
  };
  if (meta.host) env.hostName = meta.host;
  if (meta.desc) env.description = meta.desc;
  if (meta.dns1) env.dns1 = meta.dns1;
  if (meta.dns2) env.dns2 = meta.dns2;

  const payload = new TextEncoder().encode(JSON.stringify(env));
  return `vpn://${toBase64Url(qCompress(payload))}`;
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
