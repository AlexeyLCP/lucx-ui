import { Base64, Wireguard } from '@/utils';

import type { Inbound } from '@/schemas/api/inbound';
import type { AmneziawgInboundSettings } from '@/schemas/protocols/inbound/amneziawg';
import type { VlessClient } from '@/schemas/protocols/inbound/vless';
import type { VmessSecurity } from '@/schemas/protocols/shared/vmess';
import type {
  WireguardInboundPeer,
  WireguardInboundSettings,
} from '@/schemas/protocols/inbound/wireguard';
import type { AwgInboundSettings } from '@/schemas/protocols/inbound/awg'; // LUCX-HOOK: AWG
import type { ExternalProxyEntry } from '@/schemas/protocols/stream/external-proxy';
import type { FinalMaskStreamSettings } from '@/schemas/protocols/stream/finalmask';
import type { XHttpStreamSettings } from '@/schemas/protocols/stream/xhttp';

import { collapseKeepaliveForVersion } from '@/lib/awg/timer';
import { getHeaderValue } from './headers';
import { canEnableTlsFlow } from './protocol-capabilities';
import { deriveSpiderX } from './spider-x';

// Share-link generators. Each per-protocol fn takes a typed inbound plus
// client overrides and returns a URL (or '' when the protocol doesn't
// support shareable links). The helpers below were previously static
// methods on the Inbound class; extracting them removes the
// XrayCommonClass dependency and lets these run against Zod-parsed data
// directly.

type ForceTls = 'same' | 'tls' | 'none';
const SHARE_HOSTNAME_RE =
  /^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$/;

// Format a host for interpolation into a URL authority. IPv6 literals are
// wrapped in square brackets per RFC 3986; IPv4 and hostnames are left as-is.
// Any brackets already present are first stripped so the helper is idempotent.
function formatUrlHost(address: string): string {
  const bare = address.replace(/^\[|\]$/g, '');
  return bare.includes(':') ? `[${bare}]` : bare;
}

// xHTTP headers ship as Record<string, string> on the wire (Zod schema)
// rather than the legacy class's HeaderEntry[]. Lookup by case-folded key.
function xhttpHostFallback(xhttp: XHttpStreamSettings | undefined): string {
  return getHeaderValue(xhttp?.headers, 'host');
}

// Pull the bidirectional SplitHTTPConfig fields out of xhttp into a
// compact extra payload. Server-only fields (noSSEHeader, scMaxBufferedPosts,
// scStreamUpServerSecs, serverMaxHeaderBytes) are excluded — the client
// reading the share link wouldn't honor them.
function buildXhttpExtra(xhttp: XHttpStreamSettings | undefined): Record<string, unknown> | null {
  if (!xhttp) return null;
  const extra: Record<string, unknown> = {};

  if (typeof xhttp.mode === 'string' && xhttp.mode.length > 0) {
    extra.mode = xhttp.mode;
  }

  if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
    extra.xPaddingBytes = xhttp.xPaddingBytes;
  }
  if (xhttp.xPaddingObfsMode === true) {
    extra.xPaddingObfsMode = true;
    for (const k of [
      'xPaddingKey',
      'xPaddingHeader',
      'xPaddingPlacement',
      'xPaddingMethod',
    ] as const) {
      const v = xhttp[k];
      if (typeof v === 'string' && v.length > 0) extra[k] = v;
    }
  }

  const stringFields = [
    'uplinkHTTPMethod',
    'sessionIDPlacement',
    'sessionIDKey',
    'sessionIDTable',
    'sessionIDLength',
    'seqPlacement',
    'seqKey',
    'uplinkDataPlacement',
    'uplinkDataKey',
    'scMaxEachPostBytes',
  ] as const;
  // Values matching xray-core's own defaults stay off the wire — old panels
  // seeded them into every config and the literal values are a DPI
  // fingerprint (#5141). Mirrors the sub service's filter.
  const coreDefaults: Partial<Record<(typeof stringFields)[number], string>> = {
    scMaxEachPostBytes: '1000000',
  };
  for (const k of stringFields) {
    const v = xhttp[k];
    if (typeof v === 'string' && v.length > 0 && v !== coreDefaults[k]) extra[k] = v;
  }
  // xray-core #6258 renamed these fields, but older clients still read the
  // legacy names from share-link extra. Emit both names so one link works
  // across old and new clients while the stored panel config stays canonical.
  if (typeof extra.sessionIDPlacement === 'string') {
    extra.sessionPlacement = extra.sessionIDPlacement;
  }
  if (typeof extra.sessionIDKey === 'string') {
    extra.sessionKey = extra.sessionIDKey;
  }

  // Headers on the wire are a record; emit them as a map upstream's
  // SplitHTTPConfig.headers expects, dropping Host (already on the URL).
  if (xhttp.headers && Object.keys(xhttp.headers).length > 0) {
    const headersMap: Record<string, string> = {};
    for (const [name, value] of Object.entries(xhttp.headers)) {
      if (name.toLowerCase() === 'host') continue;
      headersMap[name] = value;
    }
    if (Object.keys(headersMap).length > 0) extra.headers = headersMap;
  }

  return Object.keys(extra).length > 0 ? extra : null;
}

function applyXhttpExtraToObj(
  xhttp: XHttpStreamSettings | undefined,
  obj: Record<string, unknown>,
): void {
  if (!xhttp) return;
  if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
    obj.x_padding_bytes = xhttp.xPaddingBytes;
  }
  const extra = buildXhttpExtra(xhttp);
  if (!extra) return;
  for (const [k, v] of Object.entries(extra)) obj[k] = v;
}

// Recursively checks whether a finalmask payload has any non-empty
// content. Empty arrays / empty objects / empty strings all return false;
// any truthy primitive returns true. Used to decide whether the link
// should carry an `fm` blob at all.
function hasShareableFinalMaskValue(value: unknown): boolean {
  if (value == null) return false;
  if (Array.isArray(value)) return value.some(hasShareableFinalMaskValue);
  if (typeof value === 'object') {
    return Object.values(value as Record<string, unknown>).some(hasShareableFinalMaskValue);
  }
  if (typeof value === 'string') return value.length > 0;
  return true;
}

function serializeFinalMask(finalmask: FinalMaskStreamSettings | undefined): string {
  if (!finalmask) return '';
  return hasShareableFinalMaskValue(finalmask) ? JSON.stringify(finalmask) : '';
}

function applyFinalMaskToObj(
  finalmask: FinalMaskStreamSettings | undefined,
  obj: Record<string, unknown>,
): void {
  const payload = serializeFinalMask(finalmask);
  if (payload.length > 0) obj.fm = payload;
}

function externalProxyAlpn(value: ExternalProxyEntry['alpn']): string {
  if (Array.isArray(value)) return value.filter(Boolean).join(',');
  return '';
}

function externalProxyPins(value: ExternalProxyEntry['pinnedPeerCertSha256']): string {
  if (Array.isArray(value)) return value.filter(Boolean).join(',');
  return '';
}

function applyExternalProxyTLSObj(
  externalProxy: ExternalProxyEntry | null | undefined,
  obj: Record<string, unknown>,
  security: string,
): void {
  if (!externalProxy || security !== 'tls') return;
  const sni =
    externalProxy.sni && externalProxy.sni.length > 0 ? externalProxy.sni : externalProxy.dest;
  if (sni && sni.length > 0) obj.sni = sni;
  if (externalProxy.fingerprint && externalProxy.fingerprint.length > 0)
    obj.fp = externalProxy.fingerprint;
  const alpn = externalProxyAlpn(externalProxy.alpn);
  if (alpn.length > 0) obj.alpn = alpn;
  const pins = externalProxyPins(externalProxy.pinnedPeerCertSha256);
  if (pins.length > 0) obj.pcs = pins;
  if (externalProxy.verifyPeerCertByName && externalProxy.verifyPeerCertByName.length > 0) {
    obj.vcn = externalProxy.verifyPeerCertByName;
  }
  if (externalProxy.echConfigList && externalProxy.echConfigList.length > 0)
    obj.ech = externalProxy.echConfigList;
}

export interface GenVmessLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientId: string;
  security?: VmessSecurity;
  externalProxy?: ExternalProxyEntry | null;
}

// VMess share link: `vmess://` followed by base64-encoded JSON. The JSON
// schema is the v2rayN-compatible "v2" shape. Returns '' if the inbound
// is not vmess so dispatcher code can fall through cleanly.
export function genVmessLink(input: GenVmessLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientId,
    security,
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'vmess') return '';

  const stream = inbound.streamSettings;
  if (!stream) return '';

  const tls = forceTls === 'same' ? (stream.security ?? 'none') : forceTls;
  const obj: Record<string, unknown> = {
    v: '2',
    ps: remark,
    add: address,
    port,
    id: clientId,
    scy: security,
    net: stream.network,
    tls,
  };

  if (stream.network === 'tcp') {
    const tcp = stream.tcpSettings;
    const header = tcp.header;
    if (header) {
      obj.type = header.type;
      if (header.type === 'http') {
        const request = header.request;
        if (request) {
          obj.path = request.path.join(',');
          const host =
            getHeaderValue(header.response?.headers, 'host') ||
            getHeaderValue(request.headers, 'host');
          if (host) obj.host = host;
        }
      }
    } else {
      obj.type = 'none';
    }
  } else if (stream.network === 'kcp') {
    const kcp = stream.kcpSettings;
    obj.mtu = kcp.mtu;
    obj.tti = kcp.tti;
  } else if (stream.network === 'ws') {
    const ws = stream.wsSettings;
    obj.path = ws.path;
    obj.host = ws.host.length > 0 ? ws.host : getHeaderValue(ws.headers, 'host');
  } else if (stream.network === 'grpc') {
    const grpc = stream.grpcSettings;
    obj.path = grpc.serviceName;
    obj.authority = grpc.authority;
    if (grpc.multiMode) obj.type = 'multi';
  } else if (stream.network === 'httpupgrade') {
    const hu = stream.httpupgradeSettings;
    obj.path = hu.path;
    obj.host = hu.host.length > 0 ? hu.host : getHeaderValue(hu.headers, 'host');
  } else if (stream.network === 'xhttp') {
    const xhttp = stream.xhttpSettings;
    obj.path = xhttp.path;
    obj.host = xhttp.host.length > 0 ? xhttp.host : xhttpHostFallback(xhttp);
    obj.type = xhttp.mode;
    applyXhttpExtraToObj(xhttp, obj);
  }

  applyFinalMaskToObj(stream.finalmask, obj);

  if (tls === 'tls' && stream.security === 'tls') {
    const tlsSettings = stream.tlsSettings;
    if (tlsSettings.serverName.length > 0) obj.sni = tlsSettings.serverName;
    if (tlsSettings.settings.fingerprint.length > 0) obj.fp = tlsSettings.settings.fingerprint;
    if (tlsSettings.alpn.length > 0) obj.alpn = tlsSettings.alpn.join(',');
    if (tlsSettings.settings.echConfigList.length > 0) obj.ech = tlsSettings.settings.echConfigList;
    if (tlsSettings.settings.verifyPeerCertByName.length > 0) {
      obj.vcn = tlsSettings.settings.verifyPeerCertByName;
    }
    if (tlsSettings.settings.pinnedPeerCertSha256.length > 0) {
      obj.pcs = tlsSettings.settings.pinnedPeerCertSha256.join(',');
    }
  }

  applyExternalProxyTLSObj(externalProxy, obj, tls);

  return 'vmess://' + Base64.encode(JSON.stringify(obj, null, 2));
}

// Param-style helpers (vless/trojan/ss/hysteria links). These mirror the
// legacy applyXhttpExtraToParams / applyFinalMaskToParams /
// applyExternalProxyTLSParams but write to a URLSearchParams instance
// directly. Number values get coerced via .toString() on set — same as
// what URLSearchParams does internally so the resulting URL bytes match.

function applyXhttpExtraToParams(
  xhttp: XHttpStreamSettings | undefined,
  params: URLSearchParams,
): void {
  if (!xhttp) return;
  params.set('path', xhttp.path);
  const host = xhttp.host.length > 0 ? xhttp.host : xhttpHostFallback(xhttp);
  params.set('host', host);
  params.set('mode', xhttp.mode);
  if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
    params.set('x_padding_bytes', xhttp.xPaddingBytes);
  }
  const extra = buildXhttpExtra(xhttp);
  if (extra) params.set('extra', JSON.stringify(extra));
}

function applyFinalMaskToParams(
  finalmask: FinalMaskStreamSettings | undefined,
  params: URLSearchParams,
): void {
  const payload = serializeFinalMask(finalmask);
  if (payload.length > 0) params.set('fm', payload);
}

function applyExternalProxyTLSParams(
  externalProxy: ExternalProxyEntry | null | undefined,
  params: URLSearchParams,
  security: string,
): void {
  if (!externalProxy || security !== 'tls') return;
  const sni =
    externalProxy.sni && externalProxy.sni.length > 0 ? externalProxy.sni : externalProxy.dest;
  if (sni && sni.length > 0) params.set('sni', sni);
  if (externalProxy.fingerprint && externalProxy.fingerprint.length > 0)
    params.set('fp', externalProxy.fingerprint);
  const alpn = externalProxyAlpn(externalProxy.alpn);
  if (alpn.length > 0) params.set('alpn', alpn);
  const pins = externalProxyPins(externalProxy.pinnedPeerCertSha256);
  if (pins.length > 0) params.set('pcs', pins);
  if (externalProxy.verifyPeerCertByName && externalProxy.verifyPeerCertByName.length > 0) {
    params.set('vcn', externalProxy.verifyPeerCertByName);
  }
  if (externalProxy.echConfigList && externalProxy.echConfigList.length > 0)
    params.set('ech', externalProxy.echConfigList);
}

export interface GenVlessLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientId: string;
  clientKey?: string;
  flow?: VlessClient['flow'];
  externalProxy?: ExternalProxyEntry | null;
}

// Mirror of the Go applyVlessRoute: bake a single 0-65535 value into the UUID's
// 3rd group (bytes 6-7), which xray reads as the vless route. Empty/invalid/non-
// UUID input is returned unchanged.
export function applyVlessRoute(id: string, route: string | undefined): string {
  const r = (route ?? '').trim();
  if (r === '' || !/^\d{1,5}$/.test(r)) return id;
  const n = Number(r);
  if (n > 65535) return id;
  if (!/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/.test(id))
    return id;
  return id.slice(0, 14) + n.toString(16).padStart(4, '0') + id.slice(18);
}

// VLESS share link: vless://<uuid>@<host>:<port>?<query>#<remark>. The
// query carries network type, encryption, network-specific knobs, and
// security-specific knobs (TLS fingerprint/alpn/sni or Reality
// pbk/sid/spx). Returns '' if the inbound isn't vless.
export function genVlessLink(input: GenVlessLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientId,
    clientKey = '',
    flow = '',
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'vless') return '';
  const stream = inbound.streamSettings;
  if (!stream) return '';

  const security = forceTls === 'same' ? stream.security : forceTls;
  const params = new URLSearchParams();
  params.set('type', stream.network ?? 'tcp');
  params.set('encryption', inbound.settings.encryption);

  if (stream.network === 'tcp') {
    const tcp = stream.tcpSettings;
    if (tcp.header?.type === 'http') {
      const request = tcp.header.request;
      if (request) {
        params.set('path', request.path.join(','));
        const host =
          getHeaderValue(tcp.header.response?.headers, 'host') ||
          getHeaderValue(request.headers, 'host');
        if (host) params.set('host', host);
        params.set('headerType', 'http');
      }
    }
  } else if (stream.network === 'kcp') {
    const kcp = stream.kcpSettings;
    params.set('mtu', String(kcp.mtu));
    params.set('tti', String(kcp.tti));
  } else if (stream.network === 'ws') {
    const ws = stream.wsSettings;
    params.set('path', ws.path);
    params.set('host', ws.host.length > 0 ? ws.host : getHeaderValue(ws.headers, 'host'));
  } else if (stream.network === 'grpc') {
    const grpc = stream.grpcSettings;
    params.set('serviceName', grpc.serviceName);
    params.set('authority', grpc.authority);
    if (grpc.multiMode) params.set('mode', 'multi');
  } else if (stream.network === 'httpupgrade') {
    const hu = stream.httpupgradeSettings;
    params.set('path', hu.path);
    params.set('host', hu.host.length > 0 ? hu.host : getHeaderValue(hu.headers, 'host'));
  } else if (stream.network === 'xhttp') {
    applyXhttpExtraToParams(stream.xhttpSettings, params);
  }

  applyFinalMaskToParams(stream.finalmask, params);

  if (security === 'tls') {
    params.set('security', 'tls');
    if (stream.security === 'tls') {
      const tls = stream.tlsSettings;
      params.set('fp', tls.settings.fingerprint);
      params.set('alpn', tls.alpn.join(','));
      if (tls.serverName.length > 0) params.set('sni', tls.serverName);
      if (tls.settings.echConfigList.length > 0) params.set('ech', tls.settings.echConfigList);
      if (tls.settings.verifyPeerCertByName.length > 0) {
        params.set('vcn', tls.settings.verifyPeerCertByName);
      }
      if (tls.settings.pinnedPeerCertSha256.length > 0) {
        params.set('pcs', tls.settings.pinnedPeerCertSha256.join(','));
      }
    }
    applyExternalProxyTLSParams(externalProxy, params, security);
  } else if (security === 'reality') {
    params.set('security', 'reality');
    if (stream.security === 'reality') {
      const reality = stream.realitySettings;
      params.set('pbk', reality.settings.publicKey);
      params.set('fp', reality.settings.fingerprint);

      const sni =
        reality.settings.serverName || reality.serverNames?.[0] || reality.target?.split(':')[0];

      if (sni && sni.length > 0) params.set('sni', sni);

      if (reality.shortIds.length > 0) params.set('sid', reality.shortIds[0]);
      const spx = deriveSpiderX(reality.settings.spiderX, clientKey);
      if (spx.length > 0) params.set('spx', spx);
      if (reality.settings.mldsa65Verify.length > 0)
        params.set('pqv', reality.settings.mldsa65Verify);
    }
  } else {
    params.set('security', 'none');
  }

  // XTLS Vision flow: TCP over tls/reality (classic) or XHTTP+vlessenc (the
  // VLESS-level encryption stands in for transport TLS). Mirrors the backend's
  // vlessFlowAllowed and the form's flow-field gating so panel link, share
  // link and subscription agree.
  if (
    flow.length > 0 &&
    canEnableTlsFlow({
      protocol: inbound.protocol,
      settings: inbound.settings,
      streamSettings: stream,
    })
  ) {
    params.set('flow', flow);
  }

  const url = new URL(
    `vless://${applyVlessRoute(clientId, externalProxy?.vlessRoute)}@${formatUrlHost(address)}:${port}`,
  );
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

// Shared network-branch writer used by trojan + shadowsocks links.
// VLESS and VMess don't call this because they have minor per-protocol
// quirks inline (vmess maps `multi` differently into obj.type; vless sets
// encryption=none up-front).
function writeNetworkParams(
  stream: NonNullable<Inbound['streamSettings']>,
  params: URLSearchParams,
): void {
  if (stream.network === 'tcp') {
    const tcp = stream.tcpSettings;
    if (tcp.header?.type === 'http') {
      const request = tcp.header.request;
      if (request) {
        params.set('path', request.path.join(','));
        const host =
          getHeaderValue(tcp.header.response?.headers, 'host') ||
          getHeaderValue(request.headers, 'host');
        if (host) params.set('host', host);
        params.set('headerType', 'http');
      }
    }
  } else if (stream.network === 'kcp') {
    const kcp = stream.kcpSettings;
    params.set('mtu', String(kcp.mtu));
    params.set('tti', String(kcp.tti));
  } else if (stream.network === 'ws') {
    const ws = stream.wsSettings;
    params.set('path', ws.path);
    params.set('host', ws.host.length > 0 ? ws.host : getHeaderValue(ws.headers, 'host'));
  } else if (stream.network === 'grpc') {
    const grpc = stream.grpcSettings;
    params.set('serviceName', grpc.serviceName);
    params.set('authority', grpc.authority);
    if (grpc.multiMode) params.set('mode', 'multi');
  } else if (stream.network === 'httpupgrade') {
    const hu = stream.httpupgradeSettings;
    params.set('path', hu.path);
    params.set('host', hu.host.length > 0 ? hu.host : getHeaderValue(hu.headers, 'host'));
  } else if (stream.network === 'xhttp') {
    applyXhttpExtraToParams(stream.xhttpSettings, params);
  }
}

function writeTlsParams(
  stream: NonNullable<Inbound['streamSettings']>,
  params: URLSearchParams,
): void {
  if (stream.security !== 'tls') return;
  const tls = stream.tlsSettings;
  params.set('fp', tls.settings.fingerprint);
  params.set('alpn', tls.alpn.join(','));
  if (tls.settings.echConfigList.length > 0) params.set('ech', tls.settings.echConfigList);
  if (tls.serverName.length > 0) params.set('sni', tls.serverName);
  if (tls.settings.verifyPeerCertByName.length > 0) {
    params.set('vcn', tls.settings.verifyPeerCertByName);
  }
  if (tls.settings.pinnedPeerCertSha256.length > 0) {
    params.set('pcs', tls.settings.pinnedPeerCertSha256.join(','));
  }
}

// Reality query-string writer shared by VLESS and Trojan. Preserves the
// legacy SNI-omission quirk (see genVlessLink for the full story).
function writeRealityParams(
  stream: NonNullable<Inbound['streamSettings']>,
  params: URLSearchParams,
  clientKey: string,
): void {
  if (stream.security !== 'reality') return;
  const reality = stream.realitySettings;
  params.set('pbk', reality.settings.publicKey);
  params.set('fp', reality.settings.fingerprint);

  const sni =
    reality.settings.serverName || reality.serverNames?.[0] || reality.target?.split(':')[0];

  if (sni && sni.length > 0) params.set('sni', sni);

  if (reality.shortIds.length > 0) params.set('sid', reality.shortIds[0]);
  const spx = deriveSpiderX(reality.settings.spiderX, clientKey);
  if (spx.length > 0) params.set('spx', spx);
  if (reality.settings.mldsa65Verify.length > 0) params.set('pqv', reality.settings.mldsa65Verify);
}

export interface GenTrojanLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientPassword: string;
  clientKey?: string;
  externalProxy?: ExternalProxyEntry | null;
}

// Trojan share link: trojan://<password>@<host>:<port>?<query>#<remark>.
// Same query-string shape as VLESS minus the `encryption` and `flow`
// fields. Returns '' if the inbound isn't trojan.
export function genTrojanLink(input: GenTrojanLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientPassword,
    clientKey = '',
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'trojan') return '';
  const stream = inbound.streamSettings;
  if (!stream) return '';

  const security = forceTls === 'same' ? stream.security : forceTls;
  const params = new URLSearchParams();
  params.set('type', stream.network ?? 'tcp');

  writeNetworkParams(stream, params);
  applyFinalMaskToParams(stream.finalmask, params);

  if (security === 'tls') {
    params.set('security', 'tls');
    writeTlsParams(stream, params);
    applyExternalProxyTLSParams(externalProxy, params, security);
  } else if (security === 'reality') {
    params.set('security', 'reality');
    writeRealityParams(stream, params, clientKey);
  } else {
    params.set('security', 'none');
  }

  const url = new URL(
    `trojan://${encodeURIComponent(clientPassword)}@${formatUrlHost(address)}:${port}`,
  );
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

export interface GenShadowsocksLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  clientPassword?: string;
  externalProxy?: ExternalProxyEntry | null;
}

// Shadowsocks 2022 share link. The userinfo portion is base64(method:pw)
// for single-user and base64(method:settingsPw:clientPw) for multi-user
// 2022-blake3. Legacy SS (non-2022) leaves the password out of the
// userinfo entirely — matches the legacy class's password-array logic.
// Note: legacy `isSSMultiUser` returns true for everything except
// 2022-blake3-chacha20-poly1305 (a curious classification, but we
// preserve it for byte-stable parity).
export function genShadowsocksLink(input: GenShadowsocksLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    clientPassword = '',
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'shadowsocks') return '';
  const stream = inbound.streamSettings;
  if (!stream) return '';
  const settings = inbound.settings;

  const security = forceTls === 'same' ? stream.security : forceTls;
  const params = new URLSearchParams();
  params.set('type', stream.network ?? 'tcp');

  writeNetworkParams(stream, params);
  applyFinalMaskToParams(stream.finalmask, params);

  if (security === 'tls') {
    params.set('security', 'tls');
    writeTlsParams(stream, params);
    applyExternalProxyTLSParams(externalProxy, params, security);
  }

  // SIP002 clients (v2rayN) ignore type/headerType/host/path and only read
  // `plugin`. Re-encode a TCP http header as obfs-local so they build a
  // matching tcp/http outbound (v2rayN forces request path "/").
  if ((stream.network ?? 'tcp') === 'tcp' && params.get('headerType') === 'http') {
    const host = params.get('host') ?? '';
    params.delete('type');
    params.delete('headerType');
    params.delete('host');
    params.delete('path');
    params.set('plugin', `obfs-local;obfs=http;obfs-host=${host}`);
  }

  const isSS2022 = settings.method.substring(0, 4) === '2022';
  const isSSMultiUser = settings.method !== '2022-blake3-chacha20-poly1305';
  const passwords: string[] = [];
  if (isSS2022) passwords.push(settings.password);
  if (isSSMultiUser) passwords.push(clientPassword);

  if (isSS2022) {
    // SIP022 (2022-blake3-*) forbids base64 userinfo: method and each key are
    // percent-encoded, joined by literal ':' separators. Built by hand because
    // `new URL` would re-encode the inner key separator to %3A.
    const userinfo = [settings.method, ...passwords].map(encodeURIComponent).join(':');
    let link = `ss://${userinfo}@${formatUrlHost(address)}:${port}`;
    const query = params.toString();
    if (query) link += `?${query}`;
    link += `#${encodeURIComponent(remark)}`;
    return link;
  }

  // SIP002 userinfo is base64(method:pw).
  const userinfo = Base64.encode(`${settings.method}:${passwords.join(':')}`, true);
  const url = new URL(`ss://${userinfo}@${formatUrlHost(address)}:${port}`);
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

export interface GenHysteriaLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  remark?: string;
  clientAuth: string;
  externalProxy?: ExternalProxyEntry | null;
}

// Hysteria2's pinSHA256 must be a 64-char lowercase hex string — Xray-core
// clients hex-decode it and crash on a base64 value. The panel stores pins as
// base64 (xray-core's native TLS format / the generate button) or hex, either
// bare or colon-separated as `openssl x509 -fingerprint -sha256` emits it. Each
// entry is coerced to bare hex. Values that are neither a 32-byte hex nor a
// 32-byte base64 SHA-256 pass through unchanged.
function hysteriaPinHex(pin: string): string {
  const stripped = pin.trim().replace(/:/g, '');
  if (/^[0-9a-fA-F]{64}$/.test(stripped)) return stripped.toLowerCase();
  try {
    const binary = atob(pin.trim().replace(/-/g, '+').replace(/_/g, '/'));
    if (binary.length !== 32) return pin;
    let hex = '';
    for (let i = 0; i < binary.length; i++) {
      hex += binary.charCodeAt(i).toString(16).padStart(2, '0');
    }
    return hex;
  } catch {
    return pin;
  }
}

// Hysteria share link: hysteria2://<auth>@<host>:<port>?<query>#<remark>.
// The scheme is always hysteria2 — xray-core builds version 2 only, so the
// settings schema pins it there and the subscription server emits the same
// scheme. Salamander obfuscation pulls its password from
// finalmask.udp[type=salamander] when present; the broader finalmask payload
// still rides under `fm` like the other links.
//
// Note: legacy genHysteriaLink reads stream.tls.settings.allowInsecure,
// which isn't a field on TlsStreamSettings.Settings — the guard is always
// false. We omit the `insecure` param here to stay byte-stable.
export function genHysteriaLink(input: GenHysteriaLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    remark = '',
    clientAuth,
    externalProxy = null,
  } = input;

  if (inbound.protocol !== 'hysteria') return '';
  const stream = inbound.streamSettings;
  if (!stream || stream.security !== 'tls') return '';

  const scheme = 'hysteria2';

  const params = new URLSearchParams();
  params.set('security', 'tls');
  const tls = stream.tlsSettings;
  if (tls.settings.fingerprint.length > 0) params.set('fp', tls.settings.fingerprint);
  if (tls.alpn.length > 0) params.set('alpn', tls.alpn.join(','));
  if (tls.settings.echConfigList.length > 0) params.set('ech', tls.settings.echConfigList);
  if (tls.serverName.length > 0) params.set('sni', tls.serverName);
  if (tls.settings.verifyPeerCertByName.length > 0) {
    params.set('vcn', tls.settings.verifyPeerCertByName);
  }
  if (tls.settings.pinnedPeerCertSha256.length > 0) {
    params.set('pinSHA256', tls.settings.pinnedPeerCertSha256.map(hysteriaPinHex).join(','));
  }
  // An external-proxy entry can pin a different endpoint's certificate.
  // Hysteria carries it as hex `pinSHA256` (not the `pcs` other protocols
  // use), so coerce each entry through hysteriaPinHex like the main pin.
  if (Array.isArray(externalProxy?.pinnedPeerCertSha256)) {
    const epPins = externalProxy.pinnedPeerCertSha256.filter(Boolean).map(hysteriaPinHex);
    if (epPins.length > 0) params.set('pinSHA256', epPins.join(','));
  }

  const udpMasks = stream.finalmask?.udp;
  if (Array.isArray(udpMasks)) {
    const salamander = udpMasks.find((m) => m?.type === 'salamander');
    const obfsPassword = salamander?.settings?.password;
    if (typeof obfsPassword === 'string' && obfsPassword.length > 0) {
      params.set('obfs', 'salamander');
      params.set('obfs-password', obfsPassword);
    }
  }

  applyFinalMaskToParams(stream.finalmask, params);

  const hopPorts = stream.finalmask?.quicParams?.udpHop?.ports?.trim() ?? '';
  if (hopPorts.length > 0) {
    params.set('mport', hopPorts);
  }

  const url = new URL(`${scheme}://${clientAuth}@${formatUrlHost(address)}:${port}`);
  for (const [key, value] of params) url.searchParams.set(key, value);
  url.hash = encodeURIComponent(remark);
  return url.toString();
}

export interface GenMtprotoLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  clientSecret?: string;
}

// Builds a per-client Telegram proxy deep link for an mtproto inbound from the
// client's own FakeTLS secret. No remark fragment is added: Telegram proxy deep
// links have no name field, and a trailing "#remark" gets folded into the last
// query value by lenient parsers, breaking the server address. The panel shows
// the remark separately from the link.
export function genMtprotoLink(input: GenMtprotoLinkInput): string {
  const { inbound, address, port = inbound.port, clientSecret = '' } = input;
  if (inbound.protocol !== 'mtproto') return '';
  if (clientSecret.length === 0) return '';
  const url = new URL('tg://proxy');
  url.searchParams.set('server', address);
  url.searchParams.set('port', String(port));
  url.searchParams.set('secret', clientSecret);
  return url.toString();
}

export interface GenWireguardLinkInput {
  settings: WireguardInboundSettings;
  address: string;
  port: number;
  remark?: string;
  peerIndex: number;
}

// Wireguard share link: wireguard://<peerPrivKey>@<host>:<port>
//   ?publickey=<serverPub>&address=<peerAllowedIP>&mtu=<mtu>#<remark>
// pubKey is derived from the server's secretKey via Wireguard.generateKeypair
// at call time (Zod's schema stores secretKey only — pubKey isn't on the
// wire). Returns '' when the peer index is out of bounds.
export function genWireguardLink(input: GenWireguardLinkInput): string {
  const { settings, address, port, remark = '', peerIndex } = input;
  const peer = settings.peers[peerIndex];
  if (!peer) return '';

  const url = new URL(`wireguard://${formatUrlHost(address)}:${port}`);
  url.username = peer.privateKey ?? '';

  const pubKey =
    settings.secretKey.length > 0 ? Wireguard.generateKeypair(settings.secretKey).publicKey : '';
  if (pubKey.length > 0) url.searchParams.set('publickey', pubKey);
  if (peer.allowedIPs.length > 0) {
    url.searchParams.set('address', peer.allowedIPs.join(','));
  }
  if (typeof settings.mtu === 'number' && settings.mtu > 0) {
    url.searchParams.set('mtu', String(settings.mtu));
  }

  url.hash = encodeURIComponent(remark);
  return url.toString();
}

// Plain-text WireGuard client config (.conf format). Mirrors the legacy
// getWireguardTxt — same DNS defaults (1.1.1.1, 1.0.0.1), MTU optional,
// presharedKey + keepAlive only emitted when present on the peer. The
// final newline structure follows the legacy: no newline after Endpoint,
// optional preSharedKey appended with leading \n, keepAlive appended
// with leading \n AND trailing \n.
export function genWireguardConfig(input: GenWireguardLinkInput): string {
  const { settings, address, port, remark = '', peerIndex } = input;
  const peer = settings.peers[peerIndex];
  if (!peer) return '';

  const pubKey =
    settings.secretKey.length > 0 ? Wireguard.generateKeypair(settings.secretKey).publicKey : '';

  let txt = `[Interface]\n`;
  txt += `PrivateKey = ${peer.privateKey ?? ''}\n`;
  txt += `Address = ${peer.allowedIPs.join(', ')}\n`;
  txt += `DNS = ${settings.dns || '1.1.1.1, 1.0.0.1'}\n`;
  if (typeof settings.mtu === 'number' && settings.mtu > 0) {
    txt += `MTU = ${settings.mtu}\n`;
  }
  txt += `\n# ${remark}\n`;
  txt += `[Peer]\n`;
  txt += `PublicKey = ${pubKey}\n`;
  txt += `AllowedIPs = 0.0.0.0/0, ::/0\n`;
  txt += `Endpoint = ${address}:${port}`;
  if (peer.preSharedKey && peer.preSharedKey.length > 0) {
    txt += `\nPresharedKey = ${peer.preSharedKey}`;
  }
  {
    const ka = awgTimerEmit(peer.keepAlive as string | number | undefined);
    if (ka) txt += `\nPersistentKeepalive = ${ka}\n`;
  }
  return txt;
}

// Shared input shape for both the per-client vpn:// link and .conf
// builders below — settings.clients (not a peers array; unlike WireGuard,
// AmneziaWG was multi-client from day one, so there's no legacy format).
export interface GenAmneziaWGLinkInput {
  settings: AmneziawgInboundSettings;
  address: string;
  port: number;
  remark?: string;
  peerIndex: number;
}

function amneziaWGHLine(key: string, value: string | undefined, fallback: string): string {
  return `${key} = ${value && value.trim() !== '' ? value : fallback}`;
}

// Base64url (RFC 4648 §5), no padding — matches the real AmneziaVPN app's
// own Qt::Base64UrlEncoding | Qt::OmitTrailingEquals framing for vpn:// links.
function toBase64Url(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// AmneziaWG share link: vpn://<base64url .conf text>, matching the real
// AmneziaVPN app's own share-link scheme. The app's import path base64url-
// decodes, best-effort qUncompresses (falls back to the raw bytes when the
// input isn't qCompress-framed, which plain text never is), then parses the
// result as a flat bag of "Key = Value" lines regardless of which
// [Interface]/[Peer] section they came from — so wrapping the same .conf
// text genAmneziaWGConfig already produces is sufficient; no JSON schema or
// compression needs replicating. Confirmed against the app's own source
// (importController.cpp's checkConfigFormat/extractWireGuardConfig).
export function genAmneziaWGLink(input: GenAmneziaWGLinkInput): string {
  const cfgText = genAmneziaWGConfig(input);
  if (!cfgText) return '';
  return `vpn://${toBase64Url(cfgText)}`;
}

// Plain-text AmneziaWG client config (.conf format). Mirrors
// genWireguardConfig, plus the obfuscation lines every AmneziaWG client must
// share with the server (see internal/amneziawg.writeObfuscation on the Go
// side).
export function genAmneziaWGConfig(input: GenAmneziaWGLinkInput): string {
  const { settings, address, port, remark = '', peerIndex } = input;
  const client = settings.clients[peerIndex];
  if (!client) return '';
  const server = settings.server;

  // These land unescaped in the .conf; a newline would inject a config line
  // (e.g. a rogue PostUp) — same guard as the panel's other two emitters.
  for (const v of [
    client.privateKey ?? '',
    server.primaryDns ?? '',
    server.secondaryDns ?? '',
    remark,
  ]) {
    if (/[\r\n]/.test(v)) return '';
  }

  let txt = `[Interface]\n`;
  txt += `PrivateKey = ${client.privateKey ?? ''}\n`;
  txt += `Address = ${(client.allowedIPs ?? []).join(', ')}\n`;
  const dns = [server.primaryDns, server.secondaryDns].filter((v) => !!v && v.trim() !== '');
  if (dns.length > 0) txt += `DNS = ${dns.join(', ')}\n`;
  if (typeof server.mtu === 'number' && server.mtu > 0) {
    txt += `MTU = ${server.mtu}\n`;
  }
  txt += `Jc = ${server.jc}\n`;
  txt += `Jmin = ${server.jmin}\n`;
  txt += `Jmax = ${server.jmax}\n`;
  txt += `S1 = ${server.s1}\n`;
  txt += `S2 = ${server.s2}\n`;
  if (server.s3) txt += `S3 = ${server.s3}\n`;
  if (server.s4) txt += `S4 = ${server.s4}\n`;
  txt += `${amneziaWGHLine('H1', server.h1, '1')}\n`;
  txt += `${amneziaWGHLine('H2', server.h2, '2')}\n`;
  txt += `${amneziaWGHLine('H3', server.h3, '3')}\n`;
  txt += `${amneziaWGHLine('H4', server.h4, '4')}\n`;
  if (server.i1) txt += `I1 = ${server.i1}\n`;
  if (server.i2) txt += `I2 = ${server.i2}\n`;
  if (server.i3) txt += `I3 = ${server.i3}\n`;
  if (server.i4) txt += `I4 = ${server.i4}\n`;
  if (server.i5) txt += `I5 = ${server.i5}\n`;
  const optional31: Array<[string, string | undefined]> = [
    ['HeaderProtectionKey', server.headerProtectionKey],
    ['ContentPaddingAddition', server.contentPaddingAddition],
    ['RekeyAfterTime', server.rekeyAfterTime],
    ['RekeyTimeout', server.rekeyTimeout],
    ['RejectAfterTime', server.rejectAfterTime],
    ['KeepaliveTimeout', server.keepaliveTimeout],
    ['MaxHandshakeAttempts', server.maxHandshakeAttempts],
  ];
  for (const [key, value] of optional31) {
    if (value && value.trim() !== '') txt += `${key} = ${value}\n`;
  }
  if (server.randomTrailers) txt += `RandomTrailers = on\n`;
  if (server.disableCookies) txt += `DisableCookies = on\n`;
  // Peer field order follows wg-quick(8) and the panel's other two AmneziaWG
  // emitters (amneziaWGConfigText in Go, buildAmneziaWGClientConfig); all three
  // are independent implementations and must not drift apart.
  txt += `\n# ${remark}\n`;
  txt += `[Peer]\n`;
  txt += `PublicKey = ${server.publicKey ?? ''}\n`;
  if (client.preSharedKey && client.preSharedKey.length > 0) {
    txt += `PresharedKey = ${client.preSharedKey}\n`;
  }
  txt += `AllowedIPs = 0.0.0.0/0, ::/0\n`;
  txt += `Endpoint = ${address}:${port}`;
  if (typeof client.keepAlive === 'number' && client.keepAlive > 0) {
    txt += `\nPersistentKeepalive = ${client.keepAlive}`;
  }
  return txt;
}

export interface GenAmneziaWGFanoutInput {
  inbound: Inbound;
  remark?: string;
  hostOverride?: string;
  fallbackHostname: string;
}

export function genAmneziaWGLinks(input: GenAmneziaWGFanoutInput): string {
  const { inbound, remark = '', hostOverride = '', fallbackHostname } = input;
  if (inbound.protocol !== 'amneziawg') return '';
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const sep = '-';
  const settings = inbound.settings as AmneziawgInboundSettings;
  const clients = settings.clients ?? [];
  return clients
    .map((c, i) =>
      genAmneziaWGLink({
        settings,
        address: addr,
        port: inbound.port,
        remark: `${remark}${sep}${i + 1}${wgPeerCommentSuffix(c)}`,
        peerIndex: i,
      }),
    )
    .join('\r\n');
}

export function genAmneziaWGConfigs(input: GenAmneziaWGFanoutInput): string {
  const { inbound, remark = '', hostOverride = '', fallbackHostname } = input;
  if (inbound.protocol !== 'amneziawg') return '';
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const sep = '-';
  const settings = inbound.settings as AmneziawgInboundSettings;
  const clients = settings.clients ?? [];
  return clients
    .map((c, i) =>
      genAmneziaWGConfig({
        settings,
        address: addr,
        port: inbound.port,
        remark: `${remark}${sep}${i + 1}${wgPeerCommentSuffix(c)}`,
        peerIndex: i,
      }),
    )
    .join('\r\n');
}

export function wireguardConfigFromLink(link: string, fallbackRemark = ''): string {
  let url: URL;
  try {
    url = new URL(link);
  } catch {
    return '';
  }
  const scheme = url.protocol.replace(/:$/, '');
  if (scheme !== 'wireguard' && scheme !== 'wg') return '';

  const params = url.searchParams;
  const pick = (...keys: string[]): string => {
    for (const k of keys) {
      const v = params.get(k);
      if (v) return v;
    }
    return '';
  };

  let privateKey: string;
  try {
    privateKey = decodeURIComponent(url.username);
  } catch {
    privateKey = url.username;
  }
  const host = url.hostname;
  const endpoint = host ? (url.port ? `${host}:${url.port}` : host) : '';
  const address = pick('address', 'ip') || '10.0.0.2/32';
  const publicKey = pick('publickey', 'publicKey', 'public_key', 'peerPublicKey');
  const dns = pick('dns') || '1.1.1.1, 1.0.0.1';
  const mtu = pick('mtu');
  const psk = pick('presharedkey', 'preshared_key', 'pre-shared-key', 'psk');
  const keepAlive = pick('keepalive', 'persistentkeepalive', 'persistent_keepalive');
  const allowedIPs = pick('allowedips', 'allowed_ips') || '0.0.0.0/0, ::/0';

  let remark = fallbackRemark;
  try {
    const decoded = decodeURIComponent(url.hash.replace(/^#/, ''));
    if (decoded) remark = decoded;
  } catch {
    const raw = url.hash.replace(/^#/, '');
    if (raw) remark = raw;
  }

  const lines = [
    '[Interface]',
    `PrivateKey = ${privateKey}`,
    `Address = ${address}`,
    `DNS = ${dns}`,
  ];
  if (mtu && Number(mtu) > 0) lines.push(`MTU = ${mtu}`);
  lines.push('');
  if (remark) lines.push(`# ${remark}`);
  lines.push('[Peer]', `PublicKey = ${publicKey}`);
  if (psk) lines.push(`PresharedKey = ${psk}`);
  lines.push(`AllowedIPs = ${allowedIPs}`, `Endpoint = ${endpoint}`);
  {
    const ka = awgTimerEmit(keepAlive);
    if (ka) lines.push(`PersistentKeepalive = ${ka}`);
  }
  return lines.join('\n');
}

// Reverse of toBase64Url above -- recovers a vpn:// link's plain .conf
// payload for display/copy/download/QR, the AmneziaWG counterpart of
// wireguardConfigFromLink. Simpler than that function: a vpn:// link's
// payload already *is* the .conf text (see genAmneziaWGLink's own doc
// comment), so there's nothing to reconstruct from query params -- just
// decode. Mirrors link-label.tsx's own private fromBase64Url (used there
// only to pull the remark/port back out for the tag label); duplicated
// rather than imported since both are tiny, self-contained, and each
// file already owns the matching encode or decode half of this pair.
function fromBase64Url(value: string): string {
  const b64 = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

export function amneziawgConfigFromLink(link: string): string {
  const trimmed = link.trim();
  if (!trimmed.startsWith('vpn://')) return '';
  try {
    return fromBase64Url(trimmed.slice('vpn://'.length));
  } catch {
    return '';
  }
}

export type { WireguardInboundPeer };

function isUnixSocketListen(listen: string): boolean {
  return listen.startsWith('/') || listen.startsWith('@');
}

function normalizeShareHost(host: string): string {
  const h = host.trim();
  if (h.length === 0 || h.includes('://') || h.startsWith('//') || /[/?#@]/.test(h)) {
    return '';
  }
  if (h.startsWith('[')) {
    if (!h.endsWith(']')) return '';
    try {
      return new URL(`http://${h}`).hostname;
    } catch {
      return '';
    }
  }
  if (h.includes(':')) {
    try {
      return new URL(`http://[${h}]`).hostname;
    } catch {
      return '';
    }
  }
  return SHARE_HOSTNAME_RE.test(h) ? h : '';
}

function isShareableHost(host: string): boolean {
  const h = normalizeShareHost(host)
    .replace(/^\[|\]$/g, '')
    .toLowerCase();
  if (h.length === 0) return false;
  if (h === '0.0.0.0' || h === '::' || h === '::0') return false;
  if (h === 'localhost' || h === '::1' || h.startsWith('127.')) return false;
  return true;
}

function shareableListenFrom(listen: string): string {
  const trimmed = listen.trim();
  return trimmed.length > 0 && !isUnixSocketListen(trimmed) && isShareableHost(trimmed)
    ? normalizeShareHost(trimmed)
    : '';
}

type ShareAddrStrategy = 'node' | 'listen' | 'custom';

function normalizeShareAddrStrategy(strategy: string | undefined): ShareAddrStrategy {
  return strategy === 'listen' || strategy === 'custom' ? strategy : 'node';
}

// ShareHostFields is the subset of an inbound resolveShareHost needs, so callers
// holding only a lightweight projection (e.g. the clients page InboundOption)
// can pick the same host as the full-inbound share/QR path.
export interface ShareHostFields {
  listen?: string;
  shareAddr?: string;
  shareAddrStrategy?: string;
}

// resolveShareHost picks the host that goes into share/QR links, the browser-side
// analog of the backend resolveInboundAddress. hostOverride is the hosting node's
// address (empty for this panel's own inbounds); fallbackHostname is the
// already-resolved panel/public host used as the last resort — kept verbatim when
// it fails normalization (e.g. an underscore intranet hostname) so the last
// resort never degrades to an empty host.
export function resolveShareHost(
  fields: ShareHostFields,
  hostOverride: string,
  fallbackHostname: string,
): string {
  const nodeAddr = normalizeShareHost(hostOverride);
  const listenAddr = shareableListenFrom(fields.listen ?? '');
  const customAddr = normalizeShareHost(fields.shareAddr ?? '');
  const fallbackAddr = normalizeShareHost(fallbackHostname) || fallbackHostname.trim();
  switch (normalizeShareAddrStrategy(fields.shareAddrStrategy)) {
    case 'listen':
      return listenAddr || nodeAddr || fallbackAddr;
    case 'custom':
      return customAddr || nodeAddr || listenAddr || fallbackAddr;
    default:
      return nodeAddr || listenAddr || fallbackAddr;
  }
}

// Orchestrators.
// resolveAddr picks the host that goes into share/QR links. The default
// `node` strategy keeps the previous node-address-first behavior for
// node-managed inbounds; other strategies let a row prefer its listen address
// or a custom endpoint.
export function resolveAddr(
  inbound: Inbound,
  hostOverride: string,
  fallbackHostname: string,
): string {
  return resolveShareHost(inbound, hostOverride, fallbackHostname);
}

// A loopback browser host means the panel was reached through a tunnel (e.g.
// SSH-forwarded 127.0.0.1/localhost), so it can never be a shareable link host.
function isLoopbackHost(host: string): boolean {
  const h = host
    .trim()
    .replace(/^\[|\]$/g, '')
    .toLowerCase();
  return h === 'localhost' || h === '::1' || h.startsWith('127.');
}

// preferPublicHost is the browser-side analog of the backend's
// configuredPublicHost: when the panel is reached on a loopback host, prefer a
// configured public host (Sub/Web Domain) for share/QR links instead of leaking
// localhost. An explicit per-inbound listen or node override still wins, since
// resolveAddr only reaches the fallbackHostname after those.
export function preferPublicHost(browserHost: string, publicHost: string): string {
  return publicHost && isLoopbackHost(browserHost) ? publicHost : browserHost;
}

// Returns the client array for protocols that have one. SS returns its
// clients only in 2022-blake3 multi-user mode (matches the legacy
// `this.clients` getter, which used isSSMultiUser to gate). Returns null
// for SS single-user, http, mixed, tunnel, wireguard, hysteria2-without-
// clients, and any protocol without a clients array.
type ClientShape = {
  id?: string;
  security?: VmessSecurity;
  flow?: VlessClient['flow'];
  password?: string;
  auth?: string;
  secret?: string;
  email?: string;
  subId?: string;
};

// Mirror of the Go subKey: the stable per-client identity spx derivation
// keys on — subscription id first, unique email as the fallback.
function clientSubKey(client: ClientShape): string {
  return client.subId || client.email || '';
}

export function getInboundClients(inbound: Inbound): ClientShape[] | null {
  switch (inbound.protocol) {
    case 'vmess':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'vless':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'trojan':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'hysteria':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'mtproto':
      return (inbound.settings.clients ?? []) as ClientShape[];
    case 'shadowsocks': {
      const isMultiUser = inbound.settings.method !== '2022-blake3-chacha20-poly1305';
      return isMultiUser ? ((inbound.settings.clients ?? []) as ClientShape[]) : null;
    }
    default:
      return null;
  }
}

export interface GenLinkInput {
  inbound: Inbound;
  address: string;
  port?: number;
  forceTls?: ForceTls;
  remark?: string;
  client: ClientShape;
  externalProxy?: ExternalProxyEntry | null;
}

// Per-protocol dispatcher matching the legacy `genLink` switch. Returns
// '' for protocols that don't have client-based share links (wireguard
// goes through genWireguardLinks/Configs separately, http/mixed/tunnel
// don't have share URLs).
export function genLink(input: GenLinkInput): string {
  const {
    inbound,
    address,
    port = inbound.port,
    forceTls = 'same',
    remark = '',
    client,
    externalProxy = null,
  } = input;
  switch (inbound.protocol) {
    case 'vmess':
      return genVmessLink({
        inbound,
        address,
        port,
        forceTls,
        remark,
        clientId: client.id ?? '',
        security: client.security,
        externalProxy,
      });
    case 'vless':
      return genVlessLink({
        inbound,
        address,
        port,
        forceTls,
        remark,
        clientId: client.id ?? '',
        clientKey: clientSubKey(client),
        flow: client.flow,
        externalProxy,
      });
    case 'shadowsocks': {
      const isMultiUser = inbound.settings.method !== '2022-blake3-chacha20-poly1305';
      return genShadowsocksLink({
        inbound,
        address,
        port,
        forceTls,
        remark,
        clientPassword: isMultiUser ? (client.password ?? '') : '',
        externalProxy,
      });
    }
    case 'trojan':
      return genTrojanLink({
        inbound,
        address,
        port,
        forceTls,
        remark,
        clientPassword: client.password ?? '',
        clientKey: clientSubKey(client),
        externalProxy,
      });
    case 'hysteria':
      return genHysteriaLink({
        inbound,
        address,
        port,
        remark,
        clientAuth: client.auth ?? '',
        externalProxy,
      });
    case 'mtproto':
      return genMtprotoLink({ inbound, address, port, clientSecret: client.secret ?? '' });
    case 'qwdtt':
      return genQwdttLink({ inbound, address, remark });
    case 'olcrtc':
      return genOlcrtcLink({ inbound, remark });
    default:
      return '';
  }
}

export interface GenAllLinksEntry {
  remark: string;
  link: string;
}

export interface GenAllLinksInput {
  inbound: Inbound;
  remark?: string;
  client: ClientShape;
  hostOverride?: string;
  fallbackHostname: string;
}

// Fans out a single client's link per externalProxy entry, or just one link
// when there are no external proxies. The panel copy/QR remark is the inbound
// remark plus the externalProxy remark, dash-joined (the configurable
// subscription remark model was removed; subscription output uses the template).
export function genAllLinks(input: GenAllLinksInput): GenAllLinksEntry[] {
  const { inbound, remark = '', client, hostOverride = '', fallbackHostname } = input;

  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const port = inbound.port;

  const composeRemark = (proxyRemark: string): string =>
    [remark, proxyRemark].filter((x) => x.length > 0).join('-');

  const externals = inbound.streamSettings?.externalProxy;
  if (!externals || externals.length === 0) {
    const r = composeRemark('');
    const link = genLink({ inbound, address: addr, port, forceTls: 'same', remark: r, client });
    return link ? [{ remark: r, link }] : [];
  }
  return externals.map((ep) => {
    const r = composeRemark(ep.remark);
    return {
      remark: r,
      link: genLink({
        inbound,
        address: ep.dest,
        port: ep.port,
        forceTls: ep.forceTls,
        remark: r,
        client,
        externalProxy: ep,
      }),
    };
  });
}

export interface GenInboundLinksInput {
  inbound: Inbound;
  remark?: string;
  hostOverride?: string;
  fallbackHostname: string;
}

// Top-level entrypoint that produces the full \r\n-joined block a user
// pastes into a client. Iterates per-client for protocols with clients,
// falls back to a single SS link for single-user 2022-blake3-chacha20,
// and emits per-peer .conf blocks for wireguard and amneziawg. Returns '' for the
// other clientless protocols (http, mixed, tunnel).
export function genInboundLinks(input: GenInboundLinksInput): string {
  const { inbound, remark = '', hostOverride = '', fallbackHostname } = input;
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const clients = getInboundClients(inbound);
  if (clients) {
    const links: string[] = [];
    for (const client of clients) {
      const entries = genAllLinks({ inbound, remark, client, hostOverride, fallbackHostname });
      for (const e of entries) links.push(e.link);
    }
    return links.join('\r\n');
  }
  if (inbound.protocol === 'shadowsocks') {
    return genShadowsocksLink({
      inbound,
      address: addr,
      port: inbound.port,
      forceTls: 'same',
      remark,
    });
  }
  if (inbound.protocol === 'wireguard') {
    return genWireguardConfigs({ inbound, remark, hostOverride, fallbackHostname });
  }
  if (inbound.protocol === 'amneziawg') {
    return genAmneziaWGConfigs({ inbound, remark, hostOverride, fallbackHostname });
  }
  // LUCX-HOOK: AWG — render per-client .conf blocks (same path as WireGuard).
  if (inbound.protocol === 'awg') {
    return genAwgConfigs({ inbound, remark, hostOverride, fallbackHostname });
  }
  // LUCX-HOOK: single-credential tunnel sidecars (no clients array).
  if (inbound.protocol === 'qwdtt') {
    return genQwdttLink({ inbound, address: addr, remark });
  }
  if (inbound.protocol === 'olcrtc') {
    return genOlcrtcLink({ inbound, remark });
  }
  // END LUCX-HOOK
  return '';
}

export interface GenQwdttLinkInput {
  inbound: Inbound;
  address?: string;
  remark?: string;
}

// genQwdttLink builds qwdtt://config?... for the SpaceNeuroX Android client.
// peer = settings.subHost, else address:dtlsPort (address from resolveAddr /
// panel host). Empty password → ''.
export function genQwdttLink(input: GenQwdttLinkInput): string {
  if (input.inbound.protocol !== 'qwdtt') return '';
  const s = input.inbound.settings as {
    listenAddr?: string;
    password?: string;
    subHost?: string;
    vkHashes?: string;
    workers?: number;
    clientPort?: number;
    remark?: string;
  };
  const pass = (s.password ?? '').trim();
  if (!pass) return '';
  let peer = (s.subHost ?? '').trim();
  if (!peer) {
    const host = (input.address ?? '').trim();
    if (!host) return '';
    let dtlsPort = input.inbound.port || 56000;
    const la = (s.listenAddr ?? '').trim();
    if (la.includes(':')) {
      const p = Number(la.slice(la.lastIndexOf(':') + 1));
      if (Number.isFinite(p) && p > 0) dtlsPort = p;
    }
    peer = host.includes(':') && !host.startsWith('[') ? host : `${host}:${dtlsPort}`;
  }
  const name = (input.remark || s.remark || 'qWDTT').trim() || 'qWDTT';
  const q = new URLSearchParams();
  q.set('name', name);
  q.set('peer', peer);
  const hashes = (s.vkHashes ?? '').trim();
  if (hashes) q.set('hashes', hashes);
  q.set('workers', String(s.workers && s.workers > 0 ? s.workers : 16));
  q.set('port', String(s.clientPort && s.clientPort > 0 ? s.clientPort : 9000));
  q.set('pass', pass);
  return `qwdtt://config?${q.toString()}`;
}

export interface GenOlcrtcLinkInput {
  inbound: Inbound;
  remark?: string;
}

// genOlcrtcLink builds olcrtc://provider?transport@room#key (single credential).
export function genOlcrtcLink(input: GenOlcrtcLinkInput): string {
  if (input.inbound.protocol !== 'olcrtc') return '';
  const s = input.inbound.settings as {
    provider?: string;
    roomId?: string;
    cryptoKey?: string;
    transport?: string;
    vp8Fps?: number;
    vp8Batch?: number;
  };
  const room = (s.roomId ?? '').trim();
  const key = (s.cryptoKey ?? '').trim();
  if (!room || !key) return '';
  const provider = (s.provider ?? 'jitsi').trim() || 'jitsi';
  let transport = (s.transport ?? 'datachannel').trim() || 'datachannel';
  if (transport === 'vp8channel') {
    const fps = s.vp8Fps && s.vp8Fps > 0 ? s.vp8Fps : 60;
    const batch = s.vp8Batch && s.vp8Batch > 0 ? s.vp8Batch : 64;
    transport = `vp8channel<vp8-fps=${fps}&vp8-batch=${batch}>`;
  }
  return `olcrtc://${provider}?${transport}@${room}#${key}`;
}

// Per-peer wireguard fanout. Each peer gets its own link (or .conf
// block) with an index-suffixed remark, joined by \r\n. Matches the
// legacy genWireguardLinks / genWireguardConfigs exactly.
export interface GenWireguardFanoutInput {
  inbound: Inbound;
  remark?: string;
  hostOverride?: string;
  fallbackHostname: string;
}

// WireGuard is multi-client: each client is one accepted peer. The canonical
// store is settings.clients; legacy single-config inbounds (pre-migration) are
// still rendered from settings.peers. Both carry the privateKey/allowedIPs/
// preSharedKey/keepAlive the link and .conf need, so they project to the same
// peer shape and reuse genWireguardLink/genWireguardConfig unchanged.
function wgRenderPeers(settings: WireguardInboundSettings): WireguardInboundPeer[] {
  const clients = settings.clients ?? [];
  if (clients.length > 0) {
    return clients.map((c) => ({ ...c, publicKey: c.publicKey ?? '' }));
  }
  return settings.peers;
}

export function genWireguardLinks(input: GenWireguardFanoutInput): string {
  const { inbound, remark = '', hostOverride = '', fallbackHostname } = input;
  if (inbound.protocol !== 'wireguard') return '';
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const sep = '-';
  const baseSettings = inbound.settings as WireguardInboundSettings;
  const peers = wgRenderPeers(baseSettings);
  const settings: WireguardInboundSettings = { ...baseSettings, peers };
  return peers
    .map((p, i) =>
      genWireguardLink({
        settings,
        address: addr,
        port: inbound.port,
        remark: `${remark}${sep}${i + 1}${wgPeerCommentSuffix(p)}`,
        peerIndex: i,
      }),
    )
    .join('\r\n');
}

export function genWireguardConfigs(input: GenWireguardFanoutInput): string {
  const { inbound, remark = '', hostOverride = '', fallbackHostname } = input;
  if (inbound.protocol !== 'wireguard') return '';
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const sep = '-';
  const baseSettings = inbound.settings as WireguardInboundSettings;
  const peers = wgRenderPeers(baseSettings);
  const settings: WireguardInboundSettings = { ...baseSettings, peers };
  return peers
    .map((p, i) =>
      genWireguardConfig({
        settings,
        address: addr,
        port: inbound.port,
        remark: `${remark}${sep}${i + 1}${wgPeerCommentSuffix(p)}`,
        peerIndex: i,
      }),
    )
    .join('\r\n');
}

// Peer comments (#5168) are panel-side annotations; when present they ride
// along in the share remark so the device is identifiable in client apps.
function wgPeerCommentSuffix(peer: unknown): string {
  const comment = (peer as { comment?: unknown })?.comment;
  return typeof comment === 'string' && comment.trim() !== '' ? ` (${comment.trim()})` : '';
}

// LUCX-HOOK: AWG share-link + .conf generators (mirror WireGuard with AWG obfuscation params).
export interface GenAwgLinkInput {
  settings: AwgInboundSettings;
  address: string;
  port: number;
  remark?: string;
  peerIndex: number;
  // awgVersionOverride clamps the emitted config field set to a version at or
  // below the inbound's ceiling (settings.awgVersion). Used by the clients-page
  // export selector so a v3 inbound can hand a v2 client a v2 config. Absent =
  // the ceiling (what the subscription/share-link path uses). Ignored by
  // genAwgLink — share-links always use the ceiling.
  awgVersionOverride?: AwgVersion;
}

// awgPeerShape projects an AWG client (new publicKey/privateKey/preSharedKey/
// allowedIPs/keepAlive fields, or legacy id/password) to the peer shape the
// link/config generators consume — same fields as WireguardInboundPeer.
function awgPeerShape(c: AwgInboundSettings['clients'][number]): WireguardInboundPeer {
  return {
    privateKey: c.privateKey ?? '',
    publicKey: c.publicKey || (c.id ?? ''),
    preSharedKey: c.preSharedKey ?? c.password ?? '',
    allowedIPs: c.allowedIPs ?? [],
    keepAlive: c.keepAlive ?? 0,
    comment: '',
  } as WireguardInboundPeer;
}

// AwgVersion is the AmneziaWG protocol version a client config / share-link is
// generated for. '1.5' omits S3/S4, I1-I5, and HeaderProtectionKey; '2' adds
// S3/S4 + optional I1-I5 but omits HeaderProtectionKey; '3' adds
// HeaderProtectionKey. The server .conf's version is the ceiling — a client
// version higher than the server's breaks the handshake on must-match fields.
export type AwgVersion = '1.5' | '2' | '3' | '3.1';

const AWG_VERSION_ORDER: Record<AwgVersion, number> = { '1.5': 1, '2': 2, '3': 3, '3.1': 4 };

// awgVersionCeiling normalizes a stored awgVersion to one of the known
// values, defaulting to '2' (the safe, universally-accepted version) for
// absent/garbage values. Mirrors awg.NormalizeAWGVersion on the backend.
export function awgVersionCeiling(v: string | undefined | null): AwgVersion {
  if (v === '1.5' || v === '2' || v === '3' || v === '3.1') return v;
  return '2';
}

// awgVersionAtLeast reports whether target includes the feature set of floor
// (so callers gate per-version field emission: emit HeaderProtectionKey only
// when awgVersionAtLeast(v, '3'), etc.).
export function awgVersionAtLeast(target: AwgVersion, floor: AwgVersion): boolean {
  return AWG_VERSION_ORDER[target] >= AWG_VERSION_ORDER[floor];
}

// awgTimerEmit returns the shareable form of an AWG3 device-timer value, or ''
// when it is the kernel default and must be omitted. The value is a string that
// may carry an inclusive range ("100-500") — returned verbatim so the native
// kernel range survives into share-links and exported .conf text.
function awgTimerEmit(v: string | number | undefined): string {
  if (v === undefined || v === null) return '';
  const s = String(v).trim();
  if (s === '' || s === '0' || s === '0-0') return '';
  return s;
}

export function genAwgLink(input: GenAwgLinkInput): string {
  const { settings, address, port, remark = '', peerIndex } = input;
  const peer = awgPeerShape(settings.clients[peerIndex]);
  if (!peer) return '';

  const url = new URL(`amneziawg://${formatUrlHost(address)}:${port}`);
  url.username = peer.privateKey ?? '';

  // Server public key is derived from the inbound privateKey (Curve25519).
  if (settings.privateKey.length > 0) {
    const pubKey = Wireguard.generateKeypair(settings.privateKey).publicKey;
    if (pubKey.length > 0) url.searchParams.set('publickey', pubKey);
  }
  if (peer.allowedIPs.length > 0 && peer.allowedIPs[0]) {
    url.searchParams.set('address', peer.allowedIPs[0]);
  }
  if (typeof settings.mtu === 'number' && settings.mtu > 0) {
    url.searchParams.set('mtu', String(settings.mtu));
  }
  // AWG obfuscation params, gated by the inbound's awgVersion (the server
  // ceiling). Jc/Jmin/Jmax + S1/S2 + H1-H4 exist in every AWG version; S3/S4
  // and I1-I5 were added in AWG v2 (Android 2.0.1); HeaderProtectionKey is
  // AWG3-only (desktop 5.0.0.5 / Android 3.0.1). A version higher than the
  // server's must-match fields would break the handshake, so we never emit a
  // field set the server .conf does not carry.
  const v = awgVersionCeiling(settings.awgVersion);
  if (settings.jc) url.searchParams.set('jc', String(settings.jc));
  if (settings.jmin) url.searchParams.set('jmin', String(settings.jmin));
  if (settings.jmax) url.searchParams.set('jmax', String(settings.jmax));
  if (settings.s1) url.searchParams.set('s1', String(settings.s1));
  if (settings.s2) url.searchParams.set('s2', String(settings.s2));
  if (awgVersionAtLeast(v, '2')) {
    if (settings.s3) url.searchParams.set('s3', String(settings.s3));
    if (settings.s4) url.searchParams.set('s4', String(settings.s4));
  }
  if (settings.h1) url.searchParams.set('h1', settings.h1);
  if (settings.h2) url.searchParams.set('h2', settings.h2);
  if (settings.h3) url.searchParams.set('h3', settings.h3);
  if (settings.h4) url.searchParams.set('h4', settings.h4);
  if (awgVersionAtLeast(v, '2')) {
    if (settings.i1) url.searchParams.set('i1', settings.i1);
    if (settings.i2) url.searchParams.set('i2', settings.i2);
    if (settings.i3) url.searchParams.set('i3', settings.i3);
    if (settings.i4) url.searchParams.set('i4', settings.i4);
    if (settings.i5) url.searchParams.set('i5', settings.i5);
  }
  if (awgVersionAtLeast(v, '3') && settings.headerProtectionKey) {
    url.searchParams.set('headerprotectionkey', settings.headerProtectionKey);
  }
  // AWG3 device-level timers/padding — "0"/empty = kernel default. Only emitted
  // for v3. Values may be inclusive ranges ("100-500") and pass through verbatim.
  if (awgVersionAtLeast(v, '3')) {
    const timers: Array<[string, string]> = [
      ['contentpaddingaddition', awgTimerEmit(settings.contentPaddingAddition)],
      ['rekeyaftertime', awgTimerEmit(settings.rekeyAfterTime)],
      ['rekeytimeout', awgTimerEmit(settings.rekeyTimeout)],
      ['rejectaftertime', awgTimerEmit(settings.rejectAfterTime)],
      ['keepalivetimeout', awgTimerEmit(settings.keepaliveTimeout)],
      ['maxhandshakeattempts', awgTimerEmit(settings.maxHandshakeAttempts)],
    ];
    for (const [key, val] of timers) {
      if (val) url.searchParams.set(key, val);
    }
  }
  if (awgVersionAtLeast(v, '3.1')) {
    if (settings.randomTrailers) url.searchParams.set('randomtrailers', 'true');
    if (settings.disableCookies) url.searchParams.set('disablecookies', 'true');
  }
  if (settings.dns) url.searchParams.set('dns', settings.dns);
  if (peer.preSharedKey) url.searchParams.set('presharedkey', peer.preSharedKey);
  {
    const ka = collapseKeepaliveForVersion(peer.keepAlive, awgVersionAtLeast(v, '3'));
    if (ka) url.searchParams.set('keepalive', ka);
  }

  url.hash = encodeURIComponent(remark);
  return url.toString();
}

export function genAwgConfig(input: GenAwgLinkInput): string {
  const { settings, address, port, remark = '', peerIndex } = input;
  const peer = awgPeerShape(settings.clients[peerIndex]);
  if (!peer) return '';

  const pubKey =
    settings.privateKey.length > 0 ? Wireguard.generateKeypair(settings.privateKey).publicKey : '';

  let txt = `[Interface]\n`;
  txt += `PrivateKey = ${peer.privateKey ?? ''}\n`;
  txt += `Address = ${peer.allowedIPs[0] ?? ''}\n`;
  txt += `DNS = ${settings.dns || '1.1.1.1, 1.0.0.1'}\n`;
  if (typeof settings.mtu === 'number' && settings.mtu > 0) {
    txt += `MTU = ${settings.mtu}\n`;
  }
  // AWG obfuscation params in [Interface], gated by awgVersion (the server
  // ceiling — see genAwgLink). S3/S4 and I1-I5 are AWG v2+; HeaderProtectionKey
  // is AWG3-only. versionOverride clamps the emitted set to a version at or
  // below the ceiling (clients-page export selector); absent = ceiling.
  const ceiling = awgVersionCeiling(settings.awgVersion);
  const override =
    input.awgVersionOverride && awgVersionAtLeast(ceiling, input.awgVersionOverride)
      ? input.awgVersionOverride
      : ceiling;
  if (settings.jc) txt += `Jc = ${settings.jc}\n`;
  if (settings.jmin) txt += `Jmin = ${settings.jmin}\n`;
  if (settings.jmax) txt += `Jmax = ${settings.jmax}\n`;
  if (settings.s1) txt += `S1 = ${settings.s1}\n`;
  if (settings.s2) txt += `S2 = ${settings.s2}\n`;
  if (awgVersionAtLeast(override, '2')) {
    if (settings.s3) txt += `S3 = ${settings.s3}\n`;
    if (settings.s4) txt += `S4 = ${settings.s4}\n`;
  }
  if (settings.h1) txt += `H1 = ${settings.h1}\n`;
  if (settings.h2) txt += `H2 = ${settings.h2}\n`;
  if (settings.h3) txt += `H3 = ${settings.h3}\n`;
  if (settings.h4) txt += `H4 = ${settings.h4}\n`;
  // I1-I5 are stored verbatim in CPS tag format ("<b 0xHEX>" or "<r 2><b 0xHEX>")
  // — write as-is, no double wrapping. AWG v2+ only.
  if (awgVersionAtLeast(override, '2')) {
    if (settings.i1) txt += `I1 = ${settings.i1}\n`;
    if (settings.i2) txt += `I2 = ${settings.i2}\n`;
    if (settings.i3) txt += `I3 = ${settings.i3}\n`;
    if (settings.i4) txt += `I4 = ${settings.i4}\n`;
    if (settings.i5) txt += `I5 = ${settings.i5}\n`;
  }
  // HeaderProtectionKey (AWG3) — written only at version '3'. Older awg-quick
  // builds reject the line ("Line unrecognized"), so it must never reach a v1/v2
  // config. S1-S4 >= 12 is required (enforced by the generator for v3).
  if (awgVersionAtLeast(override, '3') && settings.headerProtectionKey) {
    txt += `HeaderProtectionKey = ${settings.headerProtectionKey}\n`;
  }
  // AWG3 device-level timers/padding — "0"/empty = kernel default. Only for v3.
  // Values may be inclusive ranges ("100-500") and pass through verbatim.
  if (awgVersionAtLeast(override, '3')) {
    const lines: Array<[string, string]> = [
      ['ContentPaddingAddition', awgTimerEmit(settings.contentPaddingAddition)],
      ['RekeyAfterTime', awgTimerEmit(settings.rekeyAfterTime)],
      ['RekeyTimeout', awgTimerEmit(settings.rekeyTimeout)],
      ['RejectAfterTime', awgTimerEmit(settings.rejectAfterTime)],
      ['KeepaliveTimeout', awgTimerEmit(settings.keepaliveTimeout)],
      ['MaxHandshakeAttempts', awgTimerEmit(settings.maxHandshakeAttempts)],
    ];
    for (const [key, val] of lines) {
      if (val) txt += `${key} = ${val}\n`;
    }
  }
  if (awgVersionAtLeast(override, '3.1')) {
    if (settings.randomTrailers) txt += `RandomTrailers = on\n`;
    if (settings.disableCookies) txt += `DisableCookies = on\n`;
  }

  txt += `\n# ${remark}\n`;
  txt += `[Peer]\n`;
  txt += `PublicKey = ${pubKey}\n`;
  txt += `AllowedIPs = 0.0.0.0/0, ::/0\n`;
  txt += `Endpoint = ${formatUrlHost(address)}:${port}`;
  if (peer.preSharedKey && peer.preSharedKey.length > 0) {
    txt += `\nPresharedKey = ${peer.preSharedKey}`;
  }
  {
    const ka = collapseKeepaliveForVersion(peer.keepAlive, awgVersionAtLeast(override, '3'));
    if (ka) txt += `\nPersistentKeepalive = ${ka}\n`;
  }
  return txt;
}

export function genAwgConfigs(input: GenInboundLinksInput): string {
  const { inbound, remark = '', hostOverride = '', fallbackHostname } = input;
  if (inbound.protocol !== 'awg') return '';
  const addr = resolveAddr(inbound, hostOverride, fallbackHostname);
  const sep = '-';
  const settings = inbound.settings as AwgInboundSettings;
  return settings.clients
    .map((c, i) =>
      genAwgConfig({
        settings,
        address: addr,
        port: inbound.port,
        remark: `${remark}${sep}${i + 1}${wgPeerCommentSuffix(c)}`,
        peerIndex: i,
      }),
    )
    .join('\r\n');
}
// END LUCX-HOOK

export function isPostQuantumLink(link: string): boolean {
  if (/[?&]pqv=/.test(link)) return true;
  if (link.includes('mlkem768') || link.includes('mldsa65')) return true;
  if (link.includes('ML-KEM-768')) return true;
  return false;
}
